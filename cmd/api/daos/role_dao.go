package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type RoleDaoInterface interface {
	Create(ctx *gin.Context, role *dbs.Role) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Role, error)
	FindByName(ctx *gin.Context, name string) (*dbs.Role, error)
	GetAll(ctx *gin.Context) ([]dbs.Role, error)
	Update(ctx *gin.Context, role *dbs.Role) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type roleDao struct {
	DB *gorm.DB
}

func NewRoleDao(database *gorm.DB) RoleDaoInterface {
	return &roleDao{
		DB: database,
	}
}

func (d *roleDao) Create(ctx *gin.Context, role *dbs.Role) error {
	return d.DB.Create(role).Error
}

func (d *roleDao) FindByID(ctx *gin.Context, id int64) (*dbs.Role, error) {
	var role dbs.Role
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&role).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding role by id: %w", err)
	}
	return &role, nil
}

func (d *roleDao) FindByName(ctx *gin.Context, name string) (*dbs.Role, error) {
	var role dbs.Role
	err := d.DB.Where("name = ? AND deleted_at IS NULL", name).First(&role).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding role by name: %w", err)
	}
	return &role, nil
}

func (d *roleDao) GetAll(ctx *gin.Context) ([]dbs.Role, error) {
	var roles []dbs.Role
	err := d.DB.Where("deleted_at IS NULL").Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("error getting all roles: %w", err)
	}
	return roles, nil
}

func (d *roleDao) Update(ctx *gin.Context, role *dbs.Role) error {
	return d.DB.Save(role).Error
}

func (d *roleDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Role{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
