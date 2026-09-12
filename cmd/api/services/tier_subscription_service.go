package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tiersubscription"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// TierSubscriptionServiceInterface maneja el ledger de suscripciones de tier por
// usuario/rol: cambiar de tier (D4) y consultar la próxima cuota a pagar (D9).
type TierSubscriptionServiceInterface interface {
	ChangeTier(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error)
	GetCurrentSubscription(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error)
	CancelPendingSubscription(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error)
}

type tierSubscriptionService struct {
	db          *gorm.DB
	userRoleDao daos.UserRoleDaoInterface
	roleDao     daos.RoleDaoInterface
	tierDao     daos.TierDaoInterface
	tierSubDao  daos.TierSubscriptionDaoInterface
	installDao  daos.InstallmentDaoInterface
}

func NewTierSubscriptionService(
	db *gorm.DB,
	userRoleDao daos.UserRoleDaoInterface,
	roleDao daos.RoleDaoInterface,
	tierDao daos.TierDaoInterface,
	tierSubDao daos.TierSubscriptionDaoInterface,
	installDao daos.InstallmentDaoInterface,
) TierSubscriptionServiceInterface {
	return &tierSubscriptionService{
		db:          db,
		userRoleDao: userRoleDao,
		roleDao:     roleDao,
		tierDao:     tierDao,
		tierSubDao:  tierSubDao,
		installDao:  installDao,
	}
}

// ChangeTier aplica PUT /api/v1/users/:id/roles/:role_id/tier con las
// validaciones D4 en orden: (1) asignación previa, (2) tier del rol correcto,
// (3) sin deuda, (4) sin primer pago impago. Cierra la sub vigente y crea la
// nueva (target pago -> first_payment_pending + cuota #1; target gratis -> active
// + tier sync inmediato). Con db seteada corre todo en una transacción GORM.
func (s *tierSubscriptionService) ChangeTier(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
	customlogger.Info(ctx, "ChangeTier start",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
		customlogger.Tag("target_tier_id", fmt.Sprintf("%d", req.TierID)),
		customlogger.TagMethod("ChangeTier"))
	apply := func(
		urDao daos.UserRoleDaoInterface,
		roleDao daos.RoleDaoInterface,
		tierDao daos.TierDaoInterface,
		subDao daos.TierSubscriptionDaoInterface,
		insDao daos.InstallmentDaoInterface,
	) (*tiersubscription.ChangeTierResponse, error) {
		ur, err := urDao.FindByUserAndRole(ctx, userID, roleID)
		if err != nil {
			return nil, fmt.Errorf("error al cambiar de tier")
		}
		if ur == nil {
			return nil, fmt.Errorf("el usuario no tiene asignado este rol")
		}

		target, err := tierDao.FindByID(ctx, req.TierID)
		if err != nil {
			return nil, fmt.Errorf("error al cambiar de tier")
		}
		if target == nil {
			return nil, fmt.Errorf("tier no encontrado")
		}
		if target.RoleID != roleID {
			return nil, fmt.Errorf("el tier no pertenece al rol especificado")
		}

		role, err := roleDao.FindByID(ctx, roleID)
		if err != nil {
			return nil, fmt.Errorf("error al cambiar de tier")
		}
		if role == nil {
			return nil, fmt.Errorf("rol no encontrado")
		}

		sub, err := subDao.FindActiveByUserRole(ctx, userID, roleID)
		if err != nil {
			return nil, fmt.Errorf("error al cambiar de tier")
		}

		if sub != nil {
			customlogger.Info(ctx, "ChangeTier found active sub",
				customlogger.Tag("sub_id", fmt.Sprintf("%d", sub.ID)),
				customlogger.Tag("sub_tier_id", fmt.Sprintf("%d", sub.TierID)),
				customlogger.Tag("sub_status", sub.Status),
				customlogger.Tag("paid_installments", fmt.Sprintf("%d", sub.PaidInstallments)),
				customlogger.TagMethod("ChangeTier"))
			pending, err := insDao.FindPendingBySubscription(ctx, sub.ID)
			if err != nil {
				return nil, fmt.Errorf("error al cambiar de tier")
			}
			now := time.Now()
			for _, ins := range pending {
				if ins.BlockedDate != nil && ins.BlockedDate.Before(now) {
					return nil, fmt.Errorf("no podés cambiar de tier con deuda pendiente")
				}
			}
			if sub.Status == string(constants.SubscriptionStatusFirstPaymentPending) {
				return nil, fmt.Errorf("no podés cambiar de tier con el primer pago pendiente")
			}
		}

		resp := &tiersubscription.ChangeTierResponse{}
		resp.Tier = tiersubscription.TierInfo{
			ID:              target.ID,
			Name:            target.Name,
			Hierarchy:       target.Hierarchy,
			PaymentRequired: target.PaymentRequired,
		}
		resp.Role = tiersubscription.RoleInfo{ID: role.ID, Name: role.Name}

		if sub != nil {
			if err := subDao.SetEnded(ctx, sub.ID); err != nil {
				return nil, fmt.Errorf("error al cambiar de tier")
			}
			customlogger.Info(ctx, "ChangeTier ended previous sub",
				customlogger.Tag("old_sub_id", fmt.Sprintf("%d", sub.ID)),
				customlogger.Tag("old_sub_tier_id", fmt.Sprintf("%d", sub.TierID)),
				customlogger.TagMethod("ChangeTier"))
		}

		newSub := &dbs.UserRoleTierSubscription{
			UserID:     userID,
			RoleID:     roleID,
			TierID:     target.ID,
			StartDate:  time.Now(),
			InitAmount: target.TierAmount,
		}

		if target.PaymentRequired {
			// Target pago: sub first_payment_pending + cuota #1 pendiente. El tier de
			// acceso (user_roles.tier_id) se conserva hasta pagar la cuota #1 (D3).
			newSub.Status = string(constants.SubscriptionStatusFirstPaymentPending)
			if err := subDao.Create(ctx, newSub); err != nil {
				return nil, fmt.Errorf("error al cambiar de tier")
			}
			installment := FirstInstallment(&newSub.ID, nil, userID, target.TierAmount)
			if err := insDao.Create(ctx, installment); err != nil {
				return nil, fmt.Errorf("error al cambiar de tier")
			}
			customlogger.Info(ctx, "ChangeTier created paid sub",
				customlogger.Tag("new_sub_id", fmt.Sprintf("%d", newSub.ID)),
				customlogger.Tag("new_sub_tier_id", fmt.Sprintf("%d", newSub.TierID)),
				customlogger.Tag("new_sub_status", newSub.Status),
				customlogger.Tag("installment_id", fmt.Sprintf("%d", installment.ID)),
				customlogger.Tag("installment_number", fmt.Sprintf("%d", installment.InstallmentNumber)),
				customlogger.Tag("amount", fmt.Sprintf("%.2f", installment.Amount)),
				customlogger.TagMethod("ChangeTier"))

			resp.SubscriptionID = newSub.ID
			resp.SubscriptionStatus = newSub.Status
			paid := newSub.PaidInstallments
			resp.PaidInstallments = &paid
			num := installment.InstallmentNumber
			resp.InstallmentID = &installment.ID
			resp.InstallmentNumber = &num
			resp.InstallmentAmount = &installment.Amount
			resp.NextDueDate = installment.DueDate
			resp.BlockedDate = installment.BlockedDate
			resp.MercadoPago = &tiersubscription.MercadoPagoInfo{PublicKey: config.MyMP.PublicKey}
			return resp, nil
		}

		// Target gratis: sub active sin cuota + tier sync inmediato (D4).
		newSub.Status = string(constants.SubscriptionStatusActive)
		if err := subDao.Create(ctx, newSub); err != nil {
			return nil, fmt.Errorf("error al cambiar de tier")
		}
		if err := urDao.UpdateTier(ctx, userID, roleID, target.ID); err != nil {
			return nil, fmt.Errorf("error al cambiar de tier")
		}

		resp.SubscriptionID = newSub.ID
		resp.SubscriptionStatus = newSub.Status
		return resp, nil
	}

	if s.db != nil {
		var result *tiersubscription.ChangeTierResponse
		err := s.db.Transaction(func(tx *gorm.DB) error {
			res, err := apply(
				daos.NewUserRoleDao(tx),
				daos.NewRoleDao(tx),
				daos.NewTierDao(tx),
				daos.NewTierSubscriptionDao(tx),
				daos.NewInstallmentDao(tx),
			)
			if err != nil {
				return err
			}
			result = res
			return nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return apply(s.userRoleDao, s.roleDao, s.tierDao, s.tierSubDao, s.installDao)
}

// GetCurrentSubscription devuelve el estado vigente de la suscripción del
// usuario para el rol y período pedido (D9): próxima cuota a pagar y datos para
// el checkout Bricks. period = current → sub activa; period = next → sub con
// primer pago pendiente. Si no existe sub en el estado del período devuelve
// (nil, nil) → el controller responde 200 con body vacío.
func (s *tierSubscriptionService) GetCurrentSubscription(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error) {
	ur, err := s.userRoleDao.FindByUserAndRole(ctx, userID, roleID)
	if err != nil {
		customlogger.Error(ctx, "error finding user role for current subscription", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}
	if ur == nil {
		return nil, fmt.Errorf("el usuario no tiene asignado este rol")
	}

	role, err := s.roleDao.FindByID(ctx, roleID)
	if err != nil {
		customlogger.Error(ctx, "error finding role for current subscription", err,
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}
	if role == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	filterStatus := subscriptionStatusForPeriod(period)
	sub, err := s.tierSubDao.FindActiveByUserRole(ctx, userID, roleID, filterStatus)
	if err != nil {
		customlogger.Error(ctx, "error finding active subscription", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.Tag("period", period),
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}
	if sub == nil {
		return nil, nil
	}

	customlogger.Info(ctx, "GetCurrentSubscription active sub",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
		customlogger.Tag("sub_id", fmt.Sprintf("%d", sub.ID)),
		customlogger.Tag("sub_tier_id", fmt.Sprintf("%d", sub.TierID)),
		customlogger.Tag("sub_status", sub.Status),
		customlogger.Tag("paid_installments", fmt.Sprintf("%d", sub.PaidInstallments)),
		customlogger.TagMethod("GetCurrentSubscription"))

	tier, err := s.tierDao.FindByID(ctx, sub.TierID)
	if err != nil {
		customlogger.Error(ctx, "error finding subscription tier", err,
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}
	if tier == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	resp := &tiersubscription.CurrentSubscriptionResponse{
		SubscriptionID:     sub.ID,
		SubscriptionStatus: sub.Status,
		PaidInstallments:   &sub.PaidInstallments,
		Tier: tiersubscription.TierInfo{
			ID:              tier.ID,
			Name:            tier.Name,
			Hierarchy:       tier.Hierarchy,
			PaymentRequired: tier.PaymentRequired,
		},
		Role: tiersubscription.RoleInfo{ID: role.ID, Name: role.Name},
	}

	next, err := s.installDao.FindNext(ctx, sub.ID)
	if err != nil {
		customlogger.Error(ctx, "error finding next installment", err,
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}
	if next != nil {
		num := next.InstallmentNumber
		amount := next.Amount
		resp.InstallmentID = &next.ID
		resp.InstallmentNumber = &num
		resp.InstallmentAmount = &amount
		resp.NextDueDate = next.DueDate
		resp.BlockedDate = next.BlockedDate

		customlogger.Info(ctx, "GetCurrentSubscription next installment",
			customlogger.Tag("installment_id", fmt.Sprintf("%d", next.ID)),
			customlogger.Tag("installment_number", fmt.Sprintf("%d", next.InstallmentNumber)),
			customlogger.Tag("amount", fmt.Sprintf("%.2f", next.Amount)),
			customlogger.Tag("status", next.Status),
			customlogger.TagMethod("GetCurrentSubscription"))
	}

	if tier.PaymentRequired {
		resp.MercadoPago = &tiersubscription.MercadoPagoInfo{PublicKey: config.MyMP.PublicKey}
	}
	return resp, nil
}

// subscriptionStatusForPeriod mapea el período pedido en la URL al estado de
// suscripción a filtrar. current → active; next → first_payment_pending. El
// controller valida el período antes de llegar acá.
func subscriptionStatusForPeriod(period string) string {
	switch period {
	case string(constants.SubscriptionPeriodCurrent):
		return string(constants.SubscriptionStatusActive)
	case string(constants.SubscriptionPeriodNext):
		return string(constants.SubscriptionStatusFirstPaymentPending)
	default:
		return ""
	}
}

// CancelPendingSubscription cancela la suscripción con primer pago pendiente de
// la terna (user_id, role_id, tier_id): pasa la sub a canceled, cancela sus
// cuotas pendientes y libera el slot del índice único parcial para permitir un
// nuevo cambio de tier. Errores tipificados para el controller: 404 si la terna
// no tiene ninguna suscripción, 409 si la tiene pero no en first_payment_pending.
func (s *tierSubscriptionService) CancelPendingSubscription(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error) {
	customlogger.Info(ctx, "CancelPendingSubscription start",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
		customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
		customlogger.TagMethod("CancelPendingSubscription"))
	apply := func(
		subDao daos.TierSubscriptionDaoInterface,
		insDao daos.InstallmentDaoInterface,
	) (*tiersubscription.CancelSubscriptionResponse, error) {
		sub, err := subDao.FindPendingByUserRoleTier(ctx, userID, roleID, tierID)
		if err != nil {
			return nil, fmt.Errorf("error al cancelar la suscripción")
		}
		if sub == nil {
			existing, err := subDao.FindByUserRoleTier(ctx, userID, roleID, tierID)
			if err != nil {
				return nil, fmt.Errorf("error al cancelar la suscripción")
			}
			if existing == nil {
				return nil, fmt.Errorf("suscripción no encontrada")
			}
			return nil, fmt.Errorf("la suscripción no está en primer pago pendiente")
		}

		if err := subDao.SetCanceled(ctx, sub.ID); err != nil {
			return nil, fmt.Errorf("error al cancelar la suscripción")
		}
		if err := insDao.CancelPendingBySubscription(ctx, sub.ID); err != nil {
			return nil, fmt.Errorf("error al cancelar la suscripción")
		}

		customlogger.Info(ctx, "CancelPendingSubscription canceled sub",
			customlogger.Tag("sub_id", fmt.Sprintf("%d", sub.ID)),
			customlogger.Tag("sub_tier_id", fmt.Sprintf("%d", sub.TierID)),
			customlogger.Tag("new_sub_status", string(constants.SubscriptionStatusCanceled)),
			customlogger.TagMethod("CancelPendingSubscription"))

		return &tiersubscription.CancelSubscriptionResponse{
			SubscriptionID:     sub.ID,
			SubscriptionStatus: string(constants.SubscriptionStatusCanceled),
		}, nil
	}

	if s.db != nil {
		var result *tiersubscription.CancelSubscriptionResponse
		err := s.db.Transaction(func(tx *gorm.DB) error {
			res, err := apply(
				daos.NewTierSubscriptionDao(tx),
				daos.NewInstallmentDao(tx),
			)
			if err != nil {
				return err
			}
			result = res
			return nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return apply(s.tierSubDao, s.installDao)
}
