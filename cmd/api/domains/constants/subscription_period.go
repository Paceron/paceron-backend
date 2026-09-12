package constants

// SubscriptionPeriod define los valores soportados del path param :period del
// endpoint GET /api/v1/users/:id/subscriptions/:period.
type SubscriptionPeriod string

const (
	// SubscriptionPeriodCurrent apunta a la suscripción vigente activa.
	SubscriptionPeriodCurrent SubscriptionPeriod = "current"
	// SubscriptionPeriodNext apunta a la suscripción con primer pago pendiente.
	SubscriptionPeriodNext SubscriptionPeriod = "next"
)

func GetValidSubscriptionPeriods() []string {
	return []string{
		string(SubscriptionPeriodCurrent),
		string(SubscriptionPeriodNext),
	}
}

func IsValidSubscriptionPeriod(period string) bool {
	for _, p := range GetValidSubscriptionPeriods() {
		if p == period {
			return true
		}
	}
	return false
}