package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type UserRoleDaoInterface interface {
	Create(ctx *gin.Context, userRole *dbs.UserRole) error
	FindByUserAndRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error)
	FindByUserID(ctx *gin.Context, userID int64) ([]dbs.UserRole, error)
	SoftDelete(ctx *gin.Context, id int64) error
	UpdateTier(ctx *gin.Context, userID, roleID, tierID int64) error
}

type userRoleDao struct {
	DB *gorm.DB
}

func NewUserRoleDao(database *gorm.DB) UserRoleDaoInterface {
	return &userRoleDao{
		DB: database,
	}
}

func (d *userRoleDao) Create(ctx *gin.Context, userRole *dbs.UserRole) error {
	return d.DB.Create(userRole).Error
}

func (d *userRoleDao) FindByUserAndRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
	var userRole dbs.UserRole
	err := d.DB.Where("user_id = ? AND role_id = ? AND deleted_at IS NULL", userID, roleID).First(&userRole).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding user role: %w", err)
	}
	return &userRole, nil
}

func (d *userRoleDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
	var userRoles []dbs.UserRole
	err := d.DB.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&userRoles).Error
	if err != nil {
		return nil, fmt.Errorf("error finding user roles: %w", err)
	}
	return userRoles, nil
}

func (d *userRoleDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.UserRole{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}

// UpdateTier actualiza el tier de la asignación vigente de (user_id, role_id).
// Se usa para la sincronización del acceso: tier pago al confirmarse la cuota #1
// (D3) y tier gratis inmediato al cambiar (D4).
func (d *userRoleDao) UpdateTier(ctx *gin.Context, userID, roleID, tierID int64) error {
	return d.DB.Model(&dbs.UserRole{}).
		Where("user_id = ? AND role_id = ? AND deleted_at IS NULL", userID, roleID).
		Update("tier_id", tierID).Error
}
