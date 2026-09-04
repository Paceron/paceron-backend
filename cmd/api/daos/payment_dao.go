package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type PaymentDaoInterface interface {
	Create(ctx *gin.Context, payment *dbs.Payment) error
	UpdateStatus(ctx *gin.Context, paymentID int64, status, statusDetail string) error
	UpdatePaymentID(ctx *gin.Context, paymentID int64, mpPaymentID string) error
	UpdateRawResponse(ctx *gin.Context, paymentID int64, rawResponse string) error
	UpdateExternalRef(ctx *gin.Context, paymentID int64, externalRef string) error
	FindByID(ctx *gin.Context, id int64) (*dbs.Payment, error)
	FindByPaymentID(ctx *gin.Context, mpPaymentID string) (*dbs.Payment, error)
	FindByExternalReference(ctx *gin.Context, externalRef string) (*dbs.Payment, error)
}

type paymentDao struct {
	DB *gorm.DB
}

func NewPaymentDao(database *gorm.DB) PaymentDaoInterface {
	return &paymentDao{
		DB: database,
	}
}

func (d *paymentDao) Create(ctx *gin.Context, payment *dbs.Payment) error {
	if err := d.DB.Create(payment).Error; err != nil {
		return fmt.Errorf("error creating payment: %w", err)
	}
	return nil
}

func (d *paymentDao) UpdateStatus(ctx *gin.Context, paymentID int64, status, statusDetail string) error {
	result := d.DB.Model(&dbs.Payment{}).Where("id = ?", paymentID).Updates(map[string]interface{}{
		"status":        status,
		"status_detail": statusDetail,
	})
	if result.Error != nil {
		return fmt.Errorf("error updating payment status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("payment not found")
	}
	return nil
}

func (d *paymentDao) UpdatePaymentID(ctx *gin.Context, paymentID int64, mpPaymentID string) error {
	result := d.DB.Model(&dbs.Payment{}).Where("id = ?", paymentID).Update("payment_id", mpPaymentID)
	if result.Error != nil {
		return fmt.Errorf("error updating payment id: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("payment not found")
	}
	return nil
}

func (d *paymentDao) UpdateRawResponse(ctx *gin.Context, paymentID int64, rawResponse string) error {
	result := d.DB.Model(&dbs.Payment{}).Where("id = ?", paymentID).Update("raw_response", rawResponse)
	if result.Error != nil {
		return fmt.Errorf("error updating payment raw response: %w", result.Error)
	}
	return nil
}

func (d *paymentDao) UpdateExternalRef(ctx *gin.Context, paymentID int64, externalRef string) error {
	result := d.DB.Model(&dbs.Payment{}).Where("id = ?", paymentID).Update("external_reference", externalRef)
	if result.Error != nil {
		return fmt.Errorf("error updating payment external ref: %w", result.Error)
	}
	return nil
}

func (d *paymentDao) FindByID(ctx *gin.Context, id int64) (*dbs.Payment, error) {
	var payment dbs.Payment
	err := d.DB.First(&payment, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding payment by id: %w", err)
	}
	return &payment, nil
}

func (d *paymentDao) FindByPaymentID(ctx *gin.Context, mpPaymentID string) (*dbs.Payment, error) {
	var payment dbs.Payment
	err := d.DB.Where("payment_id = ?", mpPaymentID).First(&payment).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding payment by MP payment ID: %w", err)
	}
	return &payment, nil
}

func (d *paymentDao) FindByExternalReference(ctx *gin.Context, externalRef string) (*dbs.Payment, error) {
	var payment dbs.Payment
	err := d.DB.Where("external_reference = ?", externalRef).First(&payment).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding payment by external reference: %w", err)
	}
	return &payment, nil
}
