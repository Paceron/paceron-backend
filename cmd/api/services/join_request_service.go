package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/joinrequest"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrTeamNotFound              = errors.New("equipo no encontrado")
	ErrTeamNotPublic             = errors.New("el equipo no acepta solicitudes de ingreso")
	ErrTeamFull                  = errors.New("el equipo alcanzó su cupo máximo")
	ErrAlreadyMember             = errors.New("el usuario ya pertenece a este equipo")
	ErrJoinRequestAlreadyPending = errors.New("ya existe una solicitud pendiente a este equipo")
	ErrJoinRequestNotFound       = errors.New("solicitud no encontrada")
	ErrJoinRequestForbidden      = errors.New("no autorizado")
	ErrJoinRequestNotPending     = errors.New("la solicitud ya fue resuelta")
)

// JoinRequestServiceInterface define las operaciones de negocio para
// solicitudes de ingreso de un corredor a un equipo.
type JoinRequestServiceInterface interface {
	Create(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error)
	Cancel(ctx *gin.Context, requestID, callerID int64) error
	Accept(ctx *gin.Context, requestID, callerID int64) error
	Reject(ctx *gin.Context, requestID, callerID int64) error
	ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error)
	ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error)
	PendingCount(ctx *gin.Context, ownerID int64) (int64, error)
}

type joinRequestService struct {
	joinRequestDao daos.JoinRequestDaoInterface
	teamDao        daos.TeamDaoInterface
	teamUserDao    daos.TeamUserDaoInterface
	userDao        daos.UserDaoInterface
	groupDao       daos.GroupDaoInterface
	groupUserDao   daos.GroupUserDaoInterface
	installDao     daos.InstallmentDaoInterface
	db             *gorm.DB
}

// NewJoinRequestService crea una nueva instancia de JoinRequestService.
func NewJoinRequestService(
	joinRequestDao daos.JoinRequestDaoInterface,
	teamDao daos.TeamDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	userDao daos.UserDaoInterface,
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	installDao daos.InstallmentDaoInterface,
	db *gorm.DB,
) JoinRequestServiceInterface {
	return &joinRequestService{
		joinRequestDao: joinRequestDao,
		teamDao:        teamDao,
		teamUserDao:    teamUserDao,
		userDao:        userDao,
		groupDao:       groupDao,
		groupUserDao:   groupUserDao,
		installDao:     installDao,
		db:             db,
	}
}

// Create crea una solicitud de ingreso pending. Valida que el equipo exista,
// sea público, tenga cupo, y que el caller no sea ya miembro ni tenga otra
// solicitud pendiente al mismo equipo.
func (s *joinRequestService) Create(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if teamDB == nil {
		return nil, ErrTeamNotFound
	}
	if !teamDB.IsPublic {
		return nil, ErrTeamNotPublic
	}
	// El owner puede no tener fila en team_users si la inserción best-effort
	// de team_service.Create falló — chequeo explícito, no depender solo de
	// FindByTeamAndUser.
	if teamDB.OwnerID == runnerID {
		return nil, ErrAlreadyMember
	}

	existingMember, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, runnerID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership for join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if existingMember != nil {
		return nil, ErrAlreadyMember
	}

	count, err := s.teamUserDao.CountActiveByTeam(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error counting team members for join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if count >= teamDB.MaxMembers {
		return nil, ErrTeamFull
	}

	existingPending, err := s.joinRequestDao.FindPendingByTeamAndUser(ctx, teamID, runnerID)
	if err != nil {
		customlogger.Error(ctx, "error checking duplicate join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}
	if existingPending != nil {
		return nil, ErrJoinRequestAlreadyPending
	}

	jr := &dbs.JoinRequest{
		TeamID:   teamID,
		RunnerID: runnerID,
		Status:   string(constants.InvitationStatusPending),
	}
	if err := s.joinRequestDao.Create(ctx, jr); err != nil {
		customlogger.Error(ctx, "error creating join request", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear solicitud")
	}

	return s.toResponse(jr, teamDB.Name, s.runnerName(ctx, runnerID)), nil
}

// Cancel borra la solicitud (hard delete, no hay estado "cancelled" — D1) si
// el caller es su dueño y sigue pending.
func (s *joinRequestService) Cancel(ctx *gin.Context, requestID, callerID int64) error {
	jr, err := s.joinRequestDao.FindByID(ctx, requestID)
	if err != nil {
		customlogger.Error(ctx, "error finding join request for cancel", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Cancel"))
		return fmt.Errorf("error al cancelar solicitud")
	}
	if jr == nil {
		return ErrJoinRequestNotFound
	}
	if jr.RunnerID != callerID {
		return ErrJoinRequestForbidden
	}
	if jr.Status != string(constants.InvitationStatusPending) {
		return ErrJoinRequestNotPending
	}

	if err := s.joinRequestDao.Delete(ctx, requestID); err != nil {
		customlogger.Error(ctx, "error deleting join request on cancel", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Cancel"))
		return fmt.Errorf("error al cancelar solicitud")
	}
	return nil
}

// findPendingRequestForOwner carga una solicitud pending y valida que callerID
// sea el entrenador dueño del equipo al que pertenece. Compartido por Accept/Reject.
func (s *joinRequestService) findPendingRequestForOwner(ctx *gin.Context, requestID, callerID int64, method string) (*dbs.JoinRequest, *dbs.Team, error) {
	jr, err := s.joinRequestDao.FindByID(ctx, requestID)
	if err != nil {
		customlogger.Error(ctx, "error finding join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod(method))
		return nil, nil, fmt.Errorf("error al procesar solicitud")
	}
	if jr == nil {
		return nil, nil, ErrJoinRequestNotFound
	}
	if jr.Status != string(constants.InvitationStatusPending) {
		return nil, nil, ErrJoinRequestNotPending
	}

	teamDB, err := s.teamDao.FindByID(ctx, jr.TeamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod(method))
		return nil, nil, fmt.Errorf("error al procesar solicitud")
	}
	if teamDB == nil {
		return nil, nil, ErrTeamNotFound
	}
	if teamDB.OwnerID != callerID {
		return nil, nil, ErrJoinRequestForbidden
	}

	return jr, teamDB, nil
}

// Accept crea la membresía (gateada por membership_fee, mismo patrón secuencial
// que invitation_service.AcceptInvitation), asigna al grupo default, y marca la
// solicitud como accepted.
func (s *joinRequestService) Accept(ctx *gin.Context, requestID, callerID int64) error {
	jr, teamDB, err := s.findPendingRequestForOwner(ctx, requestID, callerID, "Accept")
	if err != nil {
		return err
	}

	existingMember, err := s.teamUserDao.FindByTeamAndUser(ctx, teamDB.ID, jr.RunnerID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership on accept join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
		return fmt.Errorf("error al aceptar solicitud")
	}

	if existingMember == nil {
		count, err := s.teamUserDao.CountActiveByTeam(ctx, teamDB.ID)
		if err != nil {
			customlogger.Error(ctx, "error counting team members on accept", err,
				customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
			return fmt.Errorf("error al aceptar solicitud")
		}
		if count >= teamDB.MaxMembers {
			return ErrTeamFull
		}

		teamUser := &dbs.TeamUser{
			TeamID:         teamDB.ID,
			UserID:         jr.RunnerID,
			RoleInTeam:     string(constants.TeamUserRoleCorredor),
			Status:         "active",
			AssignmentDate: time.Now(),
		}
		if err := ApplyTeamMembershipGate(ctx, s.db, s.teamUserDao, s.installDao, teamUser, teamDB.MembershipFee); err != nil {
			customlogger.Error(ctx, "error creating team_user on accept join request", err,
				customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
			return fmt.Errorf("error al aceptar solicitud")
		}
	}

	AssignToDefaultGroup(ctx, s.groupDao, s.groupUserDao, teamDB.ID, nil, jr.RunnerID)

	if err := s.joinRequestDao.UpdateStatus(ctx, jr.ID, string(constants.InvitationStatusAccepted)); err != nil {
		customlogger.Error(ctx, "team_user creado pero join request no pudo marcarse como aceptada", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
		return fmt.Errorf("error al aceptar solicitud")
	}

	customlogger.Info(ctx, "join request accepted successfully",
		customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Accept"))
	return nil
}

// Reject marca la solicitud como rejected sin crear ninguna membresía.
func (s *joinRequestService) Reject(ctx *gin.Context, requestID, callerID int64) error {
	jr, _, err := s.findPendingRequestForOwner(ctx, requestID, callerID, "Reject")
	if err != nil {
		return err
	}

	if err := s.joinRequestDao.UpdateStatus(ctx, jr.ID, string(constants.InvitationStatusRejected)); err != nil {
		customlogger.Error(ctx, "error rejecting join request", err,
			customlogger.Tag("join_request_id", fmt.Sprintf("%d", requestID)), customlogger.TagMethod("Reject"))
		return fmt.Errorf("error al rechazar solicitud")
	}
	return nil
}

// ListMine devuelve todas las solicitudes del corredor, cualquier estado.
func (s *joinRequestService) ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error) {
	requests, err := s.joinRequestDao.FindByUser(ctx, runnerID)
	if err != nil {
		customlogger.Error(ctx, "error listing join requests for user", err,
			customlogger.Tag("runner_id", fmt.Sprintf("%d", runnerID)), customlogger.TagMethod("ListMine"))
		return nil, fmt.Errorf("error al listar solicitudes")
	}

	teamIDs := make([]int64, 0, len(requests))
	seenTeam := make(map[int64]struct{}, len(requests))
	runnerIDs := make([]int64, 0, len(requests))
	seenRunner := make(map[int64]struct{}, len(requests))
	for _, jr := range requests {
		if _, seen := seenTeam[jr.TeamID]; !seen {
			seenTeam[jr.TeamID] = struct{}{}
			teamIDs = append(teamIDs, jr.TeamID)
		}
		if _, seen := seenRunner[jr.RunnerID]; !seen {
			seenRunner[jr.RunnerID] = struct{}{}
			runnerIDs = append(runnerIDs, jr.RunnerID)
		}
	}

	teamNames := make(map[int64]string, len(teamIDs))
	if len(teamIDs) > 0 {
		if teams, err := s.teamDao.FindByIDs(ctx, teamIDs); err == nil {
			for _, team := range teams {
				teamNames[team.ID] = team.Name
			}
		}
	}
	runnerNames := s.runnerNames(ctx, runnerIDs)

	responses := make([]joinrequest.JoinRequestResponse, len(requests))
	for i, jr := range requests {
		responses[i] = *s.toResponse(&jr, teamNames[jr.TeamID], runnerNames[jr.RunnerID])
	}
	return responses, nil
}

// ListByTeam devuelve las solicitudes pending de un equipo, solo para su
// entrenador dueño.
func (s *joinRequestService) ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for join request list", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("ListByTeam"))
		return nil, fmt.Errorf("error al listar solicitudes")
	}
	if teamDB == nil {
		return nil, ErrTeamNotFound
	}
	if teamDB.OwnerID != callerID {
		return nil, ErrJoinRequestForbidden
	}

	requests, err := s.joinRequestDao.FindPendingByTeam(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error listing pending join requests for team", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)), customlogger.TagMethod("ListByTeam"))
		return nil, fmt.Errorf("error al listar solicitudes")
	}

	runnerIDs := make([]int64, 0, len(requests))
	seenRunner := make(map[int64]struct{}, len(requests))
	for _, jr := range requests {
		if _, seen := seenRunner[jr.RunnerID]; seen {
			continue
		}
		seenRunner[jr.RunnerID] = struct{}{}
		runnerIDs = append(runnerIDs, jr.RunnerID)
	}
	runnerNames := s.runnerNames(ctx, runnerIDs)

	responses := make([]joinrequest.JoinRequestResponse, len(requests))
	for i, jr := range requests {
		responses[i] = *s.toResponse(&jr, teamDB.Name, runnerNames[jr.RunnerID])
	}
	return responses, nil
}

// PendingCount suma las solicitudes pending en todos los equipos que
// administra ownerID, para el badge del entrenador.
func (s *joinRequestService) PendingCount(ctx *gin.Context, ownerID int64) (int64, error) {
	count, err := s.joinRequestDao.CountPendingByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error counting pending join requests", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)), customlogger.TagMethod("PendingCount"))
		return 0, fmt.Errorf("error al contar solicitudes")
	}
	return count, nil
}

// runnerName resuelve el nombre de un único corredor (Create).
func (s *joinRequestService) runnerName(ctx *gin.Context, runnerID int64) string {
	if runner, err := s.userDao.FindByID(ctx, runnerID); err == nil && runner != nil {
		return runner.Name + " " + runner.Surname
	}
	return ""
}

// runnerNames resuelve los nombres de varios corredores en una sola query. Las
// referencias inexistentes quedan fuera del map y el caller usa "" igual que
// antes.
func (s *joinRequestService) runnerNames(ctx *gin.Context, runnerIDs []int64) map[int64]string {
	names := make(map[int64]string, len(runnerIDs))
	if len(runnerIDs) == 0 {
		return names
	}
	users, err := s.userDao.FindByIDs(ctx, runnerIDs)
	if err != nil {
		return names
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		names[user.ID] = user.Name + " " + user.Surname
	}
	return names
}

// toResponse convierte un dbs.JoinRequest a su DTO de respuesta con los nombres
// ya resueltos por el caller.
func (s *joinRequestService) toResponse(jr *dbs.JoinRequest, teamName, runnerName string) *joinrequest.JoinRequestResponse {
	return &joinrequest.JoinRequestResponse{
		ID:         jr.ID,
		TeamID:     jr.TeamID,
		TeamName:   teamName,
		RunnerID:   jr.RunnerID,
		RunnerName: runnerName,
		Status:     jr.Status,
		CreatedAt:  jr.CreatedAt,
	}
}
