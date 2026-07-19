package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type PermissionDaoInterface interface {
	Create(ctx *gin.Context, permission *dbs.Permission) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Permission, error)
	FindByName(ctx *gin.Context, name string) (*dbs.Permission, error)
	GetAll(ctx *gin.Context) ([]dbs.Permission, error)
	Update(ctx *gin.Context, permission *dbs.Permission) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type permissionDao struct {
	DB *gorm.DB
}

func NewPermissionDao(database *gorm.DB) PermissionDaoInterface {
	return &permissionDao{
		DB: database,
	}
}

func (d *permissionDao) Create(ctx *gin.Context, permission *dbs.Permission) error {
	return d.DB.Create(permission).Error
}

func (d *permissionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Permission, error) {
	var permission dbs.Permission
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&permission).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding permission by id: %w", err)
	}
	return &permission, nil
}

func (d *permissionDao) FindByName(ctx *gin.Context, name string) (*dbs.Permission, error) {
	var permission dbs.Permission
	err := d.DB.Where("name = ? AND deleted_at IS NULL", name).First(&permission).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding permission by name: %w", err)
	}
	return &permission, nil
}

func (d *permissionDao) GetAll(ctx *gin.Context) ([]dbs.Permission, error) {
	var permissions []dbs.Permission
	err := d.DB.Where("deleted_at IS NULL").Find(&permissions).Error
	if err != nil {
		return nil, fmt.Errorf("error getting all permissions: %w", err)
	}
	return permissions, nil
}

func (d *permissionDao) Update(ctx *gin.Context, permission *dbs.Permission) error {
	return d.DB.Save(permission).Error
}

func (d *permissionDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Permission{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
