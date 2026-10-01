package constants

// PaymentStatus define los estados de pago que devuelve Mercado Pago. Los
// valores son los que MP escribe en `status`; el backend los guarda tal cual.
type PaymentStatus string

const (
	PaymentStatusApproved    PaymentStatus = "approved"
	PaymentStatusPending     PaymentStatus = "pending"
	PaymentStatusInProcess   PaymentStatus = "in_process"
	PaymentStatusAuthorized  PaymentStatus = "authorized"
	PaymentStatusRejected    PaymentStatus = "rejected"
	PaymentStatusCancelled   PaymentStatus = "cancelled"
	PaymentStatusRefunded    PaymentStatus = "refunded"
	PaymentStatusChargedBack PaymentStatus = "charged_back"
)

// PaymentStatusGroup agrupa los estados de MP por lo que significan para el
// dueño del dinero. Solo approved suma como cobrado.
type PaymentStatusGroup string

const (
	PaymentStatusGroupApproved PaymentStatusGroup = "approved"
	PaymentStatusGroupPending  PaymentStatusGroup = "pending"
	PaymentStatusGroupRejected PaymentStatusGroup = "rejected"
	PaymentStatusGroupRefunded PaymentStatusGroup = "refunded"
	// PaymentStatusGroupOther cubre cualquier estado que MP agregue y todavía
	// no esté mapeado. No se acepta como filtro.
	PaymentStatusGroupOther PaymentStatusGroup = "other"
)

var paymentStatusesByGroup = map[PaymentStatusGroup][]string{
	PaymentStatusGroupApproved: {string(PaymentStatusApproved)},
	PaymentStatusGroupPending:  {string(PaymentStatusPending), string(PaymentStatusInProcess), string(PaymentStatusAuthorized)},
	PaymentStatusGroupRejected: {string(PaymentStatusRejected), string(PaymentStatusCancelled)},
	PaymentStatusGroupRefunded: {string(PaymentStatusRefunded), string(PaymentStatusChargedBack)},
}

// GroupOfPaymentStatus devuelve el grupo de un estado de MP, u "other" si no
// se reconoce.
func GroupOfPaymentStatus(status string) PaymentStatusGroup {
	for group, statuses := range paymentStatusesByGroup {
		for _, s := range statuses {
			if s == status {
				return group
			}
		}
	}
	return PaymentStatusGroupOther
}

// StatusesForGroup devuelve los estados de MP que forman un grupo. El segundo
// valor es false si el grupo no existe (incluido "other").
func StatusesForGroup(group string) ([]string, bool) {
	statuses, ok := paymentStatusesByGroup[PaymentStatusGroup(group)]
	return statuses, ok
}
