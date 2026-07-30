package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/invitation"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

// invitationExpiryDuration es el vencimiento informativo de una invitación pendiente.
// No es un mecanismo de seguridad (la identidad se resuelve por user_id, no por
// posesión de un token) sino una regla de negocio para no dejar invitaciones
// "vivas" indefinidamente. Se chequea de forma perezosa (sin job de limpieza),
// igual que password_reset_service chequea time.Now().After(tokenDB.ExpiresAt).
const invitationExpiryDuration = 15 * 24 * time.Hour

// InvitationServiceInterface define las operaciones de negocio para invitaciones.
type InvitationServiceInterface interface {
	InviteRunner(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error)
	ListPendingInvitations(ctx *gin.Context, teamID int64) ([]invitation.InvitationResponse, error)
	AcceptInvitation(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error)
	RejectInvitation(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error)
}

type invitationService struct {
	userDao       daos.UserDaoInterface
	teamDao       daos.TeamDaoInterface
	invitationDao daos.InvitationDaoInterface
	teamUserDao   daos.TeamUserDaoInterface
	mailer        mailer.MailerInterface
}

// NewInvitationService crea una nueva instancia de InvitationService.
func NewInvitationService(
	userDao daos.UserDaoInterface,
	teamDao daos.TeamDaoInterface,
	invitationDao daos.InvitationDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	mailerClient mailer.MailerInterface,
) InvitationServiceInterface {
	return &invitationService{
		userDao:       userDao,
		teamDao:       teamDao,
		invitationDao: invitationDao,
		teamUserDao:   teamUserDao,
		mailer:        mailerClient,
	}
}

// InviteRunner envía una invitación por email a un usuario existente para unirlo a un equipo,
// y persiste la invitación para que el invitado pueda verla y responderla desde la app.
func (s *invitationService) InviteRunner(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for invitation", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("InviteRunner"))
		return nil, fmt.Errorf("error al enviar invitación")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	user, err := s.userDao.FindByEmail(ctx, req.Email)
	if err != nil {
		customlogger.Error(ctx, "error finding user for invitation", err,
			customlogger.Tag("email", req.Email),
			customlogger.TagMethod("InviteRunner"))
		return nil, fmt.Errorf("error al enviar invitación")
	}
	if user == nil {
		return nil, fmt.Errorf("no se encontró un usuario con el email proporcionado")
	}

	existingMember, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, user.ID)
	if err != nil {
		customlogger.Error(ctx, "error checking existing membership for invitation", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", user.ID)),
			customlogger.TagMethod("InviteRunner"))
		return nil, fmt.Errorf("error al enviar invitación")
	}
	if existingMember != nil {
		return nil, fmt.Errorf("el usuario ya pertenece a este equipo")
	}

	existingInvite, err := s.invitationDao.FindPendingByTeamAndInvitee(ctx, teamID, user.ID)
	if err != nil {
		customlogger.Error(ctx, "error checking existing invitation", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", user.ID)),
			customlogger.TagMethod("InviteRunner"))
		return nil, fmt.Errorf("error al enviar invitación")
	}
	if existingInvite != nil {
		return nil, fmt.Errorf("ya existe una invitación pendiente para este usuario en este equipo")
	}

	inv := &dbs.Invitation{
		TeamID:    teamID,
		InviterID: teamDB.OwnerID,
		InviteeID: user.ID,
		Status:    string(constants.InvitationStatusPending),
		ExpiresAt: time.Now().Add(invitationExpiryDuration),
	}
	if err := s.invitationDao.Create(ctx, inv); err != nil {
		customlogger.Error(ctx, "error creating invitation", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", user.ID)),
			customlogger.TagMethod("InviteRunner"))
		return nil, fmt.Errorf("error al enviar invitación")
	}

	if err := s.mailer.SendEmail(ctx, user.Email, mailer.EmailTypeInvitation, mailer.EmailData{Name: user.Name, TeamName: teamDB.Name}); err != nil {
		customlogger.Error(ctx, "error sending invitation email", err,
			customlogger.Tag("email", user.Email),
			customlogger.TagMethod("InviteRunner"))
		return nil, fmt.Errorf("error al enviar el email de invitación")
	}

	customlogger.Info(ctx, "invitation sent successfully",
		customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
		customlogger.Tag("email", user.Email),
		customlogger.TagMethod("InviteRunner"))

	return &invitation.InviteRunnerResponse{
		Message: fmt.Sprintf("Invitación enviada exitosamente a %s", user.Email),
	}, nil
}

// ListPendingInvitations devuelve las invitaciones pendientes de un equipo, excluyendo
// las que ya vencieron (chequeo perezoso, sin job de limpieza).
func (s *invitationService) ListPendingInvitations(ctx *gin.Context, teamID int64) ([]invitation.InvitationResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for listing invitations", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("ListPendingInvitations"))
		return nil, fmt.Errorf("error al listar invitaciones")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	invitations, err := s.invitationDao.FindPendingByTeamID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error listing pending invitations", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("ListPendingInvitations"))
		return nil, fmt.Errorf("error al listar invitaciones")
	}

	now := time.Now()
	responses := make([]invitation.InvitationResponse, 0, len(invitations))
	for _, inv := range invitations {
		if now.After(inv.ExpiresAt) {
			continue
		}

		inviteeName, inviteeEmail := "", ""
		inviteeUser, err := s.userDao.FindByID(ctx, inv.InviteeID)
		if err != nil {
			customlogger.Error(ctx, "error finding invitee user for invitation listing", err,
				customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
				customlogger.Tag("invitee_id", fmt.Sprintf("%d", inv.InviteeID)),
				customlogger.TagMethod("ListPendingInvitations"))
		} else if inviteeUser != nil {
			inviteeName = inviteeUser.Name
			inviteeEmail = inviteeUser.Email
		}

		responses = append(responses, invitation.InvitationResponse{
			ID:           inv.ID,
			TeamID:       inv.TeamID,
			InviteeID:    inv.InviteeID,
			InviteeName:  inviteeName,
			InviteeEmail: inviteeEmail,
			Status:       inv.Status,
			ExpiresAt:    inv.ExpiresAt,
			CreatedAt:    inv.CreatedAt,
		})
	}

	return responses, nil
}

// AcceptInvitation acepta una invitación pendiente: da de alta al invitado como corredor
// del equipo y marca la invitación como aceptada. El alta en TeamUser se hace ANTES de
// marcar la invitación como aceptada: si falla el segundo paso, un reintento del propio
// usuario detecta que ya es miembro (ver más abajo) y solo corrige el estado de la
// invitación sin duplicar el alta. El orden inverso dejaría al usuario con la invitación
// "aceptada" pero sin acceso real al equipo, sin ninguna vía de reintento.
func (s *invitationService) AcceptInvitation(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
	inv, respErr := s.findRespondableInvitation(ctx, invitationID, userID, "AcceptInvitation")
	if respErr != nil {
		return nil, respErr
	}

	existingMember, err := s.teamUserDao.FindByTeamAndUser(ctx, inv.TeamID, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking membership on accept invitation", err,
			customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
			customlogger.TagMethod("AcceptInvitation"))
		return nil, fmt.Errorf("error al procesar la invitación")
	}

	if existingMember == nil {
		teamUser := &dbs.TeamUser{
			TeamID:         inv.TeamID,
			UserID:         userID,
			RoleInTeam:     string(constants.TeamUserRoleCorredor),
			Status:         "active",
			AssignmentDate: time.Now(),
		}
		if err := s.teamUserDao.Create(ctx, teamUser); err != nil {
			customlogger.Error(ctx, "error creating team_user on accept invitation", err,
				customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
				customlogger.TagMethod("AcceptInvitation"))
			return nil, fmt.Errorf("error al procesar la invitación")
		}
	}

	if err := s.invitationDao.UpdateStatus(ctx, inv.ID, string(constants.InvitationStatusAccepted), time.Now()); err != nil {
		customlogger.Error(ctx, "team_user creado pero invitación no pudo marcarse como aceptada", err,
			customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
			customlogger.TagMethod("AcceptInvitation"))
		return nil, fmt.Errorf("error al procesar la invitación")
	}

	customlogger.Info(ctx, "invitation accepted successfully",
		customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
		customlogger.TagMethod("AcceptInvitation"))

	return &invitation.RespondInvitationResponse{Message: "Invitación aceptada"}, nil
}

// RejectInvitation rechaza una invitación pendiente, sin afectar TeamUser.
func (s *invitationService) RejectInvitation(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
	inv, respErr := s.findRespondableInvitation(ctx, invitationID, userID, "RejectInvitation")
	if respErr != nil {
		return nil, respErr
	}

	if err := s.invitationDao.UpdateStatus(ctx, inv.ID, string(constants.InvitationStatusRejected), time.Now()); err != nil {
		customlogger.Error(ctx, "error rejecting invitation", err,
			customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
			customlogger.TagMethod("RejectInvitation"))
		return nil, fmt.Errorf("error al procesar la invitación")
	}

	customlogger.Info(ctx, "invitation rejected successfully",
		customlogger.Tag("invitation_id", fmt.Sprintf("%d", inv.ID)),
		customlogger.TagMethod("RejectInvitation"))

	return &invitation.RespondInvitationResponse{Message: "Invitación rechazada"}, nil
}

// findRespondableInvitation centraliza las validaciones comunes a accept/reject:
// la invitación existe, pertenece al usuario que responde, sigue pendiente y no venció.
func (s *invitationService) findRespondableInvitation(ctx *gin.Context, invitationID, userID int64, method string) (*dbs.Invitation, error) {
	inv, err := s.invitationDao.FindByID(ctx, invitationID)
	if err != nil {
		customlogger.Error(ctx, "error finding invitation", err,
			customlogger.Tag("invitation_id", fmt.Sprintf("%d", invitationID)),
			customlogger.TagMethod(method))
		return nil, fmt.Errorf("error al procesar la invitación")
	}
	if inv == nil {
		return nil, fmt.Errorf("invitación no encontrada")
	}
	if inv.InviteeID != userID {
		return nil, fmt.Errorf("la invitación no pertenece a este usuario")
	}
	if inv.Status != string(constants.InvitationStatusPending) {
		return nil, fmt.Errorf("la invitación ya fue respondida")
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, fmt.Errorf("la invitación ha expirado")
	}
	return inv, nil
}
