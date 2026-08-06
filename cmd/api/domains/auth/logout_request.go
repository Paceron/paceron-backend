package auth

// LogoutRequest es el DTO para cerrar sesión revocando un refresh token.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
