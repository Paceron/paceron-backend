package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teambio"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// ErrTeamSubscriptionForbidden indica que el caller no administra el equipo.
var ErrTeamSubscriptionForbidden = errors.New("no autorizado")

// TeamSubscriptionServiceInterface maneja la suscripción del corredor a un
// equipo: estado de cuenta (D3) y reglas de membresía/cuotas (D2/D4/D5).
type TeamSubscriptionServiceInterface interface {
	GetTeamSubscription(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error)
	GetTeamSubscriptions(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error)
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

// GetTeamSubscriptions devuelve el estado de cuota de todos los corredores
// activos de un equipo en una sola consulta batch. Solo el owner o un
// entrenador del equipo pueden consultarlo.
func (s *teamSubscriptionService) GetTeamSubscriptions(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error) {
	teamDB, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for subscriptions", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetTeamSubscriptions"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}
	if teamDB == nil {
		return nil, fmt.Errorf("equipo no encontrado")
	}

	if callerID != teamDB.OwnerID {
		caller, err := s.teamUserDao.FindByTeamAndUser(ctx, teamID, callerID)
		if err != nil {
			customlogger.Error(ctx, "error finding caller membership for subscriptions", err,
				customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
				customlogger.Tag("user_id", fmt.Sprintf("%d", callerID)),
				customlogger.TagMethod("GetTeamSubscriptions"))
			return nil, fmt.Errorf("error al obtener el estado de cuenta")
		}
		if caller == nil || caller.RoleInTeam != string(constants.TeamUserRoleEntrenador) || caller.Status != "active" {
			return nil, ErrTeamSubscriptionForbidden
		}
	}

	teamUsers, err := s.teamUserDao.FindByTeamID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team users for subscriptions", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetTeamSubscriptions"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}

	pending, err := s.installDao.FindPendingByTeam(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding pending installments for subscriptions", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("GetTeamSubscriptions"))
		return nil, fmt.Errorf("error al obtener el estado de cuenta")
	}
	pendingByUser := make(map[int64][]dbs.Installment)
	for _, installment := range pending {
		pendingByUser[installment.UserID] = append(pendingByUser[installment.UserID], installment)
	}

	now := time.Now()
	members := make([]teambio.TeamMemberSubscriptionInfo, 0, len(teamUsers))
	for i := range teamUsers {
		member := &teamUsers[i]
		if member.RoleInTeam != string(constants.TeamUserRoleCorredor) || member.Status != "active" {
			continue
		}

		rawStatus := member.SubscriptionStatus
		if rawStatus == "" {
			rawStatus = string(constants.SubscriptionStatusActive)
		}
		freeMembership := teamDB.MembershipFee == 0 && member.InitAmount <= 0
		hasDebt := false
		if !freeMembership {
			hasDebt = HasPendingDebt(pendingByUser[member.UserID]) || membershipOverdue(member, now) > 0
		} else {
			rawStatus = string(constants.SubscriptionStatusActive)
		}

		status := string(constants.SubscriptionStatusActive)
		switch {
		case hasDebt:
			status = "overdue"
		case rawStatus == string(constants.SubscriptionStatusFirstPaymentPending):
			status = string(constants.SubscriptionStatusFirstPaymentPending)
		}

		members = append(members, teambio.TeamMemberSubscriptionInfo{
			UserID:             member.UserID,
			Status:             status,
			SubscriptionStatus: rawStatus,
			HasDebt:            hasDebt,
			InitAmount:         member.InitAmount,
			PaidInstallments:   member.PaidInstallments,
		})
	}

	return &teambio.TeamSubscriptionsResponse{
		TeamID:        teamDB.ID,
		MembershipFee: teamDB.MembershipFee,
		Members:       members,
	}, nil
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

	// Equipo gratis: sin suscripción (membership active sin cuotas). Si se unió
	// cuando cobraba (init_amount > 0) sigue debiendo lo de esa membresía.
	if teamDB.MembershipFee == 0 && membership.InitAmount <= 0 {
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
	// Las cuotas vencidas sin fila (la N+1 recién existe al pagar la N) también
	// son deuda: se calculan desde la membresía (change cobros-pendientes-y-vencimientos).
	resp.HasDebt = HasPendingDebt(pending) || membershipOverdue(membership, time.Now()) > 0

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
