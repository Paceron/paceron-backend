package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// JoinRequestDaoInterface define las operaciones de acceso a datos para
// solicitudes de ingreso de un corredor a un equipo.
type JoinRequestDaoInterface interface {
	Create(ctx *gin.Context, jr *dbs.JoinRequest) error
	FindByID(ctx *gin.Context, id int64) (*dbs.JoinRequest, error)
	FindPendingByTeamAndUser(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error)
	FindPendingByTeam(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error)
	FindByUser(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error)
	UpdateStatus(ctx *gin.Context, id int64, status string) error
	Delete(ctx *gin.Context, id int64) error
	CountPendingByOwner(ctx *gin.Context, ownerID int64) (int64, error)
}

type joinRequestDao struct {
	DB *gorm.DB
}

// NewJoinRequestDao crea una nueva instancia de JoinRequestDao.
func NewJoinRequestDao(database *gorm.DB) JoinRequestDaoInterface {
	return &joinRequestDao{DB: database}
}

func (d *joinRequestDao) Create(ctx *gin.Context, jr *dbs.JoinRequest) error {
	return d.DB.Create(jr).Error
}

func (d *joinRequestDao) FindByID(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
	var jr dbs.JoinRequest
	err := d.DB.Where("id = ?", id).First(&jr).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding join request: %w", err)
	}
	return &jr, nil
}

func (d *joinRequestDao) FindPendingByTeamAndUser(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error) {
	var jr dbs.JoinRequest
	err := d.DB.Where("team_id = ? AND runner_id = ? AND status = ?", teamID, runnerID, string(constants.InvitationStatusPending)).First(&jr).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding pending join request: %w", err)
	}
	return &jr, nil
}

func (d *joinRequestDao) FindPendingByTeam(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error) {
	var requests []dbs.JoinRequest
	err := d.DB.Where("team_id = ? AND status = ?", teamID, string(constants.InvitationStatusPending)).Order("id").Find(&requests).Error
	if err != nil {
		return nil, fmt.Errorf("error finding pending join requests: %w", err)
	}
	return requests, nil
}

func (d *joinRequestDao) FindByUser(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error) {
	var requests []dbs.JoinRequest
	err := d.DB.Where("runner_id = ?", runnerID).Order("id DESC").Find(&requests).Error
	if err != nil {
		return nil, fmt.Errorf("error finding join requests by user: %w", err)
	}
	return requests, nil
}

func (d *joinRequestDao) UpdateStatus(ctx *gin.Context, id int64, status string) error {
	return d.DB.Model(&dbs.JoinRequest{}).Where("id = ?", id).Update("status", status).Error
}

func (d *joinRequestDao) Delete(ctx *gin.Context, id int64) error {
	return d.DB.Delete(&dbs.JoinRequest{}, id).Error
}

func (d *joinRequestDao) CountPendingByOwner(ctx *gin.Context, ownerID int64) (int64, error) {
	var count int64
	err := d.DB.Model(&dbs.JoinRequest{}).
		Joins("JOIN teams ON teams.id = join_requests.team_id").
		Where("teams.owner_id = ? AND join_requests.status = ?", ownerID, string(constants.InvitationStatusPending)).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("error counting pending join requests: %w", err)
	}
	return count, nil
}
