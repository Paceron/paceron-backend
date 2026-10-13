package constants

// Custom codes usados por los controllers para tipificar errores de dominio.
// Se usan como `Code` en el DTO apierror.APIError.
const (
	ErrorCodeTierNotFound                  = "TIER_NOT_FOUND"
	ErrorCodeTierRoleMismatch              = "TIER_ROLE_MISMATCH"
	ErrorCodeSubscriptionPendingFirstPayment = "SUBSCRIPTION_PENDING_FIRST_PAYMENT"
	ErrorCodeSubscriptionNotFound            = "SUBSCRIPTION_NOT_FOUND"
	ErrorCodeSubscriptionNotPendingFirstPayment = "SUBSCRIPTION_NOT_PENDING_FIRST_PAYMENT"
	ErrorCodeDebtBlocksOperation           = "DEBT_BLOCKS_OPERATION"
	ErrorCodeSellerNotConnected            = "SELLER_NOT_CONNECTED"
	ErrorCodeTeamDebtBlocksOperation       = "TEAM_DEBT_BLOCKS_OPERATION"
	ErrorCodeNotAppOwner                   = "NOT_APP_OWNER"
	ErrorCodePaymentInstallmentNotFound    = "PAYMENT_INSTALLMENT_NOT_FOUND"
	ErrorCodePaymentInstallmentForbidden   = "PAYMENT_INSTALLMENT_FORBIDDEN"
)

// Códigos de los rechazos de parámetro del dashboard. Son estables a propósito:
// el 400 del dashboard no es reintentable —reintentar la misma request devuelve
// lo mismo—, así que el cliente necesita un `code` para no ofrecer "reintentar"
// como acción y para poder explicar qué corregir.
const (
	ErrorCodeInvalidRange       = "INVALID_RANGE"
	ErrorCodeInvalidTolerance   = "INVALID_TOLERANCE"
	ErrorCodeInvalidSort        = "INVALID_SORT"
	ErrorCodeInvalidGranularity = "INVALID_GRANULARITY"
	ErrorCodeMissingFrom        = "MISSING_FROM"
	ErrorCodeInvalidPageSize    = "INVALID_PAGE_SIZE"
)
