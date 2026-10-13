package daos

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/competition"
)

// ErrCompetitionNotFound se devuelve cuando la competencia no existe. El
// controller lo mapea a 404.
var ErrCompetitionNotFound = errors.New("competencia no encontrada")

func isGormNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// CompetitionDAOInterface es el acceso a datos de competencias (gamificacion-core
// D8/D9). La integridad de participants la gobierna el service: Create inserta
// competencia + participantes en una transacción.
type CompetitionDAOInterface interface {
	// Chequeos de guard (propios para evitar dependencias entre DAOs).
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error)
	// CountTeamGroupMembers cuenta cuántos de los userIDs pasados son miembros
	// activos de algún grupo del team (criterio del leaderboard guard).
	CountTeamGroupMembers(ctx *gin.Context, teamID int64, userIDs []int64) (int64, error)
	// GroupBelongsToTeam valida que el group_id del body apunte a un grupo
	// del mismo team.
	GroupBelongsToTeam(ctx *gin.Context, groupID, teamID int64) (bool, error)

	// Create inserta la competencia y sus participantes en una transacción.
	Create(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Competition, error)
	// ListParticipantsByCompetitionIDs trae las membresías en batch (sin N+1
	// en detalle ni listado).
	ListParticipantsByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionParticipant, error)
	ListWinnersByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionWinner, error)
	// ListByTeam aplica los filtros opcionales status y group_id.
	ListByTeam(ctx *gin.Context, teamID int64, status *string, groupID *int64) ([]dbs.Competition, error)
	// IsParticipant indica si el usuario está entre los participantes.
	IsParticipant(ctx *gin.Context, competitionID, userID int64) (bool, error)
	// Approve / Reject / Finish aplican la transición de status (guard de
	// status en el WHERE: solo corren sobre el status origen válido) y
	// devuelven si hubo cambio.
	Approve(ctx *gin.Context, competitionID, approvedByUserID int64) (bool, error)
	Reject(ctx *gin.Context, competitionID int64, reason *string) (bool, error)
	Finish(ctx *gin.Context, competitionID int64, finishedAt time.Time) (bool, error)
	// Delete borra físicamente la competencia junto con sus participantes
	// (DELETE cancel D9; borrado físico).
	Delete(ctx *gin.Context, competitionID int64) error

	// HasUnseenTeam es el EXISTS de novedades (D11) para el listado embebido.
	HasUnseenTeam(ctx *gin.Context, userID, teamID int64) (bool, error)

	// AggregateFeedbackWindow acumula métricas de feedback por atleta dentro
	// de la ventana del team (detalle on-demand D9).
	AggregateFeedbackWindow(ctx *gin.Context, teamID int64, participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error)
}

type competitionDao struct {
	DB *gorm.DB
}

// NewCompetitionDao crea una nueva instancia de CompetitionDao.
func NewCompetitionDao(database *gorm.DB) CompetitionDAOInterface {
	return &competitionDao{DB: database}
}

func (d *competitionDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Team{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition team owner: %w", err)
	}
	return count > 0, nil
}

func (d *competitionDao) IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.GroupUser{}).
		Joins("JOIN groups ON groups.id = group_users.group_id AND groups.deleted_at IS NULL").
		Where("groups.team_id = ? AND group_users.user_id = ? AND group_users.deleted_at IS NULL AND (group_users.date_end IS NULL OR group_users.date_end > NOW())", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition group membership: %w", err)
	}
	return count > 0, nil
}

func (d *competitionDao) CountTeamGroupMembers(ctx *gin.Context, teamID int64, userIDs []int64) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	var count int64
	err := d.DB.Model(&dbs.GroupUser{}).
		Joins("JOIN groups ON groups.id = group_users.group_id AND groups.deleted_at IS NULL").
		Where("groups.team_id = ? AND group_users.user_id IN ? AND group_users.deleted_at IS NULL AND (group_users.date_end IS NULL OR group_users.date_end > NOW())", teamID, userIDs).
		Distinct("group_users.user_id").
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("error counting competition team members: %w", err)
	}
	return count, nil
}

func (d *competitionDao) GroupBelongsToTeam(ctx *gin.Context, groupID, teamID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Group{}).
		Where("id = ? AND team_id = ? AND deleted_at IS NULL", groupID, teamID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition group belongs to team: %w", err)
	}
	return count > 0, nil
}

// Create inserta la competencia y sus participantes en una transacción. El
// fallo de cualquiera de las dos escrituras revierte ambas.
func (d *competitionDao) Create(ctx *gin.Context, comp *dbs.Competition, participantIDs []int64) error {
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comp).Error; err != nil {
			return fmt.Errorf("error creating competition: %w", err)
		}
		participants := make([]dbs.CompetitionParticipant, 0, len(participantIDs))
		for _, id := range participantIDs {
			participants = append(participants, dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: id})
		}
		if err := tx.Create(&participants).Error; err != nil {
			return fmt.Errorf("error creating competition participants: %w", err)
		}
		return nil
	})
	return err
}

func (d *competitionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Competition, error) {
	var comp dbs.Competition
	err := d.DB.Where("id = ?", id).First(&comp).Error
	if err != nil {
		if isGormNotFound(err) {
			return nil, ErrCompetitionNotFound
		}
		return nil, fmt.Errorf("error finding competition: %w", err)
	}
	return &comp, nil
}

func (d *competitionDao) ListParticipantsByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionParticipant, error) {
	if len(competitionIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionParticipant
	err := d.DB.Where("competition_id IN ?", competitionIDs).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition participants: %w", err)
	}
	return rows, nil
}

func (d *competitionDao) ListWinnersByCompetitionIDs(ctx *gin.Context, competitionIDs []int64) ([]dbs.CompetitionWinner, error) {
	if len(competitionIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionWinner
	err := d.DB.Where("competition_id IN ?", competitionIDs).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competition winners: %w", err)
	}
	return rows, nil
}

func (d *competitionDao) ListByTeam(ctx *gin.Context, teamID int64, status *string, groupID *int64) ([]dbs.Competition, error) {
	q := d.DB.Where("team_id = ?", teamID)
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	var comps []dbs.Competition
	err := q.Order("created_at DESC").Find(&comps).Error
	if err != nil {
		return nil, fmt.Errorf("error listing competitions: %w", err)
	}
	return comps, nil
}

func (d *competitionDao) IsParticipant(ctx *gin.Context, competitionID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.CompetitionParticipant{}).
		Where("competition_id = ? AND user_id = ?", competitionID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition participant: %w", err)
	}
	return count > 0, nil
}

func (d *competitionDao) Approve(ctx *gin.Context, competitionID, approvedByUserID int64) (bool, error) {
	res := d.DB.Model(&dbs.Competition{}).
		Where("id = ? AND status = ?", competitionID, competition.StatusPendingApproval).
		Updates(map[string]interface{}{
			"status":              competition.StatusActive,
			"approved_by_user_id": approvedByUserID,
		})
	if res.Error != nil {
		return false, fmt.Errorf("error approving competition: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (d *competitionDao) Reject(ctx *gin.Context, competitionID int64, reason *string) (bool, error) {
	res := d.DB.Model(&dbs.Competition{}).
		Where("id = ? AND status = ?", competitionID, competition.StatusPendingApproval).
		Updates(map[string]interface{}{
			"status":            competition.StatusRejected,
			"rejection_reason":  reason,
		})
	if res.Error != nil {
		return false, fmt.Errorf("error rejecting competition: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (d *competitionDao) Finish(ctx *gin.Context, competitionID int64, finishedAt time.Time) (bool, error) {
	res := d.DB.Model(&dbs.Competition{}).
		Where("id = ? AND status = ?", competitionID, competition.StatusActive).
		Updates(map[string]interface{}{
			"status":      competition.StatusFinished,
			"finished_at": finishedAt,
		})
	if res.Error != nil {
		return false, fmt.Errorf("error finishing competition: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (d *competitionDao) Delete(ctx *gin.Context, competitionID int64) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("competition_id = ?", competitionID).Delete(&dbs.CompetitionParticipant{}).Error; err != nil {
			return fmt.Errorf("error deleting competition participants: %w", err)
		}
		if err := tx.Where("id = ?", competitionID).Delete(&dbs.Competition{}).Error; err != nil {
			return fmt.Errorf("error deleting competition: %w", err)
		}
		return nil
	})
}

func (d *competitionDao) HasUnseenTeam(ctx *gin.Context, userID, teamID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.GamificationUnseen{}).
		Where("user_id = ? AND section = ? AND team_id = ?", userID, constants.GamificationSectionCompetitions, teamID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking competition unseen: %w", err)
	}
	return count > 0, nil
}

// AggregateFeedbackWindow acumula las métricas fuente por atleta dentro de la
// ventana. La ventana de session_date es de fechas: se pasa medianoche local
// de ambos bordes (inclusive) — el granularity de workout_feedback es date.
func (d *competitionDao) AggregateFeedbackWindow(ctx *gin.Context, teamID int64, participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
	if len(participantIDs) == 0 {
		return nil, nil
	}
	var rows []dbs.CompetitionMetricAgg
	err := d.DB.Table("workout_feedback AS wf").
		Select(`wf.athlete_user_id AS user_id,
			SUM(wf.distance_meters) AS sum_distance,
			SUM(wf.duration_ms) AS sum_duration_ms,
			MAX(wf.peak_speed_kph) AS max_peak`).
		Where("wf.deleted_at IS NULL AND wf.team_id = ? AND wf.athlete_user_id IN ?", teamID, participantIDs).
		Where("wf.session_date >= ? AND wf.session_date <= ?", startAt, endAt).
		Group("wf.athlete_user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error aggregating competition feedback window: %w", err)
	}
	return rows, nil
}
