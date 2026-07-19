package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type TierPermissionDaoInterface interface {
	Create(ctx *gin.Context, tierPermission *dbs.TierPermission) error
	FindByTierAndPermission(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error)
	FindByTierID(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error)
	SoftDelete(ctx *gin.Context, id int64) error
}

type tierPermissionDao struct {
	DB *gorm.DB
}

func NewTierPermissionDao(database *gorm.DB) TierPermissionDaoInterface {
	return &tierPermissionDao{
		DB: database,
	}
}

func (d *tierPermissionDao) Create(ctx *gin.Context, tierPermission *dbs.TierPermission) error {
	return d.DB.Create(tierPermission).Error
}

func (d *tierPermissionDao) FindByTierAndPermission(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
	var tierPermission dbs.TierPermission
	err := d.DB.Where("tier_id = ? AND permission_id = ? AND deleted_at IS NULL", tierID, permissionID).First(&tierPermission).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding tier permission: %w", err)
	}
	return &tierPermission, nil
}

func (d *tierPermissionDao) FindByTierID(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
	var tierPermissions []dbs.TierPermission
	err := d.DB.Where("tier_id = ? AND deleted_at IS NULL", tierID).Find(&tierPermissions).Error
	if err != nil {
		return nil, fmt.Errorf("error finding tier permissions: %w", err)
	}
	return tierPermissions, nil
}

func (d *tierPermissionDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.TierPermission{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
