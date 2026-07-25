package auth

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}
