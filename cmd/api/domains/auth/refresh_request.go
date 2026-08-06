package auth

// RefreshRequest es el DTO para pedir un access token nuevo a partir de un refresh token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
