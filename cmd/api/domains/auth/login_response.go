package auth

// LoginResponse es la respuesta de login. Los tokens van planos (no anidados bajo
// "authorization" como antes) — cambio de contrato, ver docs/AUTH_MIGRATION.md.
type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         UserResponse `json:"user"`
}
