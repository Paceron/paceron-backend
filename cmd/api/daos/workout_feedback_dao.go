package daos

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"

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