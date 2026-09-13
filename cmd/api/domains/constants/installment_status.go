package constants

// InstallmentStatus define los estados posibles de una cuota de suscripción.
// canceled es terminal y acompaña la cancelación de la suscripción con primer
// pago pendiente.
type InstallmentStatus string

const (
	InstallmentStatusPending  InstallmentStatus = "pending"
	InstallmentStatusPaid     InstallmentStatus = "paid"
	InstallmentStatusCanceled InstallmentStatus = "canceled"
)

func GetValidInstallmentStatuses() []string {
	return []string{
		string(InstallmentStatusPending),
		string(InstallmentStatusPaid),
		string(InstallmentStatusCanceled),
	}
}

func IsValidInstallmentStatus(status string) bool {
	for _, s := range GetValidInstallmentStatuses() {
		if s == status {
			return true
		}
	}
	return false
}