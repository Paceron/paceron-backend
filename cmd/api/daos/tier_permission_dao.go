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
	ListPermissionNamesByTier(ctx *gin.Context, tierID int64) ([]TierPermissionName, error)
	ListPermissionNamesByTiers(ctx *gin.Context, tierIDs []int64) ([]TierPermissionNameByTier, error)
	SoftDelete(ctx *gin.Context, id int64) error
}

// TierPermissionName es la proyección (permission_id, name) del listado de un tier.
type TierPermissionName struct {
	PermissionID   int64
	PermissionName string
}

// TierPermissionNameByTier es la fila del listado batch. PermissionName queda nil
// cuando el tier_permission apunta a un permiso inexistente o soft-deleted: la
// fila se conserva para poder reportar el missingData igual que FindByTierID +
// FindByID.
type TierPermissionNameByTier struct {
	TierID         int64
	PermissionID   int64
	PermissionName *string
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

func (d *tierPermissionDao) ListPermissionNamesByTier(ctx *gin.Context, tierID int64) ([]TierPermissionName, error) {
	var names []TierPermissionName
	err := d.DB.
		Table("tier_permissions AS tp").
		Select("tp.permission_id AS permission_id, p.name AS permission_name").
		Joins("JOIN permissions p ON p.id = tp.permission_id").
		Where("tp.tier_id = ? AND tp.deleted_at IS NULL AND p.deleted_at IS NULL", tierID).
		Order("p.id ASC").
		Scan(&names).Error
	if err != nil {
		return nil, fmt.Errorf("error listing permission names of tier: %w", err)
	}
	return names, nil
}

// ListPermissionNamesByTiers es la variante batch de ListPermissionNamesByTier.
// El LEFT JOIN conserva los tier_permissions cuyo permiso ya no existe, para que
// el service pueda seguir reportando "permiso_id=N no configurado" en vez de
// perderlo en silencio. Ordena por tier y p.id ASC.
func (d *tierPermissionDao) ListPermissionNamesByTiers(ctx *gin.Context, tierIDs []int64) ([]TierPermissionNameByTier, error) {
	if len(tierIDs) == 0 {
		return nil, nil
	}
	var rows []TierPermissionNameByTier
	err := d.DB.
		Table("tier_permissions AS tp").
		Select("tp.tier_id AS tier_id, tp.permission_id AS permission_id, p.name AS permission_name").
		Joins("LEFT JOIN permissions p ON p.id = tp.permission_id AND p.deleted_at IS NULL").
		Where("tp.tier_id IN ? AND tp.deleted_at IS NULL", tierIDs).
		Order("tp.tier_id ASC, p.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing permission names of tiers: %w", err)
	}
	return rows, nil
}

func (d *tierPermissionDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.TierPermission{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
