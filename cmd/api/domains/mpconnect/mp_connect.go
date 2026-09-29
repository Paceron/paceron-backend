package mpconnect

// AuthURLResponse devuelve la URL de autorización de Mercado Pago y el state CSRF (D7).
type AuthURLResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// CallbackRequest son los parámetros del callback OAuth (code + state + errores).
type CallbackRequest struct {
	Code             string `form:"code"`
	State            string `form:"state"`
	Error            string `form:"error"`
	ErrorDescription string `form:"error_description"`
}

// CallbackResponse es la respuesta tras procesar el callback.
type CallbackResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// StatusResponse es la respuesta de GET /api/v1/mercadopago/connect/status.
type StatusResponse struct {
	// Connected es true si la conexión está autorizada y su access token no
	// venció: es lo que hace falta para poder cobrar.
	Connected     bool   `json:"connected"`
	AccountStatus string `json:"account_status"` // authorized | deauthorized
	// TokenExpiresAt es el vencimiento del access token (RFC3339 UTC), o null si
	// no hay conexión o no se registró. Mercado Pago lo emite por 180 días y
	// nada lo renueva: pasada esa fecha hay que volver a conectar la cuenta.
	TokenExpiresAt *string `json:"token_expires_at"`
}

// DeauthWebhookRequest es el cuerpo del webhook de desautorización de Mercado
// Pago. El user_id es el MP user id del vendedor (el que queda en mp_user_id).
type DeauthWebhookRequest struct {
	UserID int64 `json:"user_id"`
}