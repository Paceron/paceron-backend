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
	FindActiveByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error)
	FindLatestByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error)
	SetEnded(ctx *gin.Context, id int64) error
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

// FindActiveByUserRole devuelve la suscripción vigente (first_payment_pending o
// active) de un usuario y rol. El índice único parcial garantiza máximo una.
func (d *tierSubscriptionDao) FindActiveByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
	var sub dbs.UserRoleTierSubscription
	err := d.DB.
		Where("user_id = ? AND role_id = ? AND status IN ?",
			userID,
			roleID,
			[]string{string(constants.SubscriptionStatusFirstPaymentPending), string(constants.SubscriptionStatusActive)},
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

// SetEnded cierra una suscripción (status = ended, ended_date = now).
func (d *tierSubscriptionDao) SetEnded(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.UserRoleTierSubscription{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     string(constants.SubscriptionStatusEnded),
			"ended_date": gorm.Expr("NOW()"),
		}).Error
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