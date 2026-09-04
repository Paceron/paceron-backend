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
	Connected     bool   `json:"connected"`
	AccountStatus string `json:"account_status"` // authorized | deauthorized
}

// DeauthWebhookRequest es el cuerpo del webhook de desautorización de Mercado
// Pago. El user_id es el MP user id del vendedor (el que queda en mp_user_id).
type DeauthWebhookRequest struct {
	UserID int64 `json:"user_id"`
}