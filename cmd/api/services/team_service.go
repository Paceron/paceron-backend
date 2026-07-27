package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// teamOwnerRoleName es el rol que debe tener un usuario para poder ser owner de un equipo.
const teamOwnerRoleName = "entrenador"

// TeamServiceInterface define las operaciones de negocio para equipos.
type TeamServiceInterface interface {
	Create(ctx *gin.Context, req *team.CreateTeamRequest) (*team.TeamResponse, error)
	Update(ctx *gin.Context, id int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error)
	Delete(ctx *gin.Context, id int64, userID int64) error
	GetByID(ctx *gin.Context, id int64) (*team.TeamResponse, error)
	GetAll(ctx *gin.Context) ([]team.TeamResponse, error)
	UpdateAddress(ctx *gin.Context, id int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error)
}

type teamService struct {
	teamDao     daos.TeamDaoInterface
	userDao     daos.UserDaoInterface
	userRoleDao daos.UserRoleDaoInterface
	roleDao     daos.RoleDaoInterface
	teamUserDao daos.TeamUserDaoInterface
}

// NewTeamService crea una nueva instancia de TeamService.
func NewTeamService(
	teamDao daos.TeamDaoInterface,
	userDao daos.UserDaoInterface,
	userRoleDao daos.UserRoleDaoInterface,
	roleDao daos.RoleDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
) TeamServiceInterface {
	return &teamService{
		teamDao:     teamDao,
		userDao:     userDao,
		userRoleDao: userRoleDao,
		roleDao:     roleDao,
		teamUserDao: teamUserDao,
	}
}

// Create crea un nuevo equipo. El owner debe tener el rol "entrenador".
func (s *teamService) Create(ctx *gin.Context, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
	user, err := s.userDao.FindByID(ctx, req.OwnerID)
	if err != nil {
		customlogger.Error(ctx, "error finding owner user", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", req.OwnerID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear equipo")
	}
	if user == nil {
		return nil, fmt.Errorf("el usuario owner no existe")
	}

	userRoles, err := s.userRoleDao.FindByUserID(ctx, req.OwnerID)
	if err != nil {
		customlogger.Error(ctx, "error finding owner roles", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", req.OwnerID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear equipo")
	}

	hasEntrenadorRole := false
	for _, ur := range userRoles {
		role, err := s.roleDao.FindByID(ctx, ur.RoleID)
		if err != nil {
			continue
		}
		if role != nil && role.Name == teamOwnerRoleName {
			hasEntrenadorRole = true
			break
		}
	}
	if !hasEntrenadorRole {
		return nil, fmt.Errorf("el owner debe tener el rol '%s'", teamOwnerRoleName)
	}

	teamDB := &dbs.Team{
		Name:         req.Name,
		Description:  req.Description,
		Level:        req.Level,
		MaxMembers:   req.MaxMembers,
		Requirements: req.Requirements,
		OwnerID:      req.OwnerID,
		Status:       "active",
	}

	if err := s.teamDao.Create(ctx, teamDB); err != nil {
		customlogger.Error(ctx, "error creating team", err,
			customlogger.Tag("name", req.Name),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear equipo")
	}

	customlogger.Info(ctx, "team created successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", teamDB.ID)),
		customlogger.Tag("name", teamDB.Name),
		customlogger.TagMethod("Create"))

	return s.toResponse(teamDB), nil
}

// Update actualiza los campos de un equipo existente.
func (s *teamService) Update(ctx *gin.Context, id int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding team for update", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar equipo")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	if req.Name != nil {
		teamDB.Name = *req.Name
	}
	if req.Description != nil {
		teamDB.Description = *req.Description
	}
	if req.Level != nil {
		teamDB.Level = *req.Level
	}
	if req.MaxMembers != nil {
		teamDB.MaxMembers = *req.MaxMembers
	}
	if req.Requirements != nil {
		teamDB.Requirements = *req.Requirements
	}

	if err := s.teamDao.Update(ctx, teamDB); err != nil {
		customlogger.Error(ctx, "error updating team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar equipo")
	}

	customlogger.Info(ctx, "team updated successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Update"))

	return s.toResponse(teamDB), nil
}

// Delete elimina lógicamente un equipo. Solo el entrenador puede hacerlo y el equipo no debe tener miembros.
func (s *teamService) Delete(ctx *gin.Context, id int64, userID int64) error {
	teamDB, err := s.teamDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding team for delete", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar equipo")
	}
	if teamDB == nil {
		return fmt.Errorf("equipo no encontrado")
	}

	member, err := s.teamUserDao.FindByTeamAndUser(ctx, id, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership for delete", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar equipo")
	}
	if member == nil {
		return fmt.Errorf("el usuario no pertenece a este equipo")
	}
	if member.RoleInTeam != "entrenador" {
		return fmt.Errorf("solo el entrenador puede eliminar el equipo")
	}

	count, err := s.teamUserDao.CountActiveByTeam(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error counting team members for delete", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar equipo")
	}
	if count > 0 {
		return fmt.Errorf("no se puede eliminar un equipo con miembros activos")
	}

	if err := s.teamDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al eliminar equipo")
	}

	customlogger.Info(ctx, "team deleted successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Delete"))

	return nil
}

// GetByID obtiene un equipo por su ID.
func (s *teamService) GetByID(ctx *gin.Context, id int64) (*team.TeamResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("GetByID"))
		return nil, fmt.Errorf("error al obtener equipo")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	return s.toResponse(teamDB), nil
}

// GetAll obtiene todos los equipos activos.
func (s *teamService) GetAll(ctx *gin.Context) ([]team.TeamResponse, error) {
	teams, err := s.teamDao.GetAll(ctx)
	if err != nil {
		customlogger.Error(ctx, "error getting all teams", err,
			customlogger.TagMethod("GetAll"))
		return nil, fmt.Errorf("error al obtener equipos")
	}

	responses := make([]team.TeamResponse, len(teams))
	for i, t := range teams {
		responses[i] = *s.toResponse(&t)
	}

	return responses, nil
}

// UpdateAddress actualiza la dirección de un equipo.
func (s *teamService) UpdateAddress(ctx *gin.Context, id int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding team for address update", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("UpdateAddress"))
		return nil, fmt.Errorf("error al actualizar dirección")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	teamDB.Country = req.Country
	teamDB.Province = req.Province
	teamDB.City = req.City
	teamDB.Street = req.Street
	teamDB.Number = req.Number

	if err := s.teamDao.Update(ctx, teamDB); err != nil {
		customlogger.Error(ctx, "error updating team address", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("UpdateAddress"))
		return nil, fmt.Errorf("error al actualizar dirección")
	}

	customlogger.Info(ctx, "team address updated successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("UpdateAddress"))

	return s.toResponse(teamDB), nil
}

// toResponse convierte un modelo DB a un DTO de respuesta.
func (s *teamService) toResponse(t *dbs.Team) *team.TeamResponse {
	return &team.TeamResponse{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Level:        t.Level,
		MaxMembers:   t.MaxMembers,
		Requirements: t.Requirements,
		OwnerID:      t.OwnerID,
		Status:       t.Status,
		Country:      t.Country,
		Province:     t.Province,
		City:         t.City,
		Street:       t.Street,
		Number:       t.Number,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}
