package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// InvitationDaoInterface define las operaciones de acceso a datos para invitaciones de equipo.
type InvitationDaoInterface interface {
	Create(ctx *gin.Context, invitation *dbs.Invitation) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Invitation, error)
	FindPendingByTeamAndInvitee(ctx *gin.Context, teamID, inviteeID int64) (*dbs.Invitation, error)
	FindPendingByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error)
	UpdateStatus(ctx *gin.Context, id int64, status string, respondedAt time.Time) error
	SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error
}

type invitationDao struct {
	DB *gorm.DB
}

// NewInvitationDao crea una nueva instancia de InvitationDao.
func NewInvitationDao(database *gorm.DB) InvitationDaoInterface {
	return &invitationDao{
		DB: database,
	}
}

// Create inserta una nueva invitación en la base de datos.
func (d *invitationDao) Create(ctx *gin.Context, invitation *dbs.Invitation) error {
	return d.DB.Create(invitation).Error
}

// FindByID busca una invitación por su ID, excluyendo las eliminadas lógicamente.
func (d *invitationDao) FindByID(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
	var invitation dbs.Invitation
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding invitation: %w", err)
	}
	return &invitation, nil
}

// FindPendingByTeamAndInvitee busca una invitación pendiente para un usuario en un equipo.
func (d *invitationDao) FindPendingByTeamAndInvitee(ctx *gin.Context, teamID, inviteeID int64) (*dbs.Invitation, error) {
	var invitation dbs.Invitation
	err := d.DB.Where("team_id = ? AND invitee_id = ? AND status = ? AND deleted_at IS NULL",
		teamID, inviteeID, string(constants.InvitationStatusPending)).First(&invitation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding pending invitation: %w", err)
	}
	return &invitation, nil
}

// FindPendingByTeamID devuelve todas las invitaciones pendientes de un equipo.
func (d *invitationDao) FindPendingByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error) {
	var invitations []dbs.Invitation
	err := d.DB.Where("team_id = ? AND status = ? AND deleted_at IS NULL", teamID, string(constants.InvitationStatusPending)).
		Find(&invitations).Error
	if err != nil {
		return nil, fmt.Errorf("error finding pending invitations: %w", err)
	}
	return invitations, nil
}

// UpdateStatus actualiza el estado de una invitación y el momento en que fue respondida.
func (d *invitationDao) UpdateStatus(ctx *gin.Context, id int64, status string, respondedAt time.Time) error {
	return d.DB.Model(&dbs.Invitation{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "responded_at": respondedAt}).Error
}

// SoftDeleteByTeamID marca como eliminadas lógicamente todas las invitaciones de un
// equipo (cualquier estado), usado en cascada al eliminar el equipo.
func (d *invitationDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	return d.DB.Model(&dbs.Invitation{}).
		Where("team_id = ? AND deleted_at IS NULL", teamID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}
