package auth

type ResetPasswordRequest struct {
	Email           string `json:"email" binding:"required"`
	Code            string `json:"code" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}
