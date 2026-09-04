package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/teambio"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// TeamSubscriptionServiceInterface maneja la suscripción del corredor a un
// equipo: estado de cuenta (D3) y reglas de membresía/cuotas (D2/D4/D5).
type TeamSubscriptionServiceInterface interface {
	GetTeamSubscription(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error)
}

type teamSubscriptionService struct {
	teamDao     daos.TeamDaoInterface
	teamUserDao daos.TeamUserDaoInterface
	installDao  daos.InstallmentDaoInterface
}

func NewTeamSubscriptionService(
	teamDao daos.TeamDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	installDao daos.InstallmentDaoInterface,
) TeamSubscriptionServiceInterface {
	return &teamSubscriptionService{
		teamDao:     teamDao,
		teamUserDao: teamUserDao,
		installDao:  installDao,
	}
}

// GetTeamSubscription devuelve el estado de cuenta de un corredor para un equipo
// (GET /api/v1/users/:id/teams/:team_id/subscription). Shape D3: equipo +
// membresía + próxima cuota + has_debt + datos checkout marketplace. Equipo
// gratis → subscription_status = active, sin cuota ni mercadopago.
func (s *teamSubscriptionService) GetTeamSubscription(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for subscription", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetTeamSubscription"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	membership, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding membership for subscription", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetTeamSubscription"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}
	if membership == nil {
		return nil, fmt.Errorf("el usuario no pertenece a este equipo")
	}

	resp := &teambio.TeamSubscriptionResponse{
		Team: teambio.TeamInfo{
			ID:            teamDB.ID,
			Name:          teamDB.Name,
			MembershipFee: teamDB.MembershipFee,
		},
	}

	// Equipo gratis: sin suscripción (membership active sin cuotas).
	if teamDB.MembershipFee == 0 {
		resp.Membership = teambio.MembershipInfo{
			SubscriptionStatus: string(constants.SubscriptionStatusActive),
			InitAmount:         membership.InitAmount,
			PaidInstallments:   membership.PaidInstallments,
		}
		if !membership.AssignmentDate.IsZero() {
			start := membership.AssignmentDate
			resp.Membership.StartDate = &start
		}
		return resp, nil
	}

	// Equipo pago: la membresía modela el estado (first_payment_pending/active).
	paid := membership.PaidInstallments
	resp.Membership = teambio.MembershipInfo{
		SubscriptionStatus: membership.SubscriptionStatus,
		InitAmount:         membership.InitAmount,
		PaidInstallments:   paid,
	}
	if !membership.AssignmentDate.IsZero() {
		start := membership.AssignmentDate
		resp.Membership.StartDate = &start
	}
	// Equipos anteriores al change: membership sin subscription_status pero con
	// membership_fee cobrado — se tratan como active (historial conservado).
	if resp.Membership.SubscriptionStatus == "" {
		resp.Membership.SubscriptionStatus = string(constants.SubscriptionStatusActive)
	}

	pending, err := s.installDao.FindPendingByUserTeam(ctx, teamID, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding pending installments for subscription", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetTeamSubscription"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}
	resp.HasDebt = HasPendingDebt(pending)

	next, err := s.installDao.FindNextByUserTeam(ctx, teamID, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding next installment for subscription", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetTeamSubscription"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}

	if next != nil {
		resp.NextInstallment = &teambio.NextInstallmentInfo{
			InstallmentID:     next.ID,
			InstallmentNumber: next.InstallmentNumber,
			InstallmentAmount: next.Amount,
			NextDueDate:       next.DueDate,
			BlockedDate:       next.BlockedDate,
		}
	}

	// El checkout Bricks marketplace solo aplica a equipos con mensualidad.
	resp.MercadoPago = &teambio.MercadoPagoCheckout{
		PublicKey:   config.MyMP.PublicKey,
		Concept:     string(constants.PaymentConceptTeamSubscription),
		Marketplace: true,
	}

	return resp, nil
}