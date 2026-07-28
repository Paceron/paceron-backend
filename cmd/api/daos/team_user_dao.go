package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// TeamUserDaoInterface define las operaciones de acceso a datos para la asociación usuario-equipo.
type TeamUserDaoInterface interface {
	Create(ctx *gin.Context, teamUser *dbs.TeamUser) error
	FindByTeamAndUser(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error)
	FindByTeamID(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error)
	FindByUserID(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error)
	CountActiveByTeam(ctx *gin.Context, teamID int64) (int64, error)
	HasOwnerByTeam(ctx *gin.Context, teamID int64) (bool, error)
	SoftDelete(ctx *gin.Context, id int64) error
}

type teamUserDao struct {
	DB *gorm.DB
}

// NewTeamUserDao crea una nueva instancia de TeamUserDao.
func NewTeamUserDao(database *gorm.DB) TeamUserDaoInterface {
	return &teamUserDao{
		DB: database,
	}
}

// Create inserta una nueva asociación usuario-equipo en la base de datos.
func (d *teamUserDao) Create(ctx *gin.Context, teamUser *dbs.TeamUser) error {
	return d.DB.Create(teamUser).Error
}

// FindByTeamAndUser busca una asociación por equipo y usuario, excluyendo las eliminadas lógicamente.
func (d *teamUserDao) FindByTeamAndUser(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
	var teamUser dbs.TeamUser
	err := d.DB.Where("team_id = ? AND user_id = ? AND deleted_at IS NULL", teamID, userID).First(&teamUser).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding team user: %w", err)
	}
	return &teamUser, nil
}

// FindByTeamID devuelve todas las asociaciones activas de un equipo.
func (d *teamUserDao) FindByTeamID(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
	var teamUsers []dbs.TeamUser
	err := d.DB.Where("team_id = ? AND deleted_at IS NULL", teamID).Find(&teamUsers).Error
	if err != nil {
		return nil, fmt.Errorf("error finding team users: %w", err)
	}
	return teamUsers, nil
}

// FindByUserID devuelve todas las asociaciones activas de un usuario.
func (d *teamUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error) {
	var teamUsers []dbs.TeamUser
	err := d.DB.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&teamUsers).Error
	if err != nil {
		return nil, fmt.Errorf("error finding user teams: %w", err)
	}
	return teamUsers, nil
}

// CountActiveByTeam cuenta los miembros activos de un equipo.
func (d *teamUserDao) CountActiveByTeam(ctx *gin.Context, teamID int64) (int64, error) {
	var count int64
	err := d.DB.Model(&dbs.TeamUser{}).Where("team_id = ? AND deleted_at IS NULL", teamID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("error counting team members: %w", err)
	}
	return count, nil
}

// HasOwnerByTeam verifica si un equipo ya tiene un owner asignado.
func (d *teamUserDao) HasOwnerByTeam(ctx *gin.Context, teamID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.TeamUser{}).Where("team_id = ? AND role_in_team = 'entrenador' AND deleted_at IS NULL", teamID).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking team owner: %w", err)
	}
	return count > 0, nil
}

// SoftDelete marca una asociación usuario-equipo como eliminada lógicamente.
func (d *teamUserDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.TeamUser{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}
