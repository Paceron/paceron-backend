package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// GroupUserDaoInterface define las operaciones de acceso a datos para la asociación usuario-grupo.
type GroupUserDaoInterface interface {
	Create(ctx *gin.Context, groupUser *dbs.GroupUser) error
	FindByGroupAndUser(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error)
	FindByGroupID(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error)
	FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error)
	SoftDelete(ctx *gin.Context, id int64) error
	SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error
}

type groupUserDao struct {
	DB *gorm.DB
}

// NewGroupUserDao crea una nueva instancia de GroupUserDao.
func NewGroupUserDao(database *gorm.DB) GroupUserDaoInterface {
	return &groupUserDao{
		DB: database,
	}
}

// Create inserta una nueva asociación usuario-grupo en la base de datos.
func (d *groupUserDao) Create(ctx *gin.Context, groupUser *dbs.GroupUser) error {
	return d.DB.Create(groupUser).Error
}

// FindByGroupAndUser busca una asociación por grupo y usuario, excluyendo las eliminadas lógicamente.
func (d *groupUserDao) FindByGroupAndUser(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
	var groupUser dbs.GroupUser
	err := d.DB.Where("group_id = ? AND user_id = ? AND deleted_at IS NULL", groupID, userID).First(&groupUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding group user: %w", err)
	}
	return &groupUser, nil
}

// FindByGroupID devuelve todas las asociaciones activas de un grupo.
func (d *groupUserDao) FindByGroupID(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
	var groupUsers []dbs.GroupUser
	err := d.DB.Where("group_id = ? AND deleted_at IS NULL", groupID).Find(&groupUsers).Error
	if err != nil {
		return nil, fmt.Errorf("error finding group users: %w", err)
	}
	return groupUsers, nil
}

// FindByUserID devuelve todas las asociaciones activas de un usuario.
func (d *groupUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	var groupUsers []dbs.GroupUser
	err := d.DB.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&groupUsers).Error
	if err != nil {
		return nil, fmt.Errorf("error finding user groups: %w", err)
	}
	return groupUsers, nil
}

// SoftDelete marca una asociación usuario-grupo como eliminada lógicamente.
func (d *groupUserDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.GroupUser{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}

// SoftDeleteByTeamID marca como eliminadas lógicamente todas las asociaciones
// usuario-grupo activas de los grupos de un equipo (usado en cascada al eliminar
// el equipo). Resuelve los grupos vía subquery en vez de recibir una lista de IDs,
// para que el caller no tenga que orquestar el fetch de grupos primero.
func (d *groupUserDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	return d.DB.Model(&dbs.GroupUser{}).
		Where("deleted_at IS NULL AND group_id IN (SELECT id FROM groups WHERE team_id = ?)", teamID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}
