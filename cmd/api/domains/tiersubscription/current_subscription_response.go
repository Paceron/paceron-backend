package tiersubscription

import "time"

// TierInfo describe el tier de la suscripción (o del rol gratis).
type TierInfo struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Hierarchy       int    `json:"hierarchy"`
	PaymentRequired bool   `json:"payment_required"`
}

// RoleInfo describe el rol de la suscripción.
type RoleInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// MercadoPagoInfo trae la public_key para armar el checkout Bricks en el frontend.
type MercadoPagoInfo struct {
	PublicKey string `json:"public_key"`
}

// CurrentSubscriptionResponse es la respuesta de GET /api/v1/users/:id/subscriptions/current
// (shape D9 del design). Los campos de suscripción/cuota quedan vacíos cuando el
// rol es gratis (payment_required=false, sin cuota pendiente).
type CurrentSubscriptionResponse struct {
	SubscriptionID     int64             `json:"subscription_id,omitempty"`
	SubscriptionStatus string            `json:"subscription_status,omitempty"`
	InstallmentID      *int64            `json:"installment_id,omitempty"`
	InstallmentNumber  *int              `json:"installment_number,omitempty"`
	InstallmentAmount  *float64          `json:"installment_amount,omitempty"`
	NextDueDate        *time.Time        `json:"next_due_date"`
	BlockedDate        *time.Time        `json:"blocked_date"`
	PaidInstallments   *int              `json:"paid_installments,omitempty"`
	Tier               TierInfo          `json:"tier"`
	Role               RoleInfo          `json:"role"`
	MercadoPago        *MercadoPagoInfo  `json:"mercadopago,omitempty"`
}

// ChangeTierResponse es la respuesta de PUT tier: el estado de la nueva suscripción
// (misma shape que la próxima cuota, para que el frontend pueda seguir el flujo Bricks).
type ChangeTierResponse struct {
	CurrentSubscriptionResponse
}