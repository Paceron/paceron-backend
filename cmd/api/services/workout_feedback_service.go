package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
)

// Errores de negocio del módulo de feedback. El controller los mapea a 400/404/403/409
// vía errors.Is (ErrWorkoutFeedbackNotFound se resuelve con daos.ErrWorkoutFeedbackNotFound).
var (
	ErrWorkoutFeedbackInvalid   = errors.New("datos inválidos")
	ErrWorkoutFeedbackForbidden = errors.New("no tenés permisos para operar sobre este feedback")
)

// maxPointsPerBulk capa el array de puntos de una sola serie: una serie larga a
// 1 punto/s (~1h30m) ronda los 5.000; es un límite de saneamiento, no de dominio.
const maxPointsPerBulk = 5000

// PointsResult son los conteos efectivos del bulk insert idempotente de puntos.
type PointsResult struct {
	Created int
	Skipped int
}

// WorkoutFeedbackServiceInterface define las operaciones de negocio de feedback
// de entrenamiento. feedback_owner_user_id se resuelve SIEMPRE del token
// (auth_user_id); athlete_user_id se infiere salvo que un entrenador autorizado lo
// indique en create.
type WorkoutFeedbackServiceInterface interface {
	Create(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error)
	GetByID(ctx *gin.Context, authUserID, feedbackID int64) (*dbs.WorkoutFeedback, error)
	Search(ctx *gin.Context, authUserID int64, filters workoutfeedback.SearchFilters) ([]dbs.WorkoutFeedback, error)
	Update(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error)
	SoftDelete(ctx *gin.Context, authUserID, feedbackID int64) error
	// CreatePoints inserta el recorrido de la serie (bulk idempotente por
	// (feedback_id, "order")) y devuelve los conteos created/skipped.
	CreatePoints(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*PointsResult, error)
	// GetPoints devuelve el recorrido de la serie ordenado por "order".
	GetPoints(ctx *gin.Context, authUserID, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error)
	// GetSessionFeedback devuelve los feedbacks activos de una sesión asignada
	// (y de un atleta en particular si viene; default self), ordenados por
	// (assigned_exercise_id, set_number) — el shape de la pantalla de revisión.
	GetSessionFeedback(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error)
}

type workoutFeedbackService struct {
	workoutFeedbackDao daos.WorkoutFeedbackDAOInterface
}

// NewWorkoutFeedbackService crea una nueva instancia de WorkoutFeedbackService.
func NewWorkoutFeedbackService(workoutFeedbackDao daos.WorkoutFeedbackDAOInterface) WorkoutFeedbackServiceInterface {
	return &workoutFeedbackService{
		workoutFeedbackDao: workoutFeedbackDao,
	}
}

// Create valida el input, resuelve el atleta (default auth) y aplica la matriz:
// feedback_owner_user_id = auth_user_id; si athlete_user_id viene y es distinto,
// el auth debe ser owner de un equipo al que pertenezca ese atleta (403 si no).
func (s *workoutFeedbackService) Create(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
	ateamID := req.TeamID
	if ateamID != nil && *ateamID <= 0 {
		return nil, fmt.Errorf("%w: team_id debe ser un número entero mayor a 0", ErrWorkoutFeedbackInvalid)
	}
	if req.AssignedSessionID <= 0 || req.AssignedExerciseID <= 0 {
		return nil, fmt.Errorf("%w: assigned_session_id y assigned_exercise_id deben ser mayores a 0", ErrWorkoutFeedbackInvalid)
	}
	if req.ReportSource == "" {
		return nil, fmt.Errorf("%w: report_source es obligatorio", ErrWorkoutFeedbackInvalid)
	}
	if req.SetNumber < 0 {
		return nil, fmt.Errorf("%w: set_number debe ser mayor o igual a 0", ErrWorkoutFeedbackInvalid)
	}

	sessionDate, err := parseSessionDate(req.SessionDate)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWorkoutFeedbackInvalid, err)
	}

	if err := validateMetrics(validateMetricsInput{
		RPE:                 req.RPE,
		DurationMs:          req.DurationMs,
		ActiveDurationMs:    req.ActiveDurationMs,
		WeightKg:            req.WeightKg,
		Reps:                req.Reps,
		DistanceMeters:      req.DistanceMeters,
		AvgHeartRate:        req.AvgHeartRate,
		MaxHeartRate:        req.MaxHeartRate,
		ElevationGainMeters: req.ElevationGainMeters,
		Cadence:             req.Cadence,
		StartedAt:           req.StartedAt,
		EndedAt:             req.EndedAt,
	}); err != nil {
		return nil, err
	}

	athleteUserID := authUserID
	if req.AthleteUserID != nil {
		if *req.AthleteUserID <= 0 {
			return nil, fmt.Errorf("%w: athlete_user_id debe ser un número entero mayor a 0", ErrWorkoutFeedbackInvalid)
		}
		if *req.AthleteUserID != authUserID {
			inOwnedTeam, err := s.workoutFeedbackDao.ExistsUserInTeamOwnedBy(ctx, *req.AthleteUserID, authUserID)
			if err != nil {
				return nil, err
			}
			if !inOwnedTeam {
				return nil, ErrWorkoutFeedbackForbidden
			}
		}
		athleteUserID = *req.AthleteUserID
	}

	feedback := &dbs.WorkoutFeedback{
		TeamID:              ateamID,
		AssignedSessionID:   req.AssignedSessionID,
		AssignedExerciseID:  req.AssignedExerciseID,
		AthleteUserID:       athleteUserID,
		FeedbackOwnerUserID: authUserID,
		ReportSource:        req.ReportSource,
		SessionDate:         sessionDate,
		SetNumber:           req.SetNumber,
		StartedAt:           req.StartedAt,
		EndedAt:             req.EndedAt,
		DurationMs:          req.DurationMs,
		ActiveDurationMs:    req.ActiveDurationMs,
		WeightKg:            req.WeightKg,
		Reps:                req.Reps,
		DistanceMeters:      req.DistanceMeters,
		RPE:                 req.RPE,
		AvgHeartRate:        req.AvgHeartRate,
		MaxHeartRate:        req.MaxHeartRate,
		CompletionStatus:    req.CompletionStatus,
		ElevationGainMeters: req.ElevationGainMeters,
		Cadence:             req.Cadence,
		Annotations:         req.Annotations,
		MediaURLs:           textArray(req.MediaURLs),
	}

	if err := s.workoutFeedbackDao.Create(ctx, feedback); err != nil {
		return nil, err
	}
	return feedback, nil
}

// GetByID autoriza como Get de la matriz: reportante, atleta o owner del team.
func (s *workoutFeedbackService) GetByID(ctx *gin.Context, authUserID, feedbackID int64) (*dbs.WorkoutFeedback, error) {
	feedback, err := s.workoutFeedbackDao.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}

	if !s.canAccess(ctx, authUserID, feedback) {
		return nil, ErrWorkoutFeedbackForbidden
	}
	return feedback, nil
}

// Search aplica la matriz de búsqueda: sin params → scope self; team_id → existe
// (404) + auth owner (403); athlete_user_id ajeno → owner de un equipo del atleta
// (403). Filtros adicionales se combinan con el scope.
func (s *workoutFeedbackService) Search(ctx *gin.Context, authUserID int64, filters workoutfeedback.SearchFilters) ([]dbs.WorkoutFeedback, error) {
	dateFrom, err := toTimeFilter(filters.SessionDateFrom)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWorkoutFeedbackInvalid, err)
	}
	dateTo, err := toTimeFilter(filters.SessionDateTo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWorkoutFeedbackInvalid, err)
	}

	if filters.AthleteUserID != nil {
		if *filters.AthleteUserID != authUserID {
			inOwnedTeam, err := s.workoutFeedbackDao.ExistsUserInTeamOwnedBy(ctx, *filters.AthleteUserID, authUserID)
			if err != nil {
				return nil, err
			}
			if !inOwnedTeam {
				return nil, ErrWorkoutFeedbackForbidden
			}
		}
		return s.workoutFeedbackDao.Search(ctx, daos.WorkoutFeedbackSearchFilters{
			AthleteUserID:       filters.AthleteUserID,
			FeedbackOwnerUserID: filters.FeedbackOwnerUserID,
			AssignedSessionID:   filters.AssignedSessionID,
			AssignedExerciseID:  filters.AssignedExerciseID,
			SessionDateFrom:     dateFrom,
			SessionDateTo:       dateTo,
		})
	}

	if filters.TeamID != nil {
		exists, err := s.workoutFeedbackDao.TeamExists(ctx, *filters.TeamID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrTeamNotFound
		}

		isOwner, err := s.workoutFeedbackDao.IsTeamOwner(ctx, *filters.TeamID, authUserID)
		if err != nil {
			return nil, err
		}
		if !isOwner {
			return nil, ErrWorkoutFeedbackForbidden
		}

		return s.workoutFeedbackDao.Search(ctx, daos.WorkoutFeedbackSearchFilters{
			TeamID:              filters.TeamID,
			FeedbackOwnerUserID: filters.FeedbackOwnerUserID,
			AssignedSessionID:   filters.AssignedSessionID,
			AssignedExerciseID:  filters.AssignedExerciseID,
			SessionDateFrom:     dateFrom,
			SessionDateTo:       dateTo,
		})
	}

	// Sin params (o solo filtros de sesión/fecha): scope self.
	return s.workoutFeedbackDao.Search(ctx, daos.WorkoutFeedbackSearchFilters{
		SelfUserID:          &authUserID,
		FeedbackOwnerUserID: filters.FeedbackOwnerUserID,
		AssignedSessionID:   filters.AssignedSessionID,
		AssignedExerciseID:  filters.AssignedExerciseID,
		SessionDateFrom:     dateFrom,
		SessionDateTo:       dateTo,
	})
}

// Update autoriza como Get (reportante, atleta u owner del team), aplica edición
// parcial de los campos del body (athlete_user_id / feedback_owner_user_id no
// editables) y devuelve el registro persistido.
func (s *workoutFeedbackService) Update(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
	current, err := s.workoutFeedbackDao.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if !s.canAccess(ctx, authUserID, current) {
		return nil, ErrWorkoutFeedbackForbidden
	}

	updates, err := buildUpdates(req)
	if err != nil {
		return nil, err
	}

	updated, err := s.workoutFeedbackDao.Update(ctx, feedbackID, updates)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// SoftDelete autoriza como Get (reportante, atleta u owner del team) y aplica la
// baja lógica (deleted_at = now).
func (s *workoutFeedbackService) SoftDelete(ctx *gin.Context, authUserID, feedbackID int64) error {
	current, err := s.workoutFeedbackDao.GetByID(ctx, feedbackID)
	if err != nil {
		return err
	}
	if !s.canAccess(ctx, authUserID, current) {
		return ErrWorkoutFeedbackForbidden
	}

	return s.workoutFeedbackDao.SoftDelete(ctx, feedbackID)
}

// CreatePoints autoriza como Get (reportante, atleta u owner del team), valida el
// array de puntos y delega el bulk idempotente en el DAO. Reintentar el mismo
// recorrido devuelve created < len y skipped = len - created, sin error ni dupes.
func (s *workoutFeedbackService) CreatePoints(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*PointsResult, error) {
	feedback, err := s.workoutFeedbackDao.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if !s.canAccess(ctx, authUserID, feedback) {
		return nil, ErrWorkoutFeedbackForbidden
	}
	if len(req.Points) == 0 {
		return nil, fmt.Errorf("%w: el array points no puede estar vacío", ErrWorkoutFeedbackInvalid)
	}
	if len(req.Points) > maxPointsPerBulk {
		return nil, fmt.Errorf("%w: el array points no puede superar %d puntos", ErrWorkoutFeedbackInvalid, maxPointsPerBulk)
	}

	models := make([]dbs.WorkoutFeedbackPoint, 0, len(req.Points))
	for i := range req.Points {
		p := req.Points[i]
		if p.Order < 0 {
			return nil, fmt.Errorf("%w: order debe ser mayor o igual a 0", ErrWorkoutFeedbackInvalid)
		}
		if p.SessionInstanceID <= 0 || p.ExerciseInstanceID <= 0 {
			return nil, fmt.Errorf("%w: session_instance_id y exercise_instance_id deben ser mayores a 0", ErrWorkoutFeedbackInvalid)
		}
		if p.Latitude < -90 || p.Latitude > 90 {
			return nil, fmt.Errorf("%w: latitude debe estar entre -90 y 90", ErrWorkoutFeedbackInvalid)
		}
		if p.Longitude < -180 || p.Longitude > 180 {
			return nil, fmt.Errorf("%w: longitude debe estar entre -180 y 180", ErrWorkoutFeedbackInvalid)
		}
		models = append(models, dbs.WorkoutFeedbackPoint{
			FeedbackID:         feedbackID,
			SessionInstanceID:  p.SessionInstanceID,
			ExerciseInstanceID: p.ExerciseInstanceID,
			Order:              p.Order,
			Latitude:           p.Latitude,
			Longitude:          p.Longitude,
			RecordedAt:         p.RecordedAt,
		})
	}

	created, err := s.workoutFeedbackDao.BulkCreatePoints(ctx, feedbackID, models)
	if err != nil {
		return nil, err
	}
	return &PointsResult{Created: int(created), Skipped: len(models) - int(created)}, nil
}

// GetPoints autoriza como Get (reportante, atleta u owner del team) y devuelve el
// recorrido de la serie ordenado por "order".
func (s *workoutFeedbackService) GetPoints(ctx *gin.Context, authUserID, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
	feedback, err := s.workoutFeedbackDao.GetByID(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if !s.canAccess(ctx, authUserID, feedback) {
		return nil, ErrWorkoutFeedbackForbidden
	}
	points, err := s.workoutFeedbackDao.GetPointsByFeedback(ctx, feedbackID)
	if err != nil {
		return nil, err
	}
	if points == nil {
		points = []dbs.WorkoutFeedbackPoint{}
	}
	return points, nil
}

// GetSessionFeedback lista los feedbacks activos de una sesión asignada. El
// atleta es el auth salvo que venga un athlete_user_id distinto, en cuyo caso
// el auth debe ser owner de un equipo al que pertenezca ese atleta (matriz del
// módulo). Devuelve [] si no hay filas.
func (s *workoutFeedbackService) GetSessionFeedback(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
	target := authUserID
	if athleteUserID != nil && *athleteUserID != authUserID {
		if *athleteUserID <= 0 {
			return nil, fmt.Errorf("%w: athlete_user_id debe ser un número entero mayor a 0", ErrWorkoutFeedbackInvalid)
		}
		inOwnedTeam, err := s.workoutFeedbackDao.ExistsUserInTeamOwnedBy(ctx, *athleteUserID, authUserID)
		if err != nil {
			return nil, err
		}
		if !inOwnedTeam {
			return nil, ErrWorkoutFeedbackForbidden
		}
		target = *athleteUserID
	}

	feedbacks, err := s.workoutFeedbackDao.GetBySession(ctx, sessionInstanceID, &target)
	if err != nil {
		return nil, err
	}
	if feedbacks == nil {
		feedbacks = []dbs.WorkoutFeedback{}
	}
	return feedbacks, nil
}

// canAccess es el corazón de la matriz: reportante, atleta u owner del team.
// Un feedback sin team_id solo es accesible por reportante o atleta.
func (s *workoutFeedbackService) canAccess(ctx *gin.Context, authUserID int64, feedback *dbs.WorkoutFeedback) bool {
	if feedback.AthleteUserID == authUserID || feedback.FeedbackOwnerUserID == authUserID {
		return true
	}
	if feedback.TeamID != nil {
		isOwner, err := s.workoutFeedbackDao.IsTeamOwner(ctx, *feedback.TeamID, authUserID)
		if err != nil {
			return false
		}
		return isOwner
	}
	return false
}

// parseSessionDate valida el formato YYYY-MM-DD del spec.
func parseSessionDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("session_date es obligatoria (YYYY-MM-DD)")
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("session_date debe tener formato YYYY-MM-DD")
	}
	return parsed, nil
}

// toTimeFilter convierte un query param de fecha opcional (YYYY-MM-DD) a *time.Time
// para el filtro del DAO. nil si el param no viene.
func toTimeFilter(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	parsed, err := parseSessionDate(*raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// textArray convierte un []string a pgtype.TextArray ready para persistir (si
// media urls vienen nulos, se persiste NULL — no un array vacío, para distinguir
// "sin datos" de "array vacío").
func textArray(urls []string) pgtype.TextArray {
	var arr pgtype.TextArray
	if urls == nil {
		arr.Status = pgtype.Null
		return arr
	}
	_ = arr.Set(urls)
	return arr
}

type validateMetricsInput struct {
	RPE                 *int16
	DurationMs          *int64
	ActiveDurationMs    *int64
	WeightKg            *float64
	Reps                *int
	DistanceMeters      *float64
	AvgHeartRate        *int16
	MaxHeartRate        *int16
	ElevationGainMeters *float64
	Cadence             *int16
	StartedAt           *time.Time
	EndedAt             *time.Time
}

// validateMetrics valida las métricas opcionales: no negativas, rpe en 1..10
// (doble control con el CHECK de la DB) y consistencia temporal started/ended.
func validateMetrics(in validateMetricsInput) error {
	if in.RPE != nil && (*in.RPE < 1 || *in.RPE > 10) {
		return fmt.Errorf("%w: rpe debe estar entre 1 y 10", ErrWorkoutFeedbackInvalid)
	}
	for name, nonNegative := range map[string]bool{
		"duration_ms":           in.DurationMs != nil && *in.DurationMs < 0,
		"active_duration_ms":    in.ActiveDurationMs != nil && *in.ActiveDurationMs < 0,
		"weight_kg":             in.WeightKg != nil && *in.WeightKg < 0,
		"reps":                  in.Reps != nil && *in.Reps < 0,
		"distance_meters":       in.DistanceMeters != nil && *in.DistanceMeters < 0,
		"avg_heart_rate":        in.AvgHeartRate != nil && *in.AvgHeartRate < 0,
		"max_heart_rate":        in.MaxHeartRate != nil && *in.MaxHeartRate < 0,
		"elevation_gain_meters": in.ElevationGainMeters != nil && *in.ElevationGainMeters < 0,
		"cadence":               in.Cadence != nil && *in.Cadence < 0,
	} {
		if nonNegative {
			return fmt.Errorf("%w: %s debe ser mayor o igual a 0", ErrWorkoutFeedbackInvalid, name)
		}
	}
	if in.StartedAt != nil && in.EndedAt != nil && in.EndedAt.Before(*in.StartedAt) {
		return fmt.Errorf("%w: ended_at no puede ser anterior a started_at", ErrWorkoutFeedbackInvalid)
	}
	return nil
}

// buildUpdates arma el mapa de update parcial a partir de los campos provistos del
// body. athlete_user_id / feedback_owner_user_id nunca se editan.
func buildUpdates(req workoutfeedback.UpdateFeedbackRequest) (map[string]interface{}, error) {
	updates := map[string]interface{}{}

	if req.TeamID != nil && *req.TeamID <= 0 {
		return nil, fmt.Errorf("%w: team_id debe ser un número entero mayor a 0", ErrWorkoutFeedbackInvalid)
	}
	if req.AssignedSessionID != nil && *req.AssignedSessionID <= 0 {
		return nil, fmt.Errorf("%w: assigned_session_id debe ser mayor a 0", ErrWorkoutFeedbackInvalid)
	}
	if req.AssignedExerciseID != nil && *req.AssignedExerciseID <= 0 {
		return nil, fmt.Errorf("%w: assigned_exercise_id debe ser mayor a 0", ErrWorkoutFeedbackInvalid)
	}
	if req.SetNumber != nil && *req.SetNumber < 0 {
		return nil, fmt.Errorf("%w: set_number debe ser mayor o igual a 0", ErrWorkoutFeedbackInvalid)
	}
	if req.ReportSource != nil && *req.ReportSource == "" {
		return nil, fmt.Errorf("%w: report_source no puede quedar vacío", ErrWorkoutFeedbackInvalid)
	}

	if err := validateMetrics(validateMetricsInput{
		RPE:                 req.RPE,
		DurationMs:          req.DurationMs,
		ActiveDurationMs:    req.ActiveDurationMs,
		WeightKg:            req.WeightKg,
		Reps:                req.Reps,
		DistanceMeters:      req.DistanceMeters,
		AvgHeartRate:        req.AvgHeartRate,
		MaxHeartRate:        req.MaxHeartRate,
		ElevationGainMeters: req.ElevationGainMeters,
		Cadence:             req.Cadence,
		StartedAt:           req.StartedAt,
		EndedAt:             req.EndedAt,
	}); err != nil {
		return nil, err
	}

	if req.TeamID != nil {
		updates["team_id"] = *req.TeamID
	}
	if req.AssignedSessionID != nil {
		updates["assigned_session_id"] = *req.AssignedSessionID
	}
	if req.AssignedExerciseID != nil {
		updates["assigned_exercise_id"] = *req.AssignedExerciseID
	}
	if req.ReportSource != nil {
		updates["report_source"] = *req.ReportSource
	}
	if req.SessionDate != nil {
		parsed, err := parseSessionDate(*req.SessionDate)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrWorkoutFeedbackInvalid, err)
		}
		updates["session_date"] = parsed
	}
	if req.SetNumber != nil {
		updates["set_number"] = *req.SetNumber
	}
	if req.StartedAt != nil {
		updates["started_at"] = *req.StartedAt
	}
	if req.EndedAt != nil {
		updates["ended_at"] = *req.EndedAt
	}
	if req.DurationMs != nil {
		updates["duration_ms"] = *req.DurationMs
	}
	if req.ActiveDurationMs != nil {
		updates["active_duration_ms"] = *req.ActiveDurationMs
	}
	if req.WeightKg != nil {
		updates["weight_kg"] = *req.WeightKg
	}
	if req.Reps != nil {
		updates["reps"] = *req.Reps
	}
	if req.DistanceMeters != nil {
		updates["distance_meters"] = *req.DistanceMeters
	}
	if req.RPE != nil {
		updates["rpe"] = *req.RPE
	}
	if req.AvgHeartRate != nil {
		updates["avg_heart_rate"] = *req.AvgHeartRate
	}
	if req.MaxHeartRate != nil {
		updates["max_heart_rate"] = *req.MaxHeartRate
	}
	if req.CompletionStatus != nil {
		updates["completion_status"] = *req.CompletionStatus
	}
	if req.ElevationGainMeters != nil {
		updates["elevation_gain_meters"] = *req.ElevationGainMeters
	}
	if req.Cadence != nil {
		updates["cadence"] = *req.Cadence
	}
	if req.Annotations != nil {
		updates["annotations"] = *req.Annotations
	}
	if req.MediaURLs != nil {
		updates["media_urls"] = textArray(req.MediaURLs)
	}
	return updates, nil
}