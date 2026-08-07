package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/groupuser"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// GroupUserServiceInterface define las operaciones de negocio para la asociación usuario-grupo.
type GroupUserServiceInterface interface {
	AddUser(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error)
	RemoveUser(ctx *gin.Context, groupID, callerID, targetUserID int64) error
	GetUsersByGroup(ctx *gin.Context, groupID int64, callerID int64) ([]groupuser.GroupUserResponse, error)
}

type groupUserService struct {
	groupUserDao daos.GroupUserDaoInterface
	groupDao     daos.GroupDaoInterface
	userDao      daos.UserDaoInterface
	teamUserDao  daos.TeamUserDaoInterface
}

// NewGroupUserService crea una nueva instancia de GroupUserService.
func NewGroupUserService(
	groupUserDao daos.GroupUserDaoInterface,
	groupDao daos.GroupDaoInterface,
	userDao daos.UserDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
) GroupUserServiceInterface {
	return &groupUserService{
		groupUserDao: groupUserDao,
		groupDao:     groupDao,
		userDao:      userDao,
		teamUserDao:  teamUserDao,
	}
}

// AddUser agrega un usuario a un grupo. Solo el entrenador del equipo puede hacerlo.
// Valida que el grupo pertenezca al equipo, que el usuario exista y que no esté ya asociado.
func (s *groupUserService) AddUser(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error) {
	groupDB, err := s.groupDao.FindByIDAndTeamID(ctx, groupID, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding group for user addition", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al grupo")
	}
	if groupDB == nil {
		return nil, fmt.Errorf("grupo no encontrado en este equipo")
	}

	caller, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for group user addition", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al grupo")
	}
	if caller == nil || caller.RoleInTeam != "entrenador" {
		return nil, fmt.Errorf("solo el entrenador puede agregar usuarios al grupo")
	}

	user, err := s.userDao.FindByID(ctx, req.UserID)
	if err != nil {
		customlogger.Error(ctx, "error finding user for group addition", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al grupo")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	existing, err := s.groupUserDao.FindByGroupAndUser(ctx, groupID, req.UserID)
	if err != nil {
		customlogger.Error(ctx, "error checking existing group user", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al grupo")
	}
	if existing != nil {
		return nil, fmt.Errorf("el usuario ya pertenece a este grupo")
	}

	dateStart := time.Now()
	if req.DateStart != nil {
		dateStart = *req.DateStart
	}

	groupUser := &dbs.GroupUser{
		GroupID:   groupID,
		UserID:    req.UserID,
		DateStart: dateStart,
		DateEnd:   req.DateEnd,
	}

	if err := s.groupUserDao.Create(ctx, groupUser); err != nil {
		customlogger.Error(ctx, "error adding user to group", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al grupo")
	}

	customlogger.Info(ctx, "user added to group successfully",
		customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
		customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
		customlogger.TagMethod("AddUser"))

	return &groupuser.GroupUserResponse{
		ID:        groupUser.ID,
		GroupID:   groupUser.GroupID,
		UserID:    groupUser.UserID,
		DateStart: groupUser.DateStart,
		DateEnd:   groupUser.DateEnd,
	}, nil
}

// RemoveUser quita un usuario de un grupo (soft-delete de la asociación). El propio
// usuario puede quitarse a sí mismo, o el entrenador del equipo del grupo puede quitar a otro.
func (s *groupUserService) RemoveUser(ctx *gin.Context, groupID, callerID, targetUserID int64) error {
	groupDB, err := s.groupDao.FindByID(ctx, groupID)
	if err != nil {
		customlogger.Error(ctx, "error finding group for user removal", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.TagMethod("RemoveUser"))
		return fmt.Errorf("error al quitar usuario del grupo")
	}
	if groupDB == nil {
		return fmt.Errorf("grupo no encontrado")
	}

	if callerID != targetUserID {
		caller, err := s.teamUserDao.FindByTeamAndUser(ctx, groupDB.TeamID, callerID)
		if err != nil {
			customlogger.Error(ctx, "error checking caller role for group user removal", err,
				customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
				customlogger.TagMethod("RemoveUser"))
			return fmt.Errorf("error al quitar usuario del grupo")
		}
		if caller == nil || caller.RoleInTeam != "entrenador" {
			return fmt.Errorf("solo el entrenador puede quitar a otro usuario del grupo")
		}
	}

	existing, err := s.groupUserDao.FindByGroupAndUser(ctx, groupID, targetUserID)
	if err != nil {
		customlogger.Error(ctx, "error finding group user for removal", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", targetUserID)),
			customlogger.TagMethod("RemoveUser"))
		return fmt.Errorf("error al quitar usuario del grupo")
	}
	if existing == nil {
		return fmt.Errorf("el usuario no pertenece a este grupo")
	}

	if err := s.groupUserDao.SoftDelete(ctx, existing.ID); err != nil {
		customlogger.Error(ctx, "error removing user from group", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", targetUserID)),
			customlogger.TagMethod("RemoveUser"))
		return fmt.Errorf("error al quitar usuario del grupo")
	}

	customlogger.Info(ctx, "user removed from group successfully",
		customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
		customlogger.Tag("user_id", fmt.Sprintf("%d", targetUserID)),
		customlogger.TagMethod("RemoveUser"))

	return nil
}

// GetUsersByGroup retorna todos los miembros activos de un grupo. Solo un miembro del
// equipo del grupo puede consultarlo (evita que cualquier logueado enumere el roster).
func (s *groupUserService) GetUsersByGroup(ctx *gin.Context, groupID int64, callerID int64) ([]groupuser.GroupUserResponse, error) {
	groupDB, err := s.groupDao.FindByID(ctx, groupID)
	if err != nil {
		customlogger.Error(ctx, "error finding group for listing users", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.TagMethod("GetUsersByGroup"))
		return nil, fmt.Errorf("error al obtener usuarios del grupo")
	}
	if groupDB == nil {
		return nil, fmt.Errorf("grupo no encontrado")
	}

	caller, err := s.teamUserDao.FindByTeamAndUser(ctx, groupDB.TeamID, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller membership for listing group users", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.TagMethod("GetUsersByGroup"))
		return nil, fmt.Errorf("error al obtener usuarios del grupo")
	}
	if caller == nil {
		return nil, fmt.Errorf("el usuario no pertenece al equipo de este grupo")
	}

	groupUsers, err := s.groupUserDao.FindByGroupID(ctx, groupID)
	if err != nil {
		customlogger.Error(ctx, "error listing group users", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", groupID)),
			customlogger.TagMethod("GetUsersByGroup"))
		return nil, fmt.Errorf("error al obtener usuarios del grupo")
	}

	responses := make([]groupuser.GroupUserResponse, len(groupUsers))
	for i, gu := range groupUsers {
		responses[i] = groupuser.GroupUserResponse{
			ID:        gu.ID,
			GroupID:   gu.GroupID,
			UserID:    gu.UserID,
			DateStart: gu.DateStart,
			DateEnd:   gu.DateEnd,
		}
	}

	return responses, nil
}
