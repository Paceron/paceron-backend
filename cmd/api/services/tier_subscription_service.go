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
	GetCurrentSubscription(ctx *gin.Context, userID, roleID int64) (*tiersubscription.CurrentSubscriptionResponse, error)
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
			installment := &dbs.Installment{
				SubscriptionID:    &newSub.ID,
				UserID:            userID,
				InstallmentNumber: 1,
				Status:            string(constants.InstallmentStatusPending),
				Amount:            target.TierAmount,
			}
			if err := insDao.Create(ctx, installment); err != nil {
				return nil, fmt.Errorf("error al cambiar de tier")
			}

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
// usuario para el rol (D9): próxima cuota a pagar y datos para el checkout
// Bricks. Si el rol es gratis (sin cuota) devuelve el estado del rol/tier
// con los campos de suscripción/cuota vacíos.
func (s *tierSubscriptionService) GetCurrentSubscription(ctx *gin.Context, userID, roleID int64) (*tiersubscription.CurrentSubscriptionResponse, error) {
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

	sub, err := s.tierSubDao.FindActiveByUserRole(ctx, userID, roleID)
	if err != nil {
		customlogger.Error(ctx, "error finding active subscription", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}

	if sub != nil {
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
		}

		if tier.PaymentRequired {
			resp.MercadoPago = &tiersubscription.MercadoPagoInfo{PublicKey: config.MyMP.PublicKey}
		}
		return resp, nil
	}

	// Sin suscripción vigente (rol gratis o sub terminada): estado del rol/tier.
	tier, err := s.tierDao.FindByID(ctx, ur.TierID)
	if err != nil {
		customlogger.Error(ctx, "error finding user role tier", err,
			customlogger.TagMethod("GetCurrentSubscription"))
		return nil, fmt.Errorf("error al obtener la suscripción")
	}
	if tier == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	return &tiersubscription.CurrentSubscriptionResponse{
		Tier: tiersubscription.TierInfo{
			ID:              tier.ID,
			Name:            tier.Name,
			Hierarchy:       tier.Hierarchy,
			PaymentRequired: tier.PaymentRequired,
		},
		Role: tiersubscription.RoleInfo{ID: role.ID, Name: role.Name},
	}, nil
}