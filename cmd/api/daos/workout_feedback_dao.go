package daos

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// Errores de negocio del módulo de feedback. El service los disfraza/mapas al
// status HTTP correspondiente vía errors.Is.
var (
	ErrWorkoutFeedbackDuplicate = errors.New("ya existe un feedback para ese set")
	ErrWorkoutFeedbackNotFound  = errors.New("feedback no encontrado")
)

// WorkoutFeedbackSearchFilters agrupa los filtros de búsqueda de feedbacks. Los
// valores llegan ya autorizados por la matriz del service: exactamente UNO de los
// scopes (SelfUserID | TeamID | AthleteUserID) debe estar seteado; el resto son
// filtros opcionales adicionales.
type WorkoutFeedbackSearchFilters struct {
	// SelfUserID, si está, restringe a (athlete_user_id = X OR feedback_owner_user_id = X).
	SelfUserID *int64
	// TeamID, si está, restringe a los feedbacks del equipo.
	TeamID *int64
	// AthleteUserID, si está, restringe a los feedbacks del atleta.
	AthleteUserID *int64
	// Filtros opcionales (se combinan con el scope).
	FeedbackOwnerUserID *int64
	AssignedSessionID   *int64
	AssignedExerciseID  *int64
	SessionDateFrom     *time.Time
	SessionDateTo       *time.Time
}

// WorkoutFeedbackHistoryFilters agrupa los filtros del historial de
// entrenamientos (design.md D5). Primer nivel (aplican a ítems, total y pools):
// AthleteUserID, TeamID, GroupID, DateFrom, DateTo. Segundo nivel (aplican solo
// a ítems y total; los pools lo ignoran): ExerciseInstanceID, SetNumber,
// AthleteFilterUserID. Los valores llegan ya validados/autorizados por el service.
type WorkoutFeedbackHistoryFilters struct {
	AthleteUserID       *int64
	TeamID              *int64
	GroupID             *int64
	DateFrom            *time.Time
	DateTo              *time.Time
	ExerciseInstanceID  *int64
	SetNumber           *int
	AthleteFilterUserID *int64
}

// WorkoutFeedbackDAOInterface define las operaciones de acceso a datos para
// feedbacks de entrenamiento.
type WorkoutFeedbackDAOInterface interface {
	// Create inserta un feedback. Si la base rechaza el insert por el índice único
	// parcial unique_feedback_per_set (mismo set activo), devuelve
	// ErrWorkoutFeedbackDuplicate. No hay SELECT previo a propósito (insert directo
	// + captura de la violación evita race conditions).
	Create(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error
	// GetByID devuelve el feedback activo (deleted_at IS NULL) con ese id, o
	// ErrWorkoutFeedbackNotFound si no existe o está soft-deleteado.
	GetByID(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error)
	// Update aplica un update parcial (solo los campos del map) sobre el feedback
	// activo, actualiza updated_at y devuelve el registro persistido. Devuelve
	// ErrWorkoutFeedbackNotFound si no hay feedback activo con ese id.
	Update(ctx *gin.Context, id int64, updates map[string]interface{}) (*dbs.WorkoutFeedback, error)
	// SoftDelete aplica baja lógica (deleted_at = now) sobre el feedback activo.
	// Devuelve ErrWorkoutFeedbackNotFound si no hay feedback activo con ese id.
	SoftDelete(ctx *gin.Context, id int64) error
	// Search devuelve los feedbacks activos que cumplen los filtros (WHERE dinámico,
	// scopes y filtros ya autorizados por el service).
	Search(ctx *gin.Context, filters WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error)
	// GetBySession devuelve los feedbacks activos de una sesión asignada (y de
	// un atleta en particular si viene), ordenados por (assigned_exercise_id,
	// set_number, id) — el orden de la pantalla de revisión por ejercicio/serie.
	GetBySession(ctx *gin.Context, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error)

	// BulkCreatePoints inserta en un solo batch los puntos del recorrido de una
	// serie. Idempotente por (feedback_id, "order") vía el índice único
	// uq_feedback_point_order: reintentar un punto ya existente no duplica ni
	// falla (INSERT ... ON CONFLICT DO NOTHING). Devuelve la cantidad de filas
	// realmente insertadas (created); el service calcula skipped = len - created.
	BulkCreatePoints(ctx *gin.Context, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (int64, error)
	// GetPointsByFeedback devuelve el recorrido de la serie ordenado por "order".
	GetPointsByFeedback(ctx *gin.Context, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error)

	// HistorySearch devuelve las filas enriquecidas del historial (design.md D1/D5)
	// que cumplen los filtros (1er + 2do nivel), con ORDER BY whitelisted
	// (feedback_date/set_number/exercise_name) y desempate determinista por wf.id
	// en la misma dirección; sort/order desconocidos caen al default
	// (feedback_date DESC) — validar la whitelist antes es del service. limit <= 0
	// significa sin tope.
	HistorySearch(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters, sortCol string, order string, limit int, offset int) ([]dbs.WorkoutFeedbackHistoryRow, error)
	// HistoryCount cuenta los feedbacks que cumplen TODOS los filtros (1er + 2do
	// nivel), sin paginación.
	HistoryCount(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters) (int64, error)
	// HistoryAvailableAthletes e HistoryAvailableExercises arman los pools de
	// filtros de la UI: DISTINCT sobre los feedbacks que cumplen SOLO el primer
	// nivel (ignoran ExerciseInstanceID/SetNumber/AthleteFilterUserID aunque
	// vengan seteados en filters).
	HistoryAvailableAthletes(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters) ([]dbs.IDName, error)
	HistoryAvailableExercises(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters) ([]dbs.IDName, error)

	// Chequeos de membresía de equipo compartidos (TeamMembershipDAO): los usa el
	// service para la matriz de autorización de equipos. Delegados internamente.
	TeamExists(ctx *gin.Context, teamID int64) (bool, error)
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
}

type workoutFeedbackDao struct {
	DB         *gorm.DB
	membership TeamMembershipDAOInterface
}

// NewWorkoutFeedbackDao crea una nueva instancia de WorkoutFeedbackDao.
func NewWorkoutFeedbackDao(database *gorm.DB) WorkoutFeedbackDAOInterface {
	return &workoutFeedbackDao{
		DB:         database,
		membership: NewTeamMembershipDao(database),
	}
}

// Create inserta un feedback. Duplicado del mismo set activo (índice único parcial
// unique_feedback_per_set) → ErrWorkoutFeedbackDuplicate. Soft-deleteados no
// bloquean: el índice es WHERE deleted_at IS NULL.
func (d *workoutFeedbackDao) Create(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error {
	err := d.DB.Create(feedback).Error
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation {
		return ErrWorkoutFeedbackDuplicate
	}
	return fmt.Errorf("error creating workout feedback: %w", err)
}

// GetByID devuelve el feedback activo con ese id. No encontrado (o borrado) →
// ErrWorkoutFeedbackNotFound.
func (d *workoutFeedbackDao) GetByID(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
	var feedback dbs.WorkoutFeedback
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&feedback).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkoutFeedbackNotFound
		}
		return nil, fmt.Errorf("error getting workout feedback: %w", err)
	}
	return &feedback, nil
}

// Update aplica los cambios del map sobre el feedback activo, refresca updated_at
// y devuelve el registro persistido. Sin feedback activo → ErrWorkoutFeedbackNotFound.
func (d *workoutFeedbackDao) Update(ctx *gin.Context, id int64, updates map[string]interface{}) (*dbs.WorkoutFeedback, error) {
	updates["updated_at"] = time.Now()
	res := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates)
	if res.Error != nil {
		var pgErr *pgconn.PgError
		if errors.As(res.Error, &pgErr) && pgErr.Code == postgresUniqueViolation {
			return nil, ErrWorkoutFeedbackDuplicate
		}
		return nil, fmt.Errorf("error updating workout feedback: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrWorkoutFeedbackNotFound
	}
	return d.GetByID(ctx, id)
}

// SoftDelete setea deleted_at = now sobre el feedback activo. Sin feedback activo
// → ErrWorkoutFeedbackNotFound.
func (d *workoutFeedbackDao) SoftDelete(ctx *gin.Context, id int64) error {
	res := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now())
	if res.Error != nil {
		return fmt.Errorf("error soft-deleting workout feedback: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrWorkoutFeedbackNotFound
	}
	return nil
}

// Search construye un WHERE dinámico con el scope y los filtros ya autorizados,
// excluyendo siempre los feedbacks soft-deleteados.
func (d *workoutFeedbackDao) Search(ctx *gin.Context, filters WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
	query := d.DB.Model(&dbs.WorkoutFeedback{}).Where("deleted_at IS NULL")

	if filters.SelfUserID != nil {
		query = query.Where("(athlete_user_id = ? OR feedback_owner_user_id = ?)", *filters.SelfUserID, *filters.SelfUserID)
	}
	if filters.TeamID != nil {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.AthleteUserID != nil {
		query = query.Where("athlete_user_id = ?", *filters.AthleteUserID)
	}
	if filters.FeedbackOwnerUserID != nil {
		query = query.Where("feedback_owner_user_id = ?", *filters.FeedbackOwnerUserID)
	}
	if filters.AssignedSessionID != nil {
		query = query.Where("assigned_session_id = ?", *filters.AssignedSessionID)
	}
	if filters.AssignedExerciseID != nil {
		query = query.Where("assigned_exercise_id = ?", *filters.AssignedExerciseID)
	}
	if filters.SessionDateFrom != nil {
		query = query.Where("session_date >= ?", *filters.SessionDateFrom)
	}
	if filters.SessionDateTo != nil {
		query = query.Where("session_date <= ?", *filters.SessionDateTo)
	}

	var feedbacks []dbs.WorkoutFeedback
	if err := query.Order("id").Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("error searching workout feedbacks: %w", err)
	}
	return feedbacks, nil
}

// BulkCreatePoints inserta el recorrido en un batch único atómico. La
// idempotencia la da la DB: el índice uq_feedback_point_order (feedback_id,
// "order") combinado con ON CONFLICT DO NOTHING hace que un reintento del mismo
// punto cuente como skipped, no como error ni como duplicado. Devuelve la
// cantidad de filas realmente insertadas (created).
func (d *workoutFeedbackDao) BulkCreatePoints(ctx *gin.Context, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (int64, error) {
	for i := range points {
		points[i].FeedbackID = feedbackID
	}
	res := d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "feedback_id"}, {Name: "order"}},
		DoNothing: true,
	}).Create(&points)
	if res.Error != nil {
		return 0, fmt.Errorf("error creating workout feedback points: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// GetPointsByFeedback devuelve el recorrido completo de la serie, ordenado por
// "order" (el ordinal 0-based del punto dentro de la serie).
func (d *workoutFeedbackDao) GetPointsByFeedback(ctx *gin.Context, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
	var points []dbs.WorkoutFeedbackPoint
	if err := d.DB.
		Where("feedback_id = ?", feedbackID).
		Order(`"order"`).
		Find(&points).Error; err != nil {
		return nil, fmt.Errorf("error getting workout feedback points: %w", err)
	}
	return points, nil
}

// GetBySession devuelve los feedbacks activos de una sesión asignada, con un
// orden estable por (assigned_exercise_id, set_number, id) — el agrupamiento
// por ejercicio/serie que la pantalla de revisión espera. athlete_user_id
// opcional restringe al atleta. Solo feedbacks con deleted_at IS NULL.
//
// Además llena WorkoutFeedback.PointsCount con un UN aggregate COUNT agrupado
// por feedback_id (no un query por fila): el frontend necesita saber si cada
// serie tiene trayectoria GPS para dibujar, y hacerlo con N queries sobre
// workout_feedback_points sería un N+1 sobre toda la sesión.
func (d *workoutFeedbackDao) GetBySession(ctx *gin.Context, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
	query := d.DB.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_session_id = ? AND deleted_at IS NULL", sessionInstanceID)
	if athleteUserID != nil {
		query = query.Where("athlete_user_id = ?", *athleteUserID)
	}
	var feedbacks []dbs.WorkoutFeedback
	if err := query.Order("assigned_exercise_id, set_number, id").Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("error getting session feedback: %w", err)
	}
	if err := d.attachPointsCounts(ctx, feedbacks); err != nil {
		return nil, err
	}
	return feedbacks, nil
}

// attachPointsCounts llena PointsCount en cada feedback con un solo COUNT
// agrupado. Si la consulta de conteo falla no aborta el listado: la distancia y
// los datos del feedback son valiosos aunque el conteo de puntos no venga.
func (d *workoutFeedbackDao) attachPointsCounts(ctx *gin.Context, feedbacks []dbs.WorkoutFeedback) error {
	if len(feedbacks) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(feedbacks))
	for _, f := range feedbacks {
		ids = append(ids, f.ID)
	}

	var rows []struct {
		FeedbackID int64
		Count      int64
	}
	err := d.DB.Model(&dbs.WorkoutFeedbackPoint{}).
		Select("feedback_id, COUNT(*) AS count").
		Where("feedback_id IN ?", ids).
		Group("feedback_id").
		Scan(&rows).Error
	if err != nil {
		return nil
	}

	counts := make(map[int64]int64, len(rows))
	for _, r := range rows {
		counts[r.FeedbackID] = r.Count
	}
	for i := range feedbacks {
		feedbacks[i].PointsCount = counts[feedbacks[i].ID]
	}
	return nil
}

// workoutFeedbackHistorySortColumns mapea los sort values de la API (whitelist
// validada por el service) a columnas SQL del join de HistorySearch.
var workoutFeedbackHistorySortColumns = map[string]string{
	"feedback_date": "wf.session_date",
	"set_number":    "wf.set_number",
	"exercise_name": "ei.name",
}

// historyOrderClause arma el ORDER BY con desempate determinista por wf.id en la
// misma dirección. sort u order fuera de la whitelist caen al default
// (feedback_date DESC); el service valida y responde 400 antes de llegar acá.
func historyOrderClause(sortCol, order string) string {
	col, ok := workoutFeedbackHistorySortColumns[sortCol]
	if !ok {
		col = workoutFeedbackHistorySortColumns["feedback_date"]
	}
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	return col + " " + dir + ", wf.id " + dir
}

// applyHistoryFilters agrega el WHERE dinámico sobre la tabla aliaseada wf.
// withSecondLevel en false deja pasar solo el primer nivel (pools).
// Ojo: GroupID filtra por gcd.group_id, así que el caller debe haber agregado el
// join con group_calendar_days si puede venir seteado.
func applyHistoryFilters(query *gorm.DB, filters WorkoutFeedbackHistoryFilters, withSecondLevel bool) *gorm.DB {
	query = query.Where("wf.deleted_at IS NULL")
	if filters.AthleteUserID != nil {
		query = query.Where("wf.athlete_user_id = ?", *filters.AthleteUserID)
	}
	if filters.TeamID != nil {
		query = query.Where("wf.team_id = ?", *filters.TeamID)
	}
	if filters.GroupID != nil {
		query = query.Where("gcd.group_id = ?", *filters.GroupID)
	}
	if filters.DateFrom != nil {
		query = query.Where("wf.session_date >= ?", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		query = query.Where("wf.session_date <= ?", *filters.DateTo)
	}
	if !withSecondLevel {
		return query
	}
	if filters.ExerciseInstanceID != nil {
		query = query.Where("wf.assigned_exercise_id = ?", *filters.ExerciseInstanceID)
	}
	if filters.SetNumber != nil {
		query = query.Where("wf.set_number = ?", *filters.SetNumber)
	}
	if filters.AthleteFilterUserID != nil {
		query = query.Where("wf.athlete_user_id = ?", *filters.AthleteFilterUserID)
	}
	return query
}

// HistorySearch devuelve las filas enriquecidas del historial. Los joins son
// todos LEFT: los huérfanos (sin día de calendario o sin fila de instancia)
// quedan dentro con los nulls correspondientes (decisión del usuario, D1).
func (d *workoutFeedbackDao) HistorySearch(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters, sortCol string, order string, limit int, offset int) ([]dbs.WorkoutFeedbackHistoryRow, error) {
	query := d.DB.Table("workout_feedback AS wf").
		Select(`wf.id, wf.athlete_user_id, wf.team_id, gcd.group_id, wf.session_date,
			wf.set_number, wf.completion_status, wf.duration_ms, wf.active_duration_ms,
			wf.distance_meters, wf.started_at, wf.ended_at,
			wf.assigned_exercise_id AS exercise_id, ei.name AS exercise_name,
			ei.source_exercise_id AS catalog_exercise_id, si.name AS session_name`).
		Joins("LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = wf.assigned_session_id").
		Joins("LEFT JOIN exercise_instances ei ON ei.id = wf.assigned_exercise_id").
		Joins("LEFT JOIN session_instances si ON si.id = wf.assigned_session_id")
	query = applyHistoryFilters(query, filters, true)
	query = query.Order(historyOrderClause(sortCol, order))
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	var rows []dbs.WorkoutFeedbackHistoryRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error searching workout feedback history: %w", err)
	}
	return rows, nil
}

// HistoryCount cuenta con todos los niveles de filtro, sin paginación. El join
// con group_calendar_days solo se agrega si el filtro de grupo lo necesita
// (los demás filtros son columnas de wf); COUNT DISTINCT por si algún día el
// join llegara a multiplicar filas (D5).
func (d *workoutFeedbackDao) HistoryCount(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters) (int64, error) {
	query := d.DB.Table("workout_feedback AS wf")
	if filters.GroupID != nil {
		query = query.Joins("LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = wf.assigned_session_id")
	}
	var total int64
	err := applyHistoryFilters(query, filters, true).
		Select("COUNT(DISTINCT wf.id) AS total").
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("error counting workout feedback history: %w", err)
	}
	return total, nil
}

// HistoryAvailableAthletes arma el pool de atletas: DISTINCT de atletas con
// feedbacks en el primer nivel, INNER JOIN users (sin fila de user no hay nombre
// útil para el filtro de la UI).
func (d *workoutFeedbackDao) HistoryAvailableAthletes(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters) ([]dbs.IDName, error) {
	query := d.DB.Table("workout_feedback AS wf").
		Select("DISTINCT wf.athlete_user_id AS id, u.name AS name").
		Joins("JOIN users u ON u.id = wf.athlete_user_id")
	if filters.GroupID != nil {
		query = query.Joins("LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = wf.assigned_session_id")
	}
	var athletes []dbs.IDName
	err := applyHistoryFilters(query, filters, false).
		Order("name ASC, id ASC").
		Scan(&athletes).Error
	if err != nil {
		return nil, fmt.Errorf("error listing workout feedback history athletes: %w", err)
	}
	return athletes, nil
}

// HistoryAvailableExercises arma el pool de ejercicios: DISTINCT de instancias
// con feedbacks en el primer nivel, INNER JOIN exercise_instances (un feedback
// huérfano sin instancia no aporta un item filtrable).
func (d *workoutFeedbackDao) HistoryAvailableExercises(ctx *gin.Context, filters WorkoutFeedbackHistoryFilters) ([]dbs.IDName, error) {
	query := d.DB.Table("workout_feedback AS wf").
		Select("DISTINCT wf.assigned_exercise_id AS id, ei.name AS name").
		Joins("JOIN exercise_instances ei ON ei.id = wf.assigned_exercise_id")
	if filters.GroupID != nil {
		query = query.Joins("LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = wf.assigned_session_id")
	}
	var exercises []dbs.IDName
	err := applyHistoryFilters(query, filters, false).
		Order("name ASC, id ASC").
		Scan(&exercises).Error
	if err != nil {
		return nil, fmt.Errorf("error listing workout feedback history exercises: %w", err)
	}
	return exercises, nil
}

// TeamExists indica si existe un team activo (sin soft-delete) con ese id.
func (d *workoutFeedbackDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	return d.membership.TeamExists(ctx, teamID)
}

// IsTeamOwner indica si userID es el owner del team (teams.owner_id).
func (d *workoutFeedbackDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	return d.membership.IsTeamOwner(ctx, teamID, userID)
}

// ExistsUserInTeamOwnedBy indica si targetUserID pertenece (team_users activo) a
// al menos un team cuyo owner sea ownerUserID.
func (d *workoutFeedbackDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	return d.membership.ExistsUserInTeamOwnedBy(ctx, targetUserID, ownerUserID)
}
