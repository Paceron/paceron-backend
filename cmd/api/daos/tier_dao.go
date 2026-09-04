package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type TierDaoInterface interface {
	Create(ctx *gin.Context, tier *dbs.Tier) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Tier, error)
	FindByNameAndRole(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error)
	FindByName(ctx *gin.Context, name string) (*dbs.Tier, error)
	FindLowestByRole(ctx *gin.Context, roleID int64) (*dbs.Tier, error)
	GetAll(ctx *gin.Context) ([]dbs.Tier, error)
	Update(ctx *gin.Context, tier *dbs.Tier) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type tierDao struct {
	DB *gorm.DB
}

func NewTierDao(database *gorm.DB) TierDaoInterface {
	return &tierDao{
		DB: database,
	}
}

func (d *tierDao) Create(ctx *gin.Context, tier *dbs.Tier) error {
	return d.DB.Create(tier).Error
}

func (d *tierDao) FindByID(ctx *gin.Context, id int64) (*dbs.Tier, error) {
	var tier dbs.Tier
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&tier).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding tier by id: %w", err)
	}
	return &tier, nil
}

func (d *tierDao) FindByNameAndRole(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
	var tier dbs.Tier
	err := d.DB.Where("name = ? AND role_id = ? AND deleted_at IS NULL", name, roleID).First(&tier).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding tier by name and role: %w", err)
	}
	return &tier, nil
}

func (d *tierDao) FindByName(ctx *gin.Context, name string) (*dbs.Tier, error) {
	var tier dbs.Tier
	err := d.DB.Where("name = ? AND deleted_at IS NULL", name).First(&tier).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding tier by name: %w", err)
	}
	return &tier, nil
}

// FindLowestByRole devuelve el tier de menor jerarquía (base) de un rol — sirve
// como tier de acceso inicial cuando se asigna un rol con tier pago (D2).
func (d *tierDao) FindLowestByRole(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
	var tier dbs.Tier
	err := d.DB.
		Where("role_id = ? AND deleted_at IS NULL", roleID).
		Order("hierarchy ASC, id ASC").
		First(&tier).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding lowest tier by role: %w", err)
	}
	return &tier, nil
}

func (d *tierDao) GetAll(ctx *gin.Context) ([]dbs.Tier, error) {
	var tiers []dbs.Tier
	err := d.DB.Where("deleted_at IS NULL").Find(&tiers).Error
	if err != nil {
		return nil, fmt.Errorf("error getting all tiers: %w", err)
	}
	return tiers, nil
}

func (d *tierDao) Update(ctx *gin.Context, tier *dbs.Tier) error {
	return d.DB.Save(tier).Error
}

func (d *tierDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Tier{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
