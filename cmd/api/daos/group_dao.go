package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// GroupWithTeamName resuelve en una sola consulta el grupo y el nombre de su
// equipo (calendarios agregados). TeamName queda vacío si el equipo no existe
// o está soft-deleted, igual que el batch separado anterior.
type GroupWithTeamName struct {
	ID       int64  `gorm:"column:id"`
	Name     string `gorm:"column:name"`
	TeamID   int64  `gorm:"column:team_id"`
	TeamName string `gorm:"column:team_name"`
}

// GroupDaoInterface define las operaciones de acceso a datos para grupos.
type GroupDaoInterface interface {
	Create(ctx *gin.Context, group *dbs.Group) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Group, error)
	FindByIDAndTeamID(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error)
	GetAll(ctx *gin.Context) ([]dbs.Group, error)
	GetByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Group, error)
	// FindByOwnerID devuelve todos los grupos activos de todos los equipos
	// administrados por ownerID (join a teams; grupos soft-deleted fuera).
	FindByOwnerID(ctx *gin.Context, ownerID int64) ([]dbs.Group, error)
	// FindByOwnerIDWithTeamName es la variante de FindByOwnerID que resuelve
	// también el nombre del equipo en la misma consulta.
	FindByOwnerIDWithTeamName(ctx *gin.Context, ownerID int64) ([]GroupWithTeamName, error)
	// FindByIDs devuelve varios grupos activos en una sola query — batch de
	// nombres para los banners del home (design.md D6/D8, evita N+1).
	FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Group, error)
	// FindNamesByIDs resuelve solo id/name de grupos activos para batches de
	// nombres que no necesitan el resto del grupo.
	FindNamesByIDs(ctx *gin.Context, ids []int64) ([]dbs.IDName, error)
	// FindWithTeamNameByIDs devuelve varios grupos activos con el nombre de su
	// equipo en una sola query.
	FindWithTeamNameByIDs(ctx *gin.Context, ids []int64) ([]GroupWithTeamName, error)
	Update(ctx *gin.Context, group *dbs.Group) error
	SoftDelete(ctx *gin.Context, id int64) error
	SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error
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

// FindByOwnerID devuelve todos los grupos activos de todos los equipos
// administrados por ownerID (reemplaza la composición GetAllByOwnerID +
// GetByTeamID por equipo con una única query con join).
func (d *groupDao) FindByOwnerID(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
	var groups []dbs.Group
	err := d.DB.
		Joins("JOIN teams ON teams.id = groups.team_id AND teams.deleted_at IS NULL").
		Where("teams.owner_id = ? AND groups.deleted_at IS NULL", ownerID).
		Order("groups.id").
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups by owner: %w", err)
	}
	return groups, nil
}

// FindByIDs devuelve varios grupos activos en una sola consulta.
func (d *groupDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var groups []dbs.Group
	err := d.DB.Where("id IN ? AND deleted_at IS NULL", ids).Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups by ids: %w", err)
	}
	return groups, nil
}

// FindNamesByIDs devuelve id/name de los grupos activos indicados.
func (d *groupDao) FindNamesByIDs(ctx *gin.Context, ids []int64) ([]dbs.IDName, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var groups []dbs.IDName
	err := d.DB.Model(&dbs.Group{}).
		Select("id, name").
		Where("id IN ? AND deleted_at IS NULL", ids).
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("error finding group names by ids: %w", err)
	}
	return groups, nil
}

// FindByOwnerIDWithTeamName resuelve en una consulta los grupos activos de los
// equipos activos del owner y el nombre de cada equipo.
func (d *groupDao) FindByOwnerIDWithTeamName(ctx *gin.Context, ownerID int64) ([]GroupWithTeamName, error) {
	var rows []GroupWithTeamName
	err := d.DB.Model(&dbs.Group{}).
		Select("groups.id, groups.name, groups.team_id, teams.name AS team_name").
		Joins("JOIN teams ON teams.id = groups.team_id AND teams.deleted_at IS NULL").
		Where("teams.owner_id = ? AND groups.deleted_at IS NULL", ownerID).
		Order("groups.id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups by owner with team: %w", err)
	}
	return rows, nil
}

// FindWithTeamNameByIDs resuelve en una consulta varios grupos activos con el
// nombre de su equipo. Usa LEFT JOIN para conservar los grupos cuyo equipo ya
// no está activo (mismo resultado que el batch de teams separado, que los
// dejaba sin nombre).
func (d *groupDao) FindWithTeamNameByIDs(ctx *gin.Context, ids []int64) ([]GroupWithTeamName, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []GroupWithTeamName
	err := d.DB.Model(&dbs.Group{}).
		Select("groups.id, groups.name, groups.team_id, teams.name AS team_name").
		Joins("LEFT JOIN teams ON teams.id = groups.team_id AND teams.deleted_at IS NULL").
		Where("groups.id IN ? AND groups.deleted_at IS NULL", ids).
		Order("groups.id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding groups with team by ids: %w", err)
	}
	return rows, nil
}

// Update actualiza los campos de un grupo existente.
func (d *groupDao) Update(ctx *gin.Context, group *dbs.Group) error {
	return d.DB.Save(group).Error
}

// SoftDelete marca un grupo como eliminado lógicamente.
func (d *groupDao) SoftDelete(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.Group{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()")).Error
}

// SoftDeleteByTeamID marca como eliminados lógicamente todos los grupos activos
// de un equipo (usado en cascada al eliminar el equipo).
func (d *groupDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	return d.DB.Model(&dbs.Group{}).
		Where("team_id = ? AND deleted_at IS NULL", teamID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}
