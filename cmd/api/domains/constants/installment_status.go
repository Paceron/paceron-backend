package constants

// InstallmentStatus define los estados posibles de una cuota de suscripción.
type InstallmentStatus string

const (
	InstallmentStatusPending InstallmentStatus = "pending"
	InstallmentStatusPaid    InstallmentStatus = "paid"
)

func GetValidInstallmentStatuses() []string {
	return []string{
		string(InstallmentStatusPending),
		string(InstallmentStatusPaid),
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