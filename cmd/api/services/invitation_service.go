package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/invitation"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

// InvitationServiceInterface define las operaciones de negocio para invitaciones.
type InvitationServiceInterface interface {
	InviteRunner(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error)
}

type invitationService struct {
	userDao daos.UserDaoInterface
	teamDao daos.TeamDaoInterface
	mailer  mailer.MailerInterface
}

// NewInvitationService crea una nueva instancia de InvitationService.
func NewInvitationService(
	userDao daos.UserDaoInterface,
	teamDao daos.TeamDaoInterface,
	mailerClient mailer.MailerInterface,
) InvitationServiceInterface {
	return &invitationService{
		userDao: userDao,
		teamDao: teamDao,
		mailer:  mailerClient,
	}
}

// InviteRunner envía una invitación por email a un usuario existente para unirlo a un equipo.
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

	if err := s.mailer.SendInvitationEmail(ctx, user.Email, user.Name, teamDB.Name); err != nil {
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
