package daos

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// ErrAttendanceAlreadyExists indica que la asistencia ya fue registrada: la base
// rechazó el insert por la constraint UNIQUE (team_id, training_session_id, user_id).
var ErrAttendanceAlreadyExists = errors.New("esta asistencia fue previamente registrada")

// postgresUniqueViolation es el SQLSTATE que Postgres reporta al violar una UNIQUE o PK.
const postgresUniqueViolation = "23505"

// AttendanceSearchFilters agrupa los filtros opcionales de búsqueda de asistencias.
// Los valores llegan ya autorizados por el service (team_id obligatorio + rol en el
// team): el DAO solo construye el WHERE.
type AttendanceSearchFilters struct {
	TeamID            *int64
	TrainingSessionID *int64
	UserID            *int64
}

// AttendanceDAOInterface define las operaciones de acceso a datos para asistencias.
type AttendanceDAOInterface interface {
	Create(ctx *gin.Context, attendance *dbs.Attendance) error
	Search(ctx *gin.Context, filters AttendanceSearchFilters) ([]dbs.Attendance, error)
	TeamExists(ctx *gin.Context, teamID int64) (bool, error)
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
	GetTeamUserRole(ctx *gin.Context, teamID, userID int64) (string, error)
}

type attendanceDao struct {
	DB         *gorm.DB
	membership TeamMembershipDAOInterface
}

// NewAttendanceDao crea una nueva instancia de AttendanceDao. Los chequeos de
// ownership/pertenencia de equipos (TeamExists, IsTeamOwner,
// ExistsUserInTeamOwnedBy) delegan en TeamMembershipDAO, compartido con otros
// módulos — ver daos/team_membership_dao.go.
func NewAttendanceDao(database *gorm.DB) AttendanceDAOInterface {
	return &attendanceDao{
		DB:         database,
		membership: NewTeamMembershipDao(database),
	}
}

// Create inserta una asistencia. Si la base rechaza el insert por la constraint
// UNIQUE (asistencia duplicada del mismo (team_id, training_session_id, user_id)),
// devuelve ErrAttendanceAlreadyExists. No se hace un SELECT previo a propósito:
// insert directo + captura de la violación evita race conditions.
func (d *attendanceDao) Create(ctx *gin.Context, attendance *dbs.Attendance) error {
	err := d.DB.Create(attendance).Error
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation {
		return ErrAttendanceAlreadyExists
	}
	return fmt.Errorf("error creating attendance: %w", err)
}

// Search devuelve las asistencias que cumplen los filtros dados (WHERE dinámico).
// Los filtros llegaron ya autorizados por el service.
func (d *attendanceDao) Search(ctx *gin.Context, filters AttendanceSearchFilters) ([]dbs.Attendance, error) {
	query := d.DB.Model(&dbs.Attendance{})
	if filters.TeamID != nil {
		query = query.Where("team_id = ?", *filters.TeamID)
	}
	if filters.TrainingSessionID != nil {
		query = query.Where("training_session_id = ?", *filters.TrainingSessionID)
	}
	if filters.UserID != nil {
		query = query.Where("user_id = ?", *filters.UserID)
	}

	var attendances []dbs.Attendance
	if err := query.Order("id").Find(&attendances).Error; err != nil {
		return nil, fmt.Errorf("error searching attendances: %w", err)
	}
	return attendances, nil
}

// TeamExists indica si existe un team activo (sin soft-delete) con ese id.
func (d *attendanceDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	return d.membership.TeamExists(ctx, teamID)
}

// IsTeamOwner indica si userID es el owner del team (teams.owner_id).
func (d *attendanceDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	return d.membership.IsTeamOwner(ctx, teamID, userID)
}

// ExistsUserInTeamOwnedBy indica si targetUserID pertenece (team_users activo) a
// al menos un team cuyo owner sea ownerUserID. Es la relación que autoriza el
// Escenario C de la matriz de búsqueda: un owner puede ver asistencias de un
// corredor solo si ese corredor está en un equipo que él administra.
func (d *attendanceDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	return d.membership.ExistsUserInTeamOwnedBy(ctx, targetUserID, ownerUserID)
}

// GetTeamUserRole devuelve el rol (entrenador/corredor) de userID en teamID si
// pertenece activamente ("" si no es miembro).
func (d *attendanceDao) GetTeamUserRole(ctx *gin.Context, teamID, userID int64) (string, error) {
	return d.membership.GetTeamUserRole(ctx, teamID, userID)
}
