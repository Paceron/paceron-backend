package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// InstallmentDaoInterface define las operaciones de acceso a datos para las
// cuotas de suscripciones (tabla compartida tier/equipo, arco exclusivo).
type InstallmentDaoInterface interface {
	Create(ctx *gin.Context, installment *dbs.Installment) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Installment, error)
	MarkPaidConditional(ctx *gin.Context, id int64, internalPaymentID *int64, externalPaymentID *string) (bool, error)
	FindPendingBySubscription(ctx *gin.Context, subscriptionID int64) ([]dbs.Installment, error)
	FindNext(ctx *gin.Context, subscriptionID int64) (*dbs.Installment, error)
	FindPendingByUserTeam(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error)
}

type installmentDao struct {
	DB *gorm.DB
}

func NewInstallmentDao(database *gorm.DB) InstallmentDaoInterface {
	return &installmentDao{
		DB: database,
	}
}

func (d *installmentDao) Create(ctx *gin.Context, installment *dbs.Installment) error {
	return d.DB.Create(installment).Error
}

func (d *installmentDao) FindByID(ctx *gin.Context, id int64) (*dbs.Installment, error) {
	var installment dbs.Installment
	err := d.DB.First(&installment, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding installment by id: %w", err)
	}
	return &installment, nil
}

// MarkPaidConditional marca la cuota como `paid` solo si sigue en `pending`.
// Devuelve true si el update afectó una fila; false si la cuota ya estaba pagada
// (doble notificación del webhook → operación sin efectos). De paso setea los
// payment ids (interno y externo) que llegan del pago confirmado.
func (d *installmentDao) MarkPaidConditional(ctx *gin.Context, id int64, internalPaymentID *int64, externalPaymentID *string) (bool, error) {
	result := d.DB.Model(&dbs.Installment{}).
		Where("id = ? AND status = ?", id, string(constants.InstallmentStatusPending)).
		Updates(map[string]interface{}{
			"status":               string(constants.InstallmentStatusPaid),
			"internal_payment_id":  internalPaymentID,
			"external_payment_id":  externalPaymentID,
		})
	if result.Error != nil {
		return false, fmt.Errorf("error marking installment paid: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// FindPendingBySubscription devuelve todas las cuotas pendientes de una
// suscripción — se usa para calcular deuda (cualquiera con blocked_date vencido).
func (d *installmentDao) FindPendingBySubscription(ctx *gin.Context, subscriptionID int64) ([]dbs.Installment, error) {
	var installments []dbs.Installment
	err := d.DB.
		Where("subscription_id = ? AND status = ?", subscriptionID, string(constants.InstallmentStatusPending)).
		Order("installment_number ASC").
		Find(&installments).Error
	if err != nil {
		return nil, fmt.Errorf("error finding pending installments by subscription: %w", err)
	}
	return installments, nil
}

// FindNext devuelve la próxima cuota a pagar de una suscripción (la pendiente de
// menor número). El índice del arco + la unicidad de la pendiente viva la hacen
// determinística en el flujo normal (una sola cuota pendiente por suscripción).
func (d *installmentDao) FindNext(ctx *gin.Context, subscriptionID int64) (*dbs.Installment, error) {
	var installment dbs.Installment
	err := d.DB.
		Where("subscription_id = ? AND status = ?", subscriptionID, string(constants.InstallmentStatusPending)).
		Order("installment_number ASC").
		First(&installment).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding next installment: %w", err)
	}
	return &installment, nil
}

// FindPendingByUserTeam devuelve las cuotas pendientes de la membresía de un
// usuario a un equipo (change suscripcion-teams-split).
func (d *installmentDao) FindPendingByUserTeam(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error) {
	var installments []dbs.Installment
	err := d.DB.
		Where("team_id = ? AND user_id = ? AND status = ?", teamID, userID, string(constants.InstallmentStatusPending)).
		Order("installment_number ASC").
		Find(&installments).Error
	if err != nil {
		return nil, fmt.Errorf("error finding pending installments by user team: %w", err)
	}
	return installments, nil
}