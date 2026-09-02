package constants

// PaymentConcept define los conceptos de pago que distinguen los flujos del
// ledger de pagos. order = flujo legacy de compra; subscription = cuota de
// suscripción de tier (individual); team_subscription = cuota de suscripción
// a un equipo (split, change suscripcion-teams-split).
type PaymentConcept string

const (
	PaymentConceptOrder             PaymentConcept = "order"
	PaymentConceptSubscription       PaymentConcept = "subscription"
	PaymentConceptTeamSubscription   PaymentConcept = "team_subscription"
)

func GetValidPaymentConcepts() []string {
	return []string{
		string(PaymentConceptOrder),
		string(PaymentConceptSubscription),
		string(PaymentConceptTeamSubscription),
	}
}