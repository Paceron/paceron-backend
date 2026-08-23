package pushtoken

// RegisterPushTokenRequest es el DTO para registrar/actualizar el token de push de un
// dispositivo. El user_id sale del token de sesión (self-only), no del body.
type RegisterPushTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}
