package dbs

import "time"

// Installment representa una cuota de una suscripción. La tabla es compartida
// entre suscripciones de tier (subscription_id) y de equipo (team_id), con un
// CHECK de arco exclusivo: exactamente uno de los dos padres debe estar seteado.
// La columna team_id queda definida desde ahora para no migrar la tabla cuando
// entre en vigencia el flujo de equipos (change suscripcion-teams-split).
type Installment struct {
	ID                int64      `gorm:"column:id;primaryKey"`
	SubscriptionID    *int64     `gorm:"column:subscription_id"`
	TeamID            *int64     `gorm:"column:team_id"`
	UserID            int64      `gorm:"column:user_id;not null"`
	InstallmentNumber int        `gorm:"column:installment_number;not null"` // arranca en 1
	Status            string     `gorm:"column:status;not null"`             // pending / paid
	InternalPaymentID *int64     `gorm:"column:internal_payment_id"`         // FK -> payments.id
	ExternalPaymentID *string    `gorm:"column:external_payment_id"`         // payment_id de Mercado Pago
	Amount            float64    `gorm:"column:amount;not null"`
	DueDate           *time.Time `gorm:"column:due_date"`     // nulo en cuota #1
	BlockedDate       *time.Time `gorm:"column:blocked_date"` // nulo en cuota #1; cutoff de gracia
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Installment) TableName() string {
	return "installments"
}