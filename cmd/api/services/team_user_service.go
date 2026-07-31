package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teamuser"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// TeamUserServiceInterface define las operaciones de negocio para la asociación usuario-equipo.
type TeamUserServiceInterface interface {
	AddUser(ctx *gin.Context, teamID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error)
	RemoveUser(ctx *gin.Context, teamID, userID int64) error
	GetUsersByTeam(ctx *gin.Context, teamID int64) ([]teamuser.TeamUserResponse, error)
}

type teamUserService struct {
	teamUserDao  daos.TeamUserDaoInterface
	teamDao      daos.TeamDaoInterface
	userDao      daos.UserDaoInterface
	groupDao     daos.GroupDaoInterface
	groupUserDao daos.GroupUserDaoInterface
}

// NewTeamUserService crea una nueva instancia de TeamUserService.
func NewTeamUserService(
	teamUserDao daos.TeamUserDaoInterface,
	teamDao daos.TeamDaoInterface,
	userDao daos.UserDaoInterface,
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
) TeamUserServiceInterface {
	return &teamUserService{
		teamUserDao:  teamUserDao,
		teamDao:      teamDao,
		userDao:      userDao,
		groupDao:     groupDao,
		groupUserDao: groupUserDao,
	}
}

// AddUser agrega un usuario a un equipo con el rol especificado.
// Valida que el equipo exista, que el usuario exista, que no esté ya asociado
// y que no se exceda la cantidad máxima de miembros.
func (s *teamUserService) AddUser(ctx *gin.Context, teamID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for user addition", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al equipo")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	user, err := s.userDao.FindByID(ctx, req.UserID)
	if err != nil {
		customlogger.Error(ctx, "error finding user for team addition", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al equipo")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	if !constants.IsValidAddableTeamUserRole(req.RoleInTeam) {
		return nil, fmt.Errorf("rol inválido, solo se permite 'corredor'")
	}

	existing, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, req.UserID)
	if err != nil {
		customlogger.Error(ctx, "error checking existing team user", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al equipo")
	}
	if existing != nil {
		return nil, fmt.Errorf("el usuario ya pertenece a este equipo")
	}

	count, err := s.teamUserDao.CountActiveByTeam(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error counting team members", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al equipo")
	}
	if count >= teamDB.MaxMembers {
		return nil, fmt.Errorf("el equipo ha alcanzado el máximo de %d miembros", teamDB.MaxMembers)
	}

	teamUser := &dbs.TeamUser{
		TeamID:         teamID,
		UserID:         req.UserID,
		RoleInTeam:     req.RoleInTeam,
		Status:         "active",
		AssignmentDate: time.Now(),
	}

	if err := s.teamUserDao.Create(ctx, teamUser); err != nil {
		customlogger.Error(ctx, "error adding user to team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
			customlogger.TagMethod("AddUser"))
		return nil, fmt.Errorf("error al agregar usuario al equipo")
	}

	customlogger.Info(ctx, "user added to team successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
		customlogger.Tag("user_id", fmt.Sprintf("%d", req.UserID)),
		customlogger.Tag("role", req.RoleInTeam),
		customlogger.TagMethod("AddUser"))

	// La membresía de equipo siempre implica pertenecer a un grupo. El alta directa
	// (sin pasar por invitación) no permite elegir un grupo específico, así que va al
	// principal del equipo. No bloqueante: mismo criterio que assignInviteeToGroup en
	// invitation_service.go (si falla o el equipo no tiene grupo principal, el alta al
	// equipo igual se completa).
	s.assignToMainGroup(ctx, teamID, req.UserID)

	return &teamuser.TeamUserResponse{
		ID:             teamUser.ID,
		TeamID:         teamUser.TeamID,
		UserID:         teamUser.UserID,
		RoleInTeam:     teamUser.RoleInTeam,
		Status:         teamUser.Status,
		AssignmentDate: teamUser.AssignmentDate,
	}, nil
}

// assignToMainGroup da de alta al usuario en el grupo principal del equipo. No bloquea
// AddUser si falla o si el equipo no tiene grupo principal (equipos creados con
// create_default_group: false, o de antes de que el grupo default fuera automático).
func (s *teamUserService) assignToMainGroup(ctx *gin.Context, teamID, userID int64) {
	groups, err := s.groupDao.GetByTeamID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team groups for main group assignment", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return
	}

	var mainGroupID *int64
	for _, g := range groups {
		if g.IsMain {
			id := g.ID
			mainGroupID = &id
			break
		}
	}
	if mainGroupID == nil {
		customlogger.Warn(ctx, "no main group found for team on user addition",
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return
	}

	existingGroupMember, err := s.groupUserDao.FindByGroupAndUser(ctx, *mainGroupID, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking main group membership on user addition", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("AddUser"))
		return
	}
	if existingGroupMember != nil {
		return
	}

	groupUser := &dbs.GroupUser{
		GroupID:   *mainGroupID,
		UserID:    userID,
		DateStart: time.Now(),
	}
	if err := s.groupUserDao.Create(ctx, groupUser); err != nil {
		customlogger.Error(ctx, "error creating group_user on user addition", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("group_id", fmt.Sprintf("%d", *mainGroupID)),
			customlogger.TagMethod("AddUser"))
	}
}

// RemoveUser quita un usuario de un equipo (soft-delete de la asociación).
func (s *teamUserService) RemoveUser(ctx *gin.Context, teamID, userID int64) error {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for user removal", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("RemoveUser"))
		return fmt.Errorf("error al quitar usuario del equipo")
	}
	if teamDB == nil {
		return fmt.Errorf("equipo no encontrado")
	}

	existing, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding team user for removal", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("RemoveUser"))
		return fmt.Errorf("error al quitar usuario del equipo")
	}
	if existing == nil {
		return fmt.Errorf("el usuario no pertenece a este equipo")
	}

	if existing.RoleInTeam == "entrenador" {
		return fmt.Errorf("no se puede quitar al entrenador del equipo")
	}

	if err := s.teamUserDao.SoftDelete(ctx, existing.ID); err != nil {
		customlogger.Error(ctx, "error removing user from team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("RemoveUser"))
		return fmt.Errorf("error al quitar usuario del equipo")
	}

	customlogger.Info(ctx, "user removed from team successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.TagMethod("RemoveUser"))

	return nil
}

// GetUsersByTeam retorna todos los miembros activos de un equipo.
func (s *teamUserService) GetUsersByTeam(ctx *gin.Context, teamID int64) ([]teamuser.TeamUserResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for listing users", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetUsersByTeam"))
		return nil, fmt.Errorf("error al obtener usuarios del equipo")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	teamUsers, err := s.teamUserDao.FindByTeamID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error listing team users", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetUsersByTeam"))
		return nil, fmt.Errorf("error al obtener usuarios del equipo")
	}

	responses := make([]teamuser.TeamUserResponse, len(teamUsers))
	for i, tu := range teamUsers {
		responses[i] = teamuser.TeamUserResponse{
			ID:             tu.ID,
			TeamID:         tu.TeamID,
			UserID:         tu.UserID,
			RoleInTeam:     tu.RoleInTeam,
			Status:         tu.Status,
			AssignmentDate: tu.AssignmentDate,
		}
	}

	return responses, nil
}
