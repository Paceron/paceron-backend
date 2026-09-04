package services

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// ApplyTeamMembershipGate aplica el gate D2 (change suscripcion-teams-split) al
// sumar un corredor a un equipo, tanto por Andar (team_user_service.AddUser) como
// por aceptar una invitación (invitation_service.AcceptInvitation):
//   - membership_fee == 0 → team_user con subscription_status = active, sin cuotas.
//   - membership_fee > 0 → team_user con subscription_status = first_payment_pending,
//     init_amount = membership_fee, paid_installments = 0 + cuota #1 en installments
//     (team_id seteado, subscription_id nulo, status pending, amount = membership_fee,
//     sin due_date/blocked_date). Todo en una transacción GORM cuando hay db.
//
// El corredor queda con la fila de membresía pero su acceso pleno (active) recién
// llega al pagarse la cuota #1.
func ApplyTeamMembershipGate(
	ctx *gin.Context,
	db *gorm.DB,
	teamUserDao daos.TeamUserDaoInterface,
	installDao daos.InstallmentDaoInterface,
	teamUser *dbs.TeamUser,
	membershipFee float64,
) error {
	if membershipFee == 0 {
		teamUser.SubscriptionStatus = string(constants.SubscriptionStatusActive)
		return teamUserDao.Create(ctx, teamUser)
	}

	teamUser.SubscriptionStatus = string(constants.SubscriptionStatusFirstPaymentPending)
	teamUser.InitAmount = membershipFee
	teamUser.PaidInstallments = 0

	apply := func(tuDao daos.TeamUserDaoInterface, insDao daos.InstallmentDaoInterface) error {
		if err := tuDao.Create(ctx, teamUser); err != nil {
			return err
		}
		installment := FirstInstallment(nil, &teamUser.TeamID, teamUser.UserID, membershipFee)
		return insDao.Create(ctx, installment)
	}

	if db != nil {
		return db.Transaction(func(tx *gorm.DB) error {
			return apply(daos.NewTeamUserDao(tx), daos.NewInstallmentDao(tx))
		})
	}
	return apply(teamUserDao, installDao)
}