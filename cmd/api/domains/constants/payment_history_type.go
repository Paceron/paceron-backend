package constants

// PaymentHistoryType distingue los pagos del historial de un usuario: lo que le
// paga a Paceron por su suscripción de tier, o lo que le paga a un entrenador
// por pertenecer a su equipo. Se deriva de la cuota (subscription_id o team_id),
// no del concept del pago, que en los pagos de tier suele quedar como "order".
type PaymentHistoryType string

const (
	PaymentHistoryTypeSubscription   PaymentHistoryType = "subscription"
	PaymentHistoryTypeTrainerPayment PaymentHistoryType = "trainer_payment"
)

// IsValidPaymentHistoryType valida el filtro por tipo del historial.
func IsValidPaymentHistoryType(t string) bool {
	return t == string(PaymentHistoryTypeSubscription) || t == string(PaymentHistoryTypeTrainerPayment)
}
