package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// TeamMembershipDAOInterface expone los chequeos de pertenencia/ownership de
// equipos que comparten varios módulos (attendances, workout_feedback). Encapsulan
// la membresía real del sistema: owner = teams.owner_id (sin soft-delete),
// pertenencia = team_users activo (deleted_at IS NULL).
type TeamMembershipDAOInterface interface {
	// TeamExists indica si existe un team activo (sin soft-delete) con ese id.
	TeamExists(ctx *gin.Context, teamID int64) (bool, error)
	// IsTeamOwner indica si userID es el owner del team (teams.owner_id).
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	// ExistsUserInTeamOwnedBy indica si targetUserID pertenece a al menos un team
	// cuyo owner sea ownerUserID.
	ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
}

type teamMembershipDao struct {
	DB *gorm.DB
}

// NewTeamMembershipDao crea una nueva instancia de TeamMembershipDao.
func NewTeamMembershipDao(database *gorm.DB) TeamMembershipDAOInterface {
	return &teamMembershipDao{
		DB: database,
	}
}

// TeamExists indica si existe un team activo (sin soft-delete) con ese id.
func (d *teamMembershipDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Team{}).
		Where("id = ? AND deleted_at IS NULL", teamID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking team exists: %w", err)
	}
	return count > 0, nil
}

// IsTeamOwner indica si userID es el owner del team (teams.owner_id).
func (d *teamMembershipDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Team{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking team owner: %w", err)
	}
	return count > 0, nil
}

// ExistsUserInTeamOwnedBy indica si targetUserID pertenece (team_users activo) a
// al menos un team cuyo owner sea ownerUserID. Es la relación que autoriza el
// escenario "entrenador/owner sobre un atleta específico": un owner puede operar
// sobre un atleta solo si ese atleta está en un equipo que él administra.
func (d *teamMembershipDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.TeamUser{}).
		Joins("JOIN teams ON teams.id = team_users.team_id AND teams.deleted_at IS NULL AND teams.owner_id = ?", ownerUserID).
		Where("team_users.user_id = ? AND team_users.deleted_at IS NULL", targetUserID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking user in team owned by: %w", err)
	}
	return count > 0, nil
}