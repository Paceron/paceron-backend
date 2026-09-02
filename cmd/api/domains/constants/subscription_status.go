package constants

// SubscriptionStatus define los estados posibles de una suscripción de tier.
// El acceso al tier pago se habilita recién cuando la cuota #1 queda pagada
// (first_payment_pending -> active). ended cierra el ledger en un cambio de tier.
type SubscriptionStatus string

const (
	SubscriptionStatusFirstPaymentPending SubscriptionStatus = "first_payment_pending"
	SubscriptionStatusActive              SubscriptionStatus = "active"
	SubscriptionStatusEnded               SubscriptionStatus = "ended"
)

func GetValidSubscriptionStatuses() []string {
	return []string{
		string(SubscriptionStatusFirstPaymentPending),
		string(SubscriptionStatusActive),
		string(SubscriptionStatusEnded),
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