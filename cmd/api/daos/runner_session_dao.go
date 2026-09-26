package daos

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// ErrRunnerSessionNotFound se devuelve cuando no existe un estado de sesión
// para la combinación (session_instance_id, athlete_user_id). El service lo
// mapea a 404.
var ErrRunnerSessionNotFound = errors.New("estado de sesión no encontrado")

// RunnerSessionDAOInterface define las operaciones de acceso a datos del estado
// de sesión del corredor (runner_session).
type RunnerSessionDAOInterface interface {
	// Create inserta la fila en wip. Idempotente vía ON CONFLICT DO NOTHING:
	// devuelve true si se insertó, false si la fila ya existía (no duplica ni
	// pisa los valores existentes).
	Create(ctx *gin.Context, r *dbs.RunnerSession) (bool, error)
	// GetBySessionAndAthlete devuelve el estado de la sesión para ese atleta, o
	// ErrRunnerSessionNotFound si no existe.
	GetBySessionAndAthlete(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error)
	// Finish pasa la fila a finished con el end_date provisto. Solo marca si
	// estaba en wip (WHERE status = 'wip'); si ya estaba finished el update no
	// toca nada. El service decide con el GET previo; este WHERE es el guard.
	Finish(ctx *gin.Context, r *dbs.RunnerSession) error
	// SessionInstanceExists indica si existe la sesión asignada (id). La FK de
	// runner_session es opaca; la existencia la valida el service al crear.
	SessionInstanceExists(ctx *gin.Context, sessionInstanceID int64) (bool, error)

	// Chequeos de membresía de equipo compartidos (TeamMembershipDAO): los usa
	// el service para la matriz de autorización del entrenador. Delegados.
	TeamExists(ctx *gin.Context, teamID int64) (bool, error)
	ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
}

type runnerSessionDao struct {
	DB         *gorm.DB
	membership TeamMembershipDAOInterface
}

// NewRunnerSessionDao crea una nueva instancia de RunnerSessionDao.
func NewRunnerSessionDao(database *gorm.DB) RunnerSessionDAOInterface {
	return &runnerSessionDao{
		DB:         database,
		membership: NewTeamMembershipDao(database),
	}
}

// Create intenta insertar la fila. Si ya existe (constraint único
// uq_runner_session_session_athlete) el ON CONFLICT DO NOTHING no inserta ni
// falla: devuelve created=false y el caller devuelve el estado actual.
func (d *runnerSessionDao) Create(ctx *gin.Context, r *dbs.RunnerSession) (bool, error) {
	res := d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_instance_id"}, {Name: "athlete_user_id"}},
		DoNothing: true,
	}).Create(r)
	if res.Error != nil {
		return false, fmt.Errorf("error creating runner session: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// GetBySessionAndAthlete devuelve el estado de la sesión del atleta. Sin fila →
// ErrRunnerSessionNotFound.
func (d *runnerSessionDao) GetBySessionAndAthlete(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
	var rs dbs.RunnerSession
	err := d.DB.
		Where("session_instance_id = ? AND athlete_user_id = ?", sessionInstanceID, athleteUserID).
		First(&rs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRunnerSessionNotFound
		}
		return nil, fmt.Errorf("error getting runner session: %w", err)
	}
	return &rs, nil
}

// Finish marca la fila como finished con el end_date provisto. El WHERE
// status='wip' garantiza que jamás se reescribe una sesión ya finalizada; si no
// matchea (ya finished o fila inexistente) no hay error, el service re-lee.
func (d *runnerSessionDao) Finish(ctx *gin.Context, r *dbs.RunnerSession) error {
	res := d.DB.Model(&dbs.RunnerSession{}).
		Where("id = ? AND status = 'wip'", r.ID).
		Updates(map[string]interface{}{
			"status":     "finished",
			"end_date":   r.EndDate,
			"updated_at": r.UpdatedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("error finishing runner session: %w", res.Error)
	}
	return nil
}

// SessionInstanceExists valida la existencia de la sesión asignada antes de
// crear el estado (la FK runner_session.session_instance_id es opaca).
func (d *runnerSessionDao) SessionInstanceExists(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.SessionInstance{}).
		Where("id = ?", sessionInstanceID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking session instance exists: %w", err)
	}
	return count > 0, nil
}

// TeamExists indica si existe un team activo (sin soft-delete) con ese id.
func (d *runnerSessionDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	return d.membership.TeamExists(ctx, teamID)
}

// ExistsUserInTeamOwnedBy indica si targetUserID pertenece (team_users activo) a
// al menos un team cuyo owner sea ownerUserID.
func (d *runnerSessionDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	return d.membership.ExistsUserInTeamOwnedBy(ctx, targetUserID, ownerUserID)
}