package dbs

import "time"

// UserRoleTierSubscription representa el ledger de suscripciones de tier por
// usuario y rol. Una misma (user_id, role_id) solo puede tener una suscripción
// vigente (status en 'active'/'first_payment_pending') a la vez (índice único
// parcial creado con SQL crudo en la migración); el resto es historial.
type UserRoleTierSubscription struct {
	ID               int64      `gorm:"column:id;primaryKey"`
	UserID           int64      `gorm:"column:user_id;not null"`
	RoleID           int64      `gorm:"column:role_id;not null"`
	TierID           int64      `gorm:"column:tier_id;not null"`
	Status           string     `gorm:"column:status;not null"` // first_payment_pending / active / ended
	InitAmount       float64    `gorm:"column:init_amount;not null;default:0"`
	PaidInstallments int        `gorm:"column:paid_installments;not null;default:0"`
	StartDate        time.Time  `gorm:"column:start_date;not null"`
	EndedDate        *time.Time `gorm:"column:ended_date"`
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserRoleTierSubscription) TableName() string {
	return "user_role_tier_subscriptions"
}