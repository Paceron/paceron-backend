package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/group"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// GroupServiceInterface define las operaciones de negocio para grupos.
type GroupServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req *group.CreateGroupRequest) (*group.GroupResponse, error)
	Update(ctx *gin.Context, id int64, callerID int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error)
	Delete(ctx *gin.Context, id int64, userID int64) error
	GetByID(ctx *gin.Context, id int64) (*group.GroupResponse, error)
	GetAll(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error)
}

type groupService struct {
	groupDao    daos.GroupDaoInterface
	teamDao     daos.TeamDaoInterface
	teamUserDao daos.TeamUserDaoInterface
}

// NewGroupService crea una nueva instancia de GroupService.
func NewGroupService(
	groupDao daos.GroupDaoInterface,
	teamDao daos.TeamDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
) GroupServiceInterface {
	return &groupService{
		groupDao:    groupDao,
		teamDao:     teamDao,
		teamUserDao: teamUserDao,
	}
}

// isEntrenadorOfTeam valida que userID sea miembro con rol "entrenador" del equipo.
func (s *groupService) isEntrenadorOfTeam(ctx *gin.Context, teamID, userID int64) (bool, error) {
	member, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, userID)
	if err != nil {
		return false, err
	}
	return member != nil && member.RoleInTeam == "entrenador", nil
}

// Create crea un nuevo grupo dentro de un equipo existente. Solo el entrenador del equipo puede hacerlo.
func (s *groupService) Create(ctx *gin.Context, callerID int64, req *group.CreateGroupRequest) (*group.GroupResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, req.TeamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for group creation", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", req.TeamID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear grupo")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("el equipo no existe")
	}

	isEntrenador, err := s.isEntrenadorOfTeam(ctx, req.TeamID, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for group creation", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", req.TeamID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear grupo")
	}
	if !isEntrenador {
		return nil, fmt.Errorf("solo el entrenador del equipo puede crear grupos")
	}

	groupDB := &dbs.Group{
		Name:        req.Name,
		Description: req.Description,
		TeamID:      req.TeamID,
		IsMain:      req.IsMain,
	}

	if err := s.groupDao.Create(ctx, groupDB); err != nil {
		customlogger.Error(ctx, "error creating group", err,
			customlogger.Tag("name", req.Name),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear grupo")
	}

	customlogger.Info(ctx, "group created successfully",
		customlogger.Tag("group_id", fmt.Sprintf("%d", groupDB.ID)),
		customlogger.Tag("name", groupDB.Name),
		customlogger.TagMethod("Create"))

	return s.toResponse(groupDB), nil
}

// Update actualiza los campos de un grupo existente. Solo el entrenador del equipo puede hacerlo.
func (s *groupService) Update(ctx *gin.Context, id int64, callerID int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error) {
	groupDB, err := s.groupDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding group for update", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar grupo")
	}
	if groupDB == nil {
		return nil, fmt.Errorf("grupo no encontrado")
	}

	isEntrenador, err := s.isEntrenadorOfTeam(ctx, groupDB.TeamID, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for group update", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar grupo")
	}
	if !isEntrenador {
		return nil, fmt.Errorf("solo el entrenador puede actualizar el grupo")
	}

	if req.Name != nil {
		groupDB.Name = *req.Name
	}
	if req.Description != nil {
		groupDB.Description = *req.Description
	}
	if req.IsMain != nil {
		groupDB.IsMain = *req.IsMain
	}

	if err := s.groupDao.Update(ctx, groupDB); err != nil {
		customlogger.Error(ctx, "error updating group", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar grupo")
	}

	customlogger.Info(ctx, "group updated successfully",
		customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Update"))

	return s.toResponse(groupDB), nil
}

// Delete elimina lógicamente un grupo. Solo el entrenador del equipo puede hacerlo.
func (s *groupService) Delete(ctx *gin.Context, id int64, userID int64) error {
	groupDB, err := s.groupDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding group for delete", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar grupo")
	}
	if groupDB == nil {
		return fmt.Errorf("grupo no encontrado")
	}

	member, err := s.teamUserDao.FindByTeamAndUser(ctx, groupDB.TeamID, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership for group delete", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar grupo")
	}
	if member == nil {
		return fmt.Errorf("el usuario no pertenece al equipo de este grupo")
	}
	if member.RoleInTeam != "entrenador" {
		return fmt.Errorf("solo el entrenador puede eliminar el grupo")
	}

	if err := s.groupDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting group", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar grupo")
	}

	customlogger.Info(ctx, "group deleted successfully",
		customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Delete"))

	return nil
}

// GetByID obtiene un grupo por su ID.
func (s *groupService) GetByID(ctx *gin.Context, id int64) (*group.GroupResponse, error) {
	groupDB, err := s.groupDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding group", err,
			customlogger.Tag("group_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("GetByID"))
		return nil, fmt.Errorf("error al obtener grupo")
	}
	if groupDB == nil {
		return nil, fmt.Errorf("grupo no encontrado")
	}

	return s.toResponse(groupDB), nil
}

// GetAll obtiene todos los grupos activos.
func (s *groupService) GetAll(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
	if teamID != nil {
		team, err := s.teamDao.FindByID(ctx, *teamID)
		if err != nil {
			customlogger.Error(ctx, "error finding team for groups", err,
				customlogger.Tag("team_id", fmt.Sprintf("%d", *teamID)),
				customlogger.TagMethod("GetAll"))
			return nil, fmt.Errorf("error al obtener grupos")
		}
		if team == nil {
			return nil, fmt.Errorf("equipo no encontrado")
		}

		if userID != nil {
			member, err := s.teamUserDao.FindByTeamAndUser(ctx, *teamID, *userID)
			if err != nil {
				customlogger.Error(ctx, "error checking team membership", err,
					customlogger.Tag("team_id", fmt.Sprintf("%d", *teamID)),
					customlogger.Tag("user_id", fmt.Sprintf("%d", *userID)),
					customlogger.TagMethod("GetAll"))
				return nil, fmt.Errorf("error al obtener grupos")
			}
			if member == nil {
				return nil, fmt.Errorf("el usuario no pertenece a este equipo")
			}
		}
	}

	var groups []dbs.Group
	var err error

	if teamID != nil {
		groups, err = s.groupDao.GetByTeamID(ctx, *teamID)
	} else {
		groups, err = s.groupDao.GetAll(ctx)
	}
	if err != nil {
		customlogger.Error(ctx, "error getting groups", err,
			customlogger.TagMethod("GetAll"))
		return nil, fmt.Errorf("error al obtener grupos")
	}

	responses := make([]group.GroupResponse, len(groups))
	for i, g := range groups {
		responses[i] = *s.toResponse(&g)
	}

	return responses, nil
}

// toResponse convierte un modelo DB a un DTO de respuesta.
func (s *groupService) toResponse(g *dbs.Group) *group.GroupResponse {
	return &group.GroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		TeamID:      g.TeamID,
		IsMain:      g.IsMain,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}
