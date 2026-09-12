package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// TierSubscriptionDaoInterface define las operaciones de acceso a datos para el
// ledger de suscripciones de tier por usuario/rol.
type TierSubscriptionDaoInterface interface {
	Create(ctx *gin.Context, sub *dbs.UserRoleTierSubscription) error
	FindByID(ctx *gin.Context, id int64) (*dbs.UserRoleTierSubscription, error)
	FindActiveByUserRole(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error)
	FindLatestByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error)
	FindPendingByUserRoleTier(ctx *gin.Context, userID, roleID, tierID int64) (*dbs.UserRoleTierSubscription, error)
	FindByUserRoleTier(ctx *gin.Context, userID, roleID, tierID int64) (*dbs.UserRoleTierSubscription, error)
	SetEnded(ctx *gin.Context, id int64) error
	SetCanceled(ctx *gin.Context, id int64) error
	Activate(ctx *gin.Context, id int64) error
	IncrementPaidInstallments(ctx *gin.Context, id int64) error
}

type tierSubscriptionDao struct {
	DB *gorm.DB
}

func NewTierSubscriptionDao(database *gorm.DB) TierSubscriptionDaoInterface {
	return &tierSubscriptionDao{
		DB: database,
	}
}

func (d *tierSubscriptionDao) Create(ctx *gin.Context, sub *dbs.UserRoleTierSubscription) error {
	return d.DB.Create(sub).Error
}

func (d *tierSubscriptionDao) FindByID(ctx *gin.Context, id int64) (*dbs.UserRoleTierSubscription, error) {
	var sub dbs.UserRoleTierSubscription
	err := d.DB.First(&sub, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding tier subscription by id: %w", err)
	}
	return &sub, nil
}

// FindActiveByUserRole devuelve la suscripción vigente de un usuario y rol. Con
// statuses explícitos filtra por esos estados (p.ej. solo active o solo
// first_payment_pending); sin statuses usa el default de vigentes
// (first_payment_pending | active). El índice único parcial garantiza máximo una.
func (d *tierSubscriptionDao) FindActiveByUserRole(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error) {
	if len(statuses) == 0 {
		statuses = []string{
			string(constants.SubscriptionStatusFirstPaymentPending),
			string(constants.SubscriptionStatusActive),
		}
	}
	var sub dbs.UserRoleTierSubscription
	err := d.DB.
		Where("user_id = ? AND role_id = ? AND status IN ?",
			userID,
			roleID,
			statuses,
		).
		First(&sub).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding active tier subscription: %w", err)
	}
	return &sub, nil
}

// FindLatestByUserRole devuelve la última suscripción de un usuario/rol (por id),
// sea cual sea su estado — útil como fallback y para el historial/ledger.
func (d *tierSubscriptionDao) FindLatestByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
	var sub dbs.UserRoleTierSubscription
	err := d.DB.
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Order("id DESC").
		First(&sub).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding latest tier subscription: %w", err)
	}
	return &sub, nil
}

// FindPendingByUserRoleTier devuelve la suscripción con primer pago pendiente
// de una terna (user_id, role_id, tier_id), si existe.
func (d *tierSubscriptionDao) FindPendingByUserRoleTier(ctx *gin.Context, userID, roleID, tierID int64) (*dbs.UserRoleTierSubscription, error) {
	var sub dbs.UserRoleTierSubscription
	err := d.DB.
		Where("user_id = ? AND role_id = ? AND tier_id = ? AND status = ?",
			userID,
			roleID,
			tierID,
			string(constants.SubscriptionStatusFirstPaymentPending),
		).
		First(&sub).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding pending tier subscription: %w", err)
	}
	return &sub, nil
}

// FindByUserRoleTier devuelve la última suscripción (por id) de una terna
// (user_id, role_id, tier_id), sea cual sea su estado — permite distinguir un
// 404 (terna inexistente) de un 409 (terna existente pero no pendiente) en la
// cancelación de una sub con primer pago pendiente.
func (d *tierSubscriptionDao) FindByUserRoleTier(ctx *gin.Context, userID, roleID, tierID int64) (*dbs.UserRoleTierSubscription, error) {
	var sub dbs.UserRoleTierSubscription
	err := d.DB.
		Where("user_id = ? AND role_id = ? AND tier_id = ?", userID, roleID, tierID).
		Order("id DESC").
		First(&sub).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding tier subscription by user role tier: %w", err)
	}
	return &sub, nil
}

// SetEnded cierra una suscripción (status = ended, ended_date = now).
func (d *tierSubscriptionDao) SetEnded(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.UserRoleTierSubscription{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     string(constants.SubscriptionStatusEnded),
			"ended_date": gorm.Expr("NOW()"),
		}).Error
}

// SetCanceled marca una suscripción como cancelada (status = canceled). Es un
// estado terminal; como el índice único parcial solo cubre active y
// first_payment_pending, cancelar una sub pendiente libera el slot y permite
// crear una suscripción vigente nueva para el mismo (user_id, role_id).
func (d *tierSubscriptionDao) SetCanceled(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.UserRoleTierSubscription{}).
		Where("id = ?", id).
		Update("status", string(constants.SubscriptionStatusCanceled)).Error
}

// Activate marca una suscripción como active. Se llama cuando se confirma el
// pago de la cuota #1 (D3) — el acceso al tier pago arranca recién ahí.
func (d *tierSubscriptionDao) Activate(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.UserRoleTierSubscription{}).
		Where("id = ?", id).
		Update("status", string(constants.SubscriptionStatusActive)).Error
}

// IncrementPaidInstallments incrementa el contador denormalizado de cuotas pagadas.
func (d *tierSubscriptionDao) IncrementPaidInstallments(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.UserRoleTierSubscription{}).
		Where("id = ?", id).
		UpdateColumn("paid_installments", gorm.Expr("paid_installments + 1")).Error
}