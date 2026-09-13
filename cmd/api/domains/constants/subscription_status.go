package constants

// SubscriptionStatus define los estados posibles de una suscripción de tier.
// El acceso al tier pago se habilita recién cuando la cuota #1 queda pagada
// (first_payment_pending -> active). ended cierra el ledger en un cambio de
// tier; canceled es terminal y se usa cuando se aborta una suscripción con el
// primer pago pendiente (libera el slot del índice único parcial).
type SubscriptionStatus string

const (
	SubscriptionStatusFirstPaymentPending SubscriptionStatus = "first_payment_pending"
	SubscriptionStatusActive              SubscriptionStatus = "active"
	SubscriptionStatusEnded               SubscriptionStatus = "ended"
	SubscriptionStatusCanceled            SubscriptionStatus = "canceled"
)

func GetValidSubscriptionStatuses() []string {
	return []string{
		string(SubscriptionStatusFirstPaymentPending),
		string(SubscriptionStatusActive),
		string(SubscriptionStatusEnded),
		string(SubscriptionStatusCanceled),
	}
}

func IsValidSubscriptionStatus(status string) bool {
	for _, s := range GetValidSubscriptionStatuses() {
		if s == status {
			return true
		}
	}
	return false
}