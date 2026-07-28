package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// GroupDaoInterface define las operaciones de acceso a datos para grupos.
type GroupDaoInterface interface {
	Create(ctx *gin.Context, group *dbs.Group) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Group, error)
	FindByIDAndTeamID(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error)
	GetAll(ctx *gin.Context) ([]dbs.Group, error)
	GetByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Group, error)
	Update(ctx *gin.Context, group *dbs.Group) error
	SoftDelete(ctx *gin.Context, id int64) error
}

type groupDao struct {
	DB *gorm.DB
}

// NewGroupDao crea una nueva instancia de GroupDao.
func NewGroupDao(database *gorm.DB) GroupDaoInterface {
	return &groupDao{
		DB: database,
	}
}

// Create inserta un nuevo grupo en la base de datos.
func (d *groupDao) Create(ctx *gin.Context, group *dbs.Group) error {
	return d.DB.Create(group).Error
}

// FindByID busca un grupo por su ID, excluyendo los eliminados lógicamente.
func (d *groupDao) FindByID(ctx *gin.Context, id int64) (*dbs.Group, error) {
	var group dbs.Group
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&group).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding group: %w", err)
	}
	return &group, nil
}

// FindByIDAndTeamID busca un grupo por ID validando que pertenezca al equipo indicado.
func (d *groupDao) FindByIDAndTeamID(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
	var group dbs.Group
	err := d.DB.Where("id = ? AND team_id = ? AND deleted_at IS NULL", groupID, teamID).First(&group).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding group: %w", err)
	}
	return &group, nil
}

// GetAll devuelve todos los grupos activos.
func (d *groupDao) GetAll(ctx *gin.Context) ([]dbs.Group, error) {
	var groups []dbs.Group
	err := d.DB.Where("deleted_at IS NULL").Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups: %w", err)
	}
	return groups, nil
}

// GetByTeamID devuelve todos los grupos activos de un equipo.
func (d *groupDao) GetByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
	var groups []dbs.Group
	err := d.DB.Where("team_id = ? AND deleted_at IS NULL", teamID).Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups by team: %w", err)
	}
	return groups, nil
}

// Update actualiza los campos de un grupo existente.
func (d *groupDao) Update(ctx *gin.Context, group *dbs.Group) error {
	return d.DB.Save(group).Error
}

// SoftDelete marca un grupo como eliminado lógicamente.
func (d *groupDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Group{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
