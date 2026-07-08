package auth

type AuthorizationData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type LoginResponse struct {
	Authorization AuthorizationData `json:"authorization"`
	User          RegisterResponse  `json:"user"`
}
