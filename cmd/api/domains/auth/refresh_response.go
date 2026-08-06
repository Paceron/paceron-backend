package auth

// RefreshResponse es la respuesta al rotar un refresh token: access y refresh nuevos.
// El refresh token usado en la solicitud queda revocado.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}
