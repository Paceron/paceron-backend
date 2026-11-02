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

// ReceivedPaymentItem es un cobro de membresía de equipo. NetAmount es el bruto
// menos la comisión de Paceron, y es nil solo si el cobro no está aprobado
// (change cobros-pendientes-y-vencimientos, design D2).
type ReceivedPaymentItem struct {
	ID                int64           `json:"id"`
	MPPaymentID       string          `json:"mp_payment_id"`
	Status            string          `json:"status"`
	StatusGroup       string          `json:"status_group"`
	StatusDetail      string          `json:"status_detail"`
	GrossAmount       float64         `json:"gross_amount"`
	NetAmount         *float64        `json:"net_amount"`
	FeeAmount         float64         `json:"fee_amount"`
	FeePercent        float64         `json:"fee_percent"`
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

// MonthlyAmount es el total cobrado en un mes (YYYY-MM, hora argentina). Los
// Pending* son las obligaciones sin pagar que vencen en ese mes (change
// cobros-pendientes-y-vencimientos, design D5).
type MonthlyAmount struct {
	Month            string   `json:"month"`
	GrossAmount      float64  `json:"gross_amount"`
	NetAmount        *float64 `json:"net_amount"`
	ApprovedCount    int      `json:"approved_count"`
	NetKnownCount    int      `json:"net_known_count"`
	PendingAmount    float64  `json:"pending_amount"`
	PendingNetAmount float64  `json:"pending_net_amount"`
	PendingCount     int      `json:"pending_count"`
}

// ObligationItem es una cuota esperada que todavía no se pagó: no existe como
// fila, se calcula a partir de la membresía (design D3).
type ObligationItem struct {
	Team              PaymentTeamRef  `json:"team"`
	Payer             PaymentPayerRef `json:"payer"`
	InstallmentNumber int             `json:"installment_number"`
	Amount            float64         `json:"amount"`
	DueDate           string          `json:"due_date"`
}

// ObligationsBlock agrupa obligaciones sin pagar del mes actual.
type ObligationsBlock struct {
	Count     int              `json:"count"`
	Amount    float64          `json:"amount"`
	NetAmount float64          `json:"net_amount"`
	Items     []ObligationItem `json:"items"`
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
	// DueThisMonth vence desde hoy hasta fin del mes actual; OverdueThisMonth
	// venció desde el 1° del mes hasta ayer (design D4 y D5). No dependen de
	// la ventana de meses.
	DueThisMonth     ObligationsBlock `json:"due_this_month"`
	OverdueThisMonth ObligationsBlock `json:"overdue_this_month"`
	GeneratedAt      string           `json:"generated_at"`
}

// UpcomingPaymentItem es la próxima cuota sin pagar de una membresía de equipo
// (Type "trainer_payment", con Team y Trainer) o de una suscripción de tier
// (Type "subscription", con Tier). OverdueCount cuenta las cuotas sin pagar
// que ya vencieron (design D6).
type UpcomingPaymentItem struct {
	Type              string             `json:"type"`
	Team              *PaymentTeamRef    `json:"team"`
	Trainer           *PaymentTrainerRef `json:"trainer"`
	Tier              *PaymentTierRef    `json:"tier"`
	InstallmentNumber int                `json:"installment_number"`
	Amount            float64            `json:"amount"`
	CurrencyID        string             `json:"currency_id"`
	DueDate           string             `json:"due_date"`
	Overdue           bool               `json:"overdue"`
	OverdueCount      int                `json:"overdue_count"`
}

// UpcomingPaymentsResponse es la respuesta de GET /api/v1/payments/upcoming.
type UpcomingPaymentsResponse struct {
	Items []UpcomingPaymentItem `json:"items"`
}
