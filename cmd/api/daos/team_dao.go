package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// TeamDaoInterface define las operaciones de acceso a datos para equipos.
type TeamDaoInterface interface {
	Create(ctx *gin.Context, team *dbs.Team) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Team, error)
	GetAll(ctx *gin.Context) ([]dbs.Team, error)
	GetAllByOwnerID(ctx *gin.Context, ownerID int64) ([]dbs.Team, error)
	GetAllByMemberID(ctx *gin.Context, memberID int64) ([]dbs.Team, error)
	Update(ctx *gin.Context, team *dbs.Team) error
	SoftDelete(ctx *gin.Context, id int64) error
	UpdateIcon(ctx *gin.Context, teamID int64, key string, updatedAt time.Time) error
	ClearIcon(ctx *gin.Context, teamID int64) error
	SearchPublic(ctx *gin.Context, filters TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error)
}

// TeamSearchFilters agrupa los filtros opcionales de búsqueda de equipos.
type TeamSearchFilters struct {
	Name     string
	Level    string
	Country  string
	Province string
	City     string
}

type teamDao struct {
	DB *gorm.DB
}

// NewTeamDao crea una nueva instancia de TeamDao.
func NewTeamDao(database *gorm.DB) TeamDaoInterface {
	return &teamDao{
		DB: database,
	}
}

// Create inserta un nuevo equipo en la base de datos.
func (d *teamDao) Create(ctx *gin.Context, team *dbs.Team) error {
	return d.DB.Create(team).Error
}

// FindByID busca un equipo por su ID, excluyendo los eliminados lógicamente.
func (d *teamDao) FindByID(ctx *gin.Context, id int64) (*dbs.Team, error) {
	var team dbs.Team
	err := d.DB.Where("id = ? AND deleted_at IS NULL", id).First(&team).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding team: %w", err)
	}
	return &team, nil
}

// GetAll devuelve todos los equipos activos.
func (d *teamDao) GetAll(ctx *gin.Context) ([]dbs.Team, error) {
	var teams []dbs.Team
	err := d.DB.Where("deleted_at IS NULL").Find(&teams).Error
	if err != nil {
		return nil, fmt.Errorf("error finding teams: %w", err)
	}
	return teams, nil
}

// GetAllByOwnerID devuelve todos los equipos activos administrados por un owner.
func (d *teamDao) GetAllByOwnerID(ctx *gin.Context, ownerID int64) ([]dbs.Team, error) {
	var teams []dbs.Team
	err := d.DB.Where("owner_id = ? AND deleted_at IS NULL", ownerID).Find(&teams).Error
	if err != nil {
		return nil, fmt.Errorf("error finding teams by owner: %w", err)
	}
	return teams, nil
}

// GetAllByMemberID devuelve todos los equipos activos donde el usuario es miembro
// (vía team_users), sin importar el rol.
func (d *teamDao) GetAllByMemberID(ctx *gin.Context, memberID int64) ([]dbs.Team, error) {
	var teams []dbs.Team
	err := d.DB.
		Joins("JOIN team_users ON team_users.team_id = teams.id").
		Where("team_users.user_id = ? AND team_users.deleted_at IS NULL AND teams.deleted_at IS NULL", memberID).
		Find(&teams).Error
	if err != nil {
		return nil, fmt.Errorf("error finding teams by member: %w", err)
	}
	return teams, nil
}

// Update actualiza los campos de un equipo existente.
func (d *teamDao) Update(ctx *gin.Context, team *dbs.Team) error {
	return d.DB.Save(team).Error
}

// SoftDelete marca un equipo como eliminado lógicamente.
func (d *teamDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Team{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}

func (d *teamDao) UpdateIcon(ctx *gin.Context, teamID int64, key string, updatedAt time.Time) error {
	err := d.DB.Model(&dbs.Team{}).Where("id = ?", teamID).Updates(map[string]interface{}{
		"icon_key":        key,
		"icon_updated_at": updatedAt,
	}).Error
	if err != nil {
		return fmt.Errorf("error updating team icon: %w", err)
	}
	return nil
}

func (d *teamDao) ClearIcon(ctx *gin.Context, teamID int64) error {
	err := d.DB.Model(&dbs.Team{}).Where("id = ?", teamID).Updates(map[string]interface{}{
		"icon_key":        nil,
		"icon_updated_at": nil,
	}).Error
	if err != nil {
		return fmt.Errorf("error clearing team icon: %w", err)
	}
	return nil
}

// SearchPublic busca equipos visible=true, excluyendo aquellos donde callerID
// es el owner o ya es miembro activo (independiente uno del otro: el team_user
// del owner se crea best-effort en team_service.Create y puede faltar). Pide
// pageSize+1 filas para derivar hasMore sin un COUNT(*) adicional.
func (d *teamDao) SearchPublic(ctx *gin.Context, filters TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error) {
	query := d.DB.Model(&dbs.Team{}).
		Where("teams.visible = true AND teams.deleted_at IS NULL").
		Where("teams.owner_id != ?", callerID).
		Where("teams.id NOT IN (?)", d.DB.Model(&dbs.TeamUser{}).Select("team_id").Where("user_id = ? AND deleted_at IS NULL", callerID))

	if filters.Name != "" {
		query = query.Where("teams.name ILIKE ?", "%"+filters.Name+"%")
	}
	if filters.Level != "" {
		query = query.Where("teams.level = ?", filters.Level)
	}
	if filters.Country != "" {
		query = query.Where("teams.country = ?", filters.Country)
	}
	if filters.Province != "" {
		query = query.Where("teams.province = ?", filters.Province)
	}
	if filters.City != "" {
		query = query.Where("teams.city = ?", filters.City)
	}

	var teams []dbs.Team
	offset := (page - 1) * pageSize
	err := query.Order("teams.id").Offset(offset).Limit(pageSize + 1).Find(&teams).Error
	if err != nil {
		return nil, false, fmt.Errorf("error searching teams: %w", err)
	}

	hasMore := len(teams) > pageSize
	if hasMore {
		teams = teams[:pageSize]
	}

	return teams, hasMore, nil
}
