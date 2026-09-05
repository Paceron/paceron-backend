package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/storageclient"
)

// teamOwnerRoleName es el rol que debe tener un usuario para poder ser owner de un equipo.
const teamOwnerRoleName = "entrenador"

// TeamServiceInterface define las operaciones de negocio para equipos.
type TeamServiceInterface interface {
	Create(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error)
	Update(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error)
	Delete(ctx *gin.Context, id int64, userID int64) error
	GetByID(ctx *gin.Context, id int64) (*team.TeamResponse, error)
	GetAll(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error)
	UpdateAddress(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error)
	UploadIcon(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error)
	DeleteIcon(ctx *gin.Context, id int64, callerID int64) error
}

type teamService struct {
	teamDao       daos.TeamDaoInterface
	userDao       daos.UserDaoInterface
	userRoleDao   daos.UserRoleDaoInterface
	roleDao       daos.RoleDaoInterface
	teamUserDao   daos.TeamUserDaoInterface
	groupDao      daos.GroupDaoInterface
	groupUserDao  daos.GroupUserDaoInterface
	invitationDao daos.InvitationDaoInterface
	storageClient storageclient.StorageClientInterface
}

// NewTeamService crea una nueva instancia de TeamService.
func NewTeamService(
	teamDao daos.TeamDaoInterface,
	userDao daos.UserDaoInterface,
	userRoleDao daos.UserRoleDaoInterface,
	roleDao daos.RoleDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	invitationDao daos.InvitationDaoInterface,
	storageClient storageclient.StorageClientInterface,
) TeamServiceInterface {
	return &teamService{
		teamDao:       teamDao,
		userDao:       userDao,
		userRoleDao:   userRoleDao,
		roleDao:       roleDao,
		teamUserDao:   teamUserDao,
		groupDao:      groupDao,
		groupUserDao:  groupUserDao,
		invitationDao: invitationDao,
		storageClient: storageClient,
	}
}

// UploadIcon valida y sube el ícono del equipo. Solo el entrenador dueño del
// equipo puede hacerlo.
func (s *teamService) UploadIcon(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error) {
	isEntrenador, err := s.isEntrenadorOfTeam(ctx, id, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for team icon upload", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("UploadIcon"))
		return nil, fmt.Errorf("error al subir el ícono")
	}
	if !isEntrenador {
		return nil, fmt.Errorf("solo el entrenador dueño del equipo puede cambiar el ícono")
	}

	if err := requireStorageClient(s.storageClient); err != nil {
		return nil, err
	}

	contentType, ext, err := validatePhotoContent(content)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("teams/team-%d-icon.%s", id, ext)
	if err := s.storageClient.Upload(ctx, key, content, contentType); err != nil {
		customlogger.Error(ctx, "error uploading team icon", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("UploadIcon"))
		return nil, fmt.Errorf("error al subir el ícono")
	}

	updatedAt := time.Now().UTC()
	if err := s.teamDao.UpdateIcon(ctx, id, key, updatedAt); err != nil {
		customlogger.Error(ctx, "error persisting team icon key", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("UploadIcon"))
	}

	return buildMediaURL(&key, &updatedAt), nil
}

// DeleteIcon borra el ícono del equipo del bucket y limpia icon_key/icon_updated_at.
// Solo el entrenador dueño del equipo puede hacerlo. Idempotente: si el equipo no
// tiene ícono cargado, no hace nada.
func (s *teamService) DeleteIcon(ctx *gin.Context, id int64, callerID int64) error {
	isEntrenador, err := s.isEntrenadorOfTeam(ctx, id, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for team icon delete", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("DeleteIcon"))
		return fmt.Errorf("error al borrar el ícono")
	}
	if !isEntrenador {
		return fmt.Errorf("solo el entrenador dueño del equipo puede cambiar el ícono")
	}

	if err := requireStorageClient(s.storageClient); err != nil {
		return err
	}

	teamDB, err := s.teamDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding team for icon delete", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("DeleteIcon"))
		return fmt.Errorf("error al borrar el ícono")
	}
	if teamDB == nil || teamDB.IconKey == nil {
		return nil
	}

	if err := s.storageClient.Delete(ctx, *teamDB.IconKey); err != nil {
		customlogger.Error(ctx, "error deleting team icon from storage", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("DeleteIcon"))
		return fmt.Errorf("error al borrar el ícono")
	}

	if err := s.teamDao.ClearIcon(ctx, id); err != nil {
		customlogger.Error(ctx, "error clearing team icon key", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("DeleteIcon"))
		return fmt.Errorf("error al borrar el ícono")
	}

	return nil
}

// Create crea un nuevo equipo a nombre del usuario autenticado, que pasa a ser el owner.
// El owner debe tener el rol "entrenador".
func (s *teamService) Create(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
	user, err := s.userDao.FindByID(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error finding owner user", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear equipo")
	}
	if user == nil {
		return nil, fmt.Errorf("el usuario owner no existe")
	}

	userRoles, err := s.userRoleDao.FindByUserID(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error finding owner roles", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)),
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
		OwnerID:      ownerID,
		Status:       "active",
	}
	if req.ShowGroupsToRunners != nil {
		teamDB.ShowGroupsToRunners = *req.ShowGroupsToRunners
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

	// El owner queda como miembro del equipo (rol entrenador) para que las
	// validaciones de pertenencia (ej. Delete) lo reconozcan. No se aborta la
	// creación si esto falla, mismo criterio tolerante que el alta del grupo
	// por defecto en TeamDelegate.
	ownerTeamUser := &dbs.TeamUser{
		TeamID:         teamDB.ID,
		UserID:         teamDB.OwnerID,
		RoleInTeam:     teamOwnerRoleName,
		Status:         "active",
		AssignmentDate: time.Now(),
	}
	if err := s.teamUserDao.Create(ctx, ownerTeamUser); err != nil {
		customlogger.Error(ctx, "error creating owner team_user membership", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamDB.ID)),
			customlogger.TagMethod("Create"))
	}

	return s.toResponse(teamDB), nil
}

// isEntrenadorOfTeam valida que userID sea miembro con rol "entrenador" del equipo.
func (s *teamService) isEntrenadorOfTeam(ctx *gin.Context, teamID, userID int64) (bool, error) {
	member, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, userID)
	if err != nil {
		return false, err
	}
	return member != nil && member.RoleInTeam == teamOwnerRoleName, nil
}

// Update actualiza los campos de un equipo existente. Solo el entrenador del equipo puede hacerlo.
func (s *teamService) Update(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
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

	isEntrenador, err := s.isEntrenadorOfTeam(ctx, id, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for team update", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar equipo")
	}
	if !isEntrenador {
		return nil, fmt.Errorf("solo el entrenador puede actualizar el equipo")
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
	if req.ShowGroupsToRunners != nil {
		teamDB.ShowGroupsToRunners = *req.ShowGroupsToRunners
	}
	if req.Visible != nil {
		teamDB.Visible = *req.Visible
	}
	if req.IsPublic != nil {
		teamDB.IsPublic = *req.IsPublic
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

	count, err := s.teamUserDao.CountActiveByTeamExcludingUser(ctx, id, userID)
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

	// Cascada: limpiar las filas huérfanas que quedarían apuntando a un equipo
	// ya eliminado (la fila team_users del propio owner que llamó Delete, los
	// grupos del equipo, sus group_users, e invitaciones pendientes). No bloquea
	// el éxito del delete si alguna falla, mismo criterio tolerante que el resto
	// de este archivo.
	if err := s.groupUserDao.SoftDeleteByTeamID(ctx, id); err != nil {
		customlogger.Error(ctx, "error cascading delete to group users", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
	}
	if err := s.groupDao.SoftDeleteByTeamID(ctx, id); err != nil {
		customlogger.Error(ctx, "error cascading delete to groups", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
	}
	if err := s.teamUserDao.SoftDeleteByTeamID(ctx, id); err != nil {
		customlogger.Error(ctx, "error cascading delete to team users", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
	}
	if err := s.invitationDao.SoftDeleteByTeamID(ctx, id); err != nil {
		customlogger.Error(ctx, "error cascading delete to invitations", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
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

// GetAll obtiene los equipos activos. Sin filtros, devuelve todos. Con owner_id
// y/o member_id, filtra por equipos administrados y/o equipos donde el usuario
// es miembro (cualquier rol). Si se pasan ambos, se aplican como AND (caso poco
// común, se resuelve filtrando en memoria sin agregar una query nueva al DAO).
func (s *teamService) GetAll(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
	var teams []dbs.Team
	var err error

	switch {
	case ownerID != nil && memberID != nil:
		teams, err = s.teamDao.GetAllByOwnerID(ctx, *ownerID)
		if err == nil {
			teams, err = s.filterByMember(ctx, teams, *memberID)
		}
	case ownerID != nil:
		teams, err = s.teamDao.GetAllByOwnerID(ctx, *ownerID)
	case memberID != nil:
		teams, err = s.teamDao.GetAllByMemberID(ctx, *memberID)
	default:
		teams, err = s.teamDao.GetAll(ctx)
	}

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

// filterByMember reduce una lista de equipos a los que el usuario indicado integra.
func (s *teamService) filterByMember(ctx *gin.Context, teams []dbs.Team, memberID int64) ([]dbs.Team, error) {
	filtered := make([]dbs.Team, 0, len(teams))
	for _, t := range teams {
		member, err := s.teamUserDao.FindByTeamAndUser(ctx, t.ID, memberID)
		if err != nil {
			return nil, err
		}
		if member != nil {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

// UpdateAddress actualiza la dirección de un equipo. Solo el entrenador del equipo puede hacerlo.
func (s *teamService) UpdateAddress(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
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

	isEntrenador, err := s.isEntrenadorOfTeam(ctx, id, callerID)
	if err != nil {
		customlogger.Error(ctx, "error checking caller role for team address update", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("UpdateAddress"))
		return nil, fmt.Errorf("error al actualizar dirección")
	}
	if !isEntrenador {
		return nil, fmt.Errorf("solo el entrenador puede actualizar el equipo")
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
		ID:                  t.ID,
		Name:                t.Name,
		Description:         t.Description,
		Level:               t.Level,
		MaxMembers:          t.MaxMembers,
		Requirements:        t.Requirements,
		OwnerID:             t.OwnerID,
		Status:              t.Status,
		Country:             t.Country,
		Province:            t.Province,
		City:                t.City,
		Street:              t.Street,
		Number:              t.Number,
		ShowGroupsToRunners: t.ShowGroupsToRunners,
		Visible:             t.Visible,
		IsPublic:            t.IsPublic,
		IconURL:             buildMediaURL(t.IconKey, t.IconUpdatedAt),
		CreatedAt:           t.CreatedAt,
		UpdatedAt:           t.UpdatedAt,
	}
}
