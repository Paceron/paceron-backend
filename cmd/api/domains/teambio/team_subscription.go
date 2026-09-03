package teambio

import "time"

// TeamInfo describe el equipo en el estado de cuenta (D3).
type TeamInfo struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	MembershipFee float64 `json:"membership_fee"`
}

// MembershipInfo describe la membresía (suscripción) del corredor al equipo.
type MembershipInfo struct {
	SubscriptionStatus string     `json:"subscription_status"`
	InitAmount         float64    `json:"init_amount"`
	PaidInstallments   int        `json:"paid_installments"`
	StartDate          *time.Time `json:"start_date"`
}

// NextInstallmentInfo es la próxima cuota a pagar de la membresía.
type NextInstallmentInfo struct {
	InstallmentID     int64      `json:"installment_id"`
	InstallmentNumber int        `json:"installment_number"`
	InstallmentAmount float64    `json:"installment_amount"`
	NextDueDate       *time.Time `json:"next_due_date"`
	BlockedDate       *time.Time `json:"blocked_date"`
}

// MercadoPagoCheckout trae los datos para armar el checkout Bricks marketplace.
type MercadoPagoCheckout struct {
	PublicKey  string `json:"public_key"`
	Concept    string `json:"concept"`
	Marketplace bool   `json:"marketplace"`
}

// TeamSubscriptionResponse es la respuesta de GET /api/v1/users/:id/teams/:team_id/subscription
// (shape D3 del design). Si el equipo es gratis devuelve subscription_status active
// sin cuota pendiente ni datos de checkout.
type TeamSubscriptionResponse struct {
	Team         TeamInfo              `json:"team"`
	Membership   MembershipInfo        `json:"membership"`
	NextInstallment *NextInstallmentInfo `json:"next_installment,omitempty"`
	HasDebt      bool                  `json:"has_debt"`
	MercadoPago  *MercadoPagoCheckout  `json:"mercadopago,omitempty"`
}