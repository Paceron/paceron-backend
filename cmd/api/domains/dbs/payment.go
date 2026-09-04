package dbs

import "time"

type Payment struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	UserID          *int64    `gorm:"column:user_id"`
	PreferenceID    string    `gorm:"column:preference_id"`
	PaymentID       string    `gorm:"column:payment_id;index"`
	ExternalRef     *string   `gorm:"column:external_reference;uniqueIndex"`
	Concept         string    `gorm:"column:concept;not null"`
	Description     string    `gorm:"column:description"`
	Amount          float64   `gorm:"column:amount;not null"`
	CurrencyID      string    `gorm:"column:currency_id;not null;default:ARS"`
	Status          string    `gorm:"column:status;not null;default:pending"`
	StatusDetail    string    `gorm:"column:status_detail"`
	PaymentMethodID string    `gorm:"column:payment_method_id"`
	Installments    int       `gorm:"column:installments"`
	PayerEmail      string    `gorm:"column:payer_email"`
	MarketplaceFee  *float64  `gorm:"column:marketplace_fee"`
	SellerUserID    *int64    `gorm:"column:seller_user_id"`
	InstallmentID   *int64    `gorm:"column:installment_id"` // FK -> installments.id (pago de cuota)
	RawResponse     *string   `gorm:"column:raw_response;type:jsonb"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
