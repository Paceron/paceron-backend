package tiersubscription

// CancelSubscriptionResponse es la respuesta de
// DELETE /api/v1/users/:id/roles/:role_id/subscriptions/pending: confirma la
// suscripción cancelada y su estado terminal.
type CancelSubscriptionResponse struct {
	SubscriptionID     int64  `json:"subscription_id"`
	SubscriptionStatus string `json:"subscription_status"`
}