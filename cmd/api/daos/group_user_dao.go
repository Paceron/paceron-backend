package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// GroupMembershipGroup es una membresía activa con el nombre del grupo
// resuelto en la misma consulta (resumen de calendario).
type GroupMembershipGroup struct {
	GroupID   int64  `gorm:"column:group_id"`
	GroupName string `gorm:"column:group_name"`
}

// GroupUserDaoInterface define las operaciones de acceso a datos para la asociación usuario-grupo.
type GroupUserDaoInterface interface {
	Create(ctx *gin.Context, groupUser *dbs.GroupUser) error
	FindByGroupAndUser(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error)
	FindByGroupID(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error)
	FindByGroupIDs(ctx *gin.Context, groupIDs []int64) ([]dbs.GroupUser, error)
	FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error)
	// FindActiveWithGroupByUserID resuelve en una sola consulta las membresías
	// activas del usuario y el nombre de cada grupo activo (JOIN), en orden de
	// membresía. Los grupos soft-deleted se omiten.
	FindActiveWithGroupByUserID(ctx *gin.Context, userID int64) ([]GroupMembershipGroup, error)
	SoftDelete(ctx *gin.Context, id int64) error
	SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error
	IsActiveGroupMember(ctx *gin.Context, groupID, userID int64, sessionDate time.Time) (bool, error)
	MissingGroupMembers(ctx *gin.Context, groupID int64, userIDs []int64, sessionDate time.Time) ([]int64, error)
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

// FindByGroupIDs es la variante batch de FindByGroupID: resuelve en una sola
// query las membresías activas de varios grupos.
func (d *groupUserDao) FindByGroupIDs(ctx *gin.Context, groupIDs []int64) ([]dbs.GroupUser, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var groupUsers []dbs.GroupUser
	err := d.DB.Where("group_id IN ? AND deleted_at IS NULL", groupIDs).Find(&groupUsers).Error
	if err != nil {
		return nil, fmt.Errorf("error finding group users by group ids: %w", err)
	}
	return groupUsers, nil
}

// FindByUserID devuelve todas las asociaciones activas de un usuario: sin
// eliminado lógico y con date_end nulo o todavía no vencido (design.md D6 —
// los banners del home solo consideran membresías activas).
func (d *groupUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	var groupUsers []dbs.GroupUser
	err := d.DB.Where("user_id = ? AND deleted_at IS NULL AND (date_end IS NULL OR date_end > NOW())", userID).Find(&groupUsers).Error
	if err != nil {
		return nil, fmt.Errorf("error finding user groups: %w", err)
	}
	return groupUsers, nil
}

// FindActiveWithGroupByUserID devuelve las membresías activas del usuario con el
// nombre de su grupo en una sola consulta, preservando el orden de inserción de
// la membresía. Grupos soft-deleted quedan fuera.
func (d *groupUserDao) FindActiveWithGroupByUserID(ctx *gin.Context, userID int64) ([]GroupMembershipGroup, error) {
	var rows []GroupMembershipGroup
	err := d.DB.Model(&dbs.GroupUser{}).
		Select("groups.id AS group_id, groups.name AS group_name").
		Joins("JOIN groups ON groups.id = group_users.group_id AND groups.deleted_at IS NULL").
		Where("group_users.user_id = ? AND group_users.deleted_at IS NULL AND (group_users.date_end IS NULL OR group_users.date_end > NOW())", userID).
		Order("group_users.id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding user groups with group: %w", err)
	}
	return rows, nil
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

// activeGroupMemberWhere es el criterio de "miembro activo" de la asistencia
// (design.md D7). Evalúa la ventana de membresía contra la fecha de la SESIÓN, no
// contra NOW(): la sesión que se está revisando ya ocurrió, así que la pregunta
// correcta es "quién era miembro cuando la sesión pasó".
//
// La ventana se compara POR FECHA (cast ::date en ambos lados): date_start con
// hora del mismo día de la sesión cuenta como dentro (Gap 25).
//
// Deliberadamente NO se usa el criterio de FindByGroupID/FindByGroupAndUser
// (solo deleted_at IS NULL): esos responden "¿quién integra el grupo hoy?" y
// alimentan el roster de la pantalla de equipo, que tiene otra semántica. La
// diferencia observable — un corredor que dejó el grupo DESPUÉS de la sesión
// sigue apareciendo en la grilla de esa sesión — es intencional.
func activeGroupMemberWhere(query *gorm.DB, sessionDate time.Time) *gorm.DB {
	return query.Where("deleted_at IS NULL").
		Where("date_start::date <= ?::date", sessionDate).
		Where("date_end IS NULL OR date_end::date >= ?::date", sessionDate)
}

// IsActiveGroupMember indica si userID era miembro activo de groupID en la fecha
// de la sesión, según la ventana de membresía (D7).
func (d *groupUserDao) IsActiveGroupMember(ctx *gin.Context, groupID, userID int64, sessionDate time.Time) (bool, error) {
	var count int64
	err := activeGroupMemberWhere(d.DB.Model(&dbs.GroupUser{}), sessionDate).
		Where("group_id = ?", groupID).
		Where("user_id = ?", userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking group membership: %w", err)
	}
	return count > 0, nil
}

// MissingGroupMembers devuelve, en el mismo orden que userIDs, los que NO eran
// miembros activos del grupo en la fecha de la sesión. Es lo que permite que
// POST /attendance/bulk rechace el lote entero con 422 indicando los user_id
// culpables, sin escribir nada.
//
// userIDs vacío devuelve nil sin tocar la DB: un lote vacío es un no-op.
func (d *groupUserDao) MissingGroupMembers(ctx *gin.Context, groupID int64, userIDs []int64, sessionDate time.Time) ([]int64, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var found []dbs.GroupUser
	err := activeGroupMemberWhere(d.DB.Model(&dbs.GroupUser{}), sessionDate).
		Where("group_id = ?", groupID).
		Where("user_id IN ?", userIDs).
		Find(&found).Error
	if err != nil {
		return nil, fmt.Errorf("error finding group members: %w", err)
	}

	member := make(map[int64]struct{}, len(found))
	for _, gu := range found {
		member[gu.UserID] = struct{}{}
	}
	missing := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if _, ok := member[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing, nil
}
