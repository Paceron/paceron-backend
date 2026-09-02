package constants

// Custom codes usados por los controllers para tipificar errores de dominio.
// Se usan como `Code` en el DTO apierror.APIError.
const (
	ErrorCodeTierNotFound                  = "TIER_NOT_FOUND"
	ErrorCodeTierRoleMismatch              = "TIER_ROLE_MISMATCH"
	ErrorCodeSubscriptionPendingFirstPayment = "SUBSCRIPTION_PENDING_FIRST_PAYMENT"
	ErrorCodeDebtBlocksOperation           = "DEBT_BLOCKS_OPERATION"
	ErrorCodeSellerNotConnected            = "SELLER_NOT_CONNECTED"
	ErrorCodeTeamDebtBlocksOperation       = "TEAM_DEBT_BLOCKS_OPERATION"
	ErrorCodeNotAppOwner                   = "NOT_APP_OWNER"
)