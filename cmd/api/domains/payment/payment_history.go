package payment

// DTOs del historial de pagos y cobros del entrenador
// (change historial-pagos-cobros-entrenador).

// PaymentTeamRef identifica al equipo de un cobro.
type PaymentTeamRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PaymentPayerRef identifica al corredor que pagó un cobro. Sale de
// installments.user_id: payments.user_id no es confiable.
type PaymentPayerRef struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
}

// ReceivedPaymentItem es un cobro de membresía de equipo. NetAmount es nil
// salvo que Mercado Pago haya informado el neto real (design D5).
type ReceivedPaymentItem struct {
	ID                int64           `json:"id"`
	MPPaymentID       string          `json:"mp_payment_id"`
	Status            string          `json:"status"`
	StatusGroup       string          `json:"status_group"`
	StatusDetail      string          `json:"status_detail"`
	GrossAmount       float64         `json:"gross_amount"`
	NetAmount         *float64        `json:"net_amount"`
	CurrencyID        string          `json:"currency_id"`
	PaymentMethodID   string          `json:"payment_method_id"`
	CreatedAt         string          `json:"created_at"`
	InstallmentID     int64           `json:"installment_id"`
	InstallmentNumber int             `json:"installment_number"`
	Team              PaymentTeamRef  `json:"team"`
	Payer             PaymentPayerRef `json:"payer"`
}

// ReceivedPaymentsResponse es la página de GET /api/v1/payments/received.
type ReceivedPaymentsResponse struct {
	Payments []ReceivedPaymentItem `json:"payments"`
	HasMore  bool                  `json:"has_more"`
}

// PaymentTierRef identifica el tier de un pago de suscripción.
type PaymentTierRef struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	RoleName string `json:"role_name"`
}

// PaymentTrainerRef identifica al entrenador que recibió un pago de membresía.
type PaymentTrainerRef struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

// HistoryPaymentItem es un pago del historial del usuario. Type es
// "subscription" (tier) o "trainer_payment" (membresía de un equipo); según el
// tipo viene Tier, o Team + Trainer, y el otro par queda en null.
type HistoryPaymentItem struct {
	ID                int64              `json:"id"`
	MPPaymentID       string             `json:"mp_payment_id"`
	Type              string             `json:"type"`
	Status            string             `json:"status"`
	StatusGroup       string             `json:"status_group"`
	StatusDetail      string             `json:"status_detail"`
	Amount            float64            `json:"amount"`
	CurrencyID        string             `json:"currency_id"`
	PaymentMethodID   string             `json:"payment_method_id"`
	CreatedAt         string             `json:"created_at"`
	InstallmentID     int64              `json:"installment_id"`
	InstallmentNumber int                `json:"installment_number"`
	DueDate           *string            `json:"due_date"`
	Tier              *PaymentTierRef    `json:"tier"`
	Team              *PaymentTeamRef    `json:"team"`
	Trainer           *PaymentTrainerRef `json:"trainer"`
}

// HistoryPaymentsResponse es la página de GET /api/v1/payments/history.
type HistoryPaymentsResponse struct {
	Payments []HistoryPaymentItem `json:"payments"`
	HasMore  bool                 `json:"has_more"`
}

// MonthlyAmount es el total cobrado en un mes (YYYY-MM, hora argentina).
type MonthlyAmount struct {
	Month         string   `json:"month"`
	GrossAmount   float64  `json:"gross_amount"`
	NetAmount     *float64 `json:"net_amount"`
	ApprovedCount int      `json:"approved_count"`
	NetKnownCount int      `json:"net_known_count"`
}

// TeamAmount es el total cobrado por un equipo dentro de la ventana.
type TeamAmount struct {
	TeamID        int64    `json:"team_id"`
	TeamName      string   `json:"team_name"`
	GrossAmount   float64  `json:"gross_amount"`
	NetAmount     *float64 `json:"net_amount"`
	ApprovedCount int      `json:"approved_count"`
	NetKnownCount int      `json:"net_known_count"`
	PendingCount  int      `json:"pending_count"`
	RejectedCount int      `json:"rejected_count"`
}

// ReceivedSummaryResponse es la respuesta de GET /api/v1/payments/received/summary.
type ReceivedSummaryResponse struct {
	CurrencyID    string          `json:"currency_id"`
	Months        int             `json:"months"`
	Monthly       []MonthlyAmount `json:"monthly"`
	ByTeam        []TeamAmount    `json:"by_team"`
	PendingCount  int             `json:"pending_count"`
	RejectedCount int             `json:"rejected_count"`
	// EarliestMonth es el primer mes con cobros del vendedor (YYYY-MM, hora
	// argentina), o null si no tiene ninguno (design D14).
	EarliestMonth *string `json:"earliest_month"`
	GeneratedAt   string  `json:"generated_at"`
}
