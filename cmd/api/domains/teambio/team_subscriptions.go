package teambio

// TeamMemberSubscriptionInfo es el estado de cuota resumido de un corredor
// dentro de la respuesta batch del equipo.
type TeamMemberSubscriptionInfo struct {
	UserID             int64   `json:"user_id"`
	Status             string  `json:"status"`
	SubscriptionStatus string  `json:"subscription_status"`
	HasDebt            bool    `json:"has_debt"`
	InitAmount         float64 `json:"init_amount"`
	PaidInstallments   int     `json:"paid_installments"`
}

// TeamSubscriptionsResponse es la respuesta batch de
// GET /api/v1/teams/{id}/subscriptions.
type TeamSubscriptionsResponse struct {
	TeamID        int64                        `json:"team_id"`
	MembershipFee float64                      `json:"membership_fee"`
	Members       []TeamMemberSubscriptionInfo `json:"members"`
}
