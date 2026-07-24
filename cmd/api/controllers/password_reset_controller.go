package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/auth"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type PasswordResetController interface {
	ForgotPassword(c *gin.Context)
	ResetPassword(c *gin.Context)
}

type passwordResetController struct {
	passwordResetService services.PasswordResetServiceInterface
}

func NewPasswordResetController(passwordResetService services.PasswordResetServiceInterface) PasswordResetController {
	return &passwordResetController{
		passwordResetService: passwordResetService,
	}
}

const forgotPasswordGenericMessage = "Si el email está registrado, recibirás un código de recuperación"

// ForgotPassword godoc
// @Summary      Request a password reset code
// @Description  Sends a 6-digit OTP code by email if it belongs to an active user. Always responds with the same generic message, to avoid leaking whether an email is registered.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      auth.ForgotPasswordRequest  true  "Email to send the reset code to"
// @Success      200   {object}  auth.ForgotPasswordResponse
// @Failure      400   {object}  apierror.APIError
// @Router       /api/v1/auth/forgot-password [post]
func (prc *passwordResetController) ForgotPassword(c *gin.Context) {
	var req auth.ForgotPasswordRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if err := prc.passwordResetService.RequestPasswordReset(c, req.Email); err != nil {
		customlogger.Error(c, "error requesting password reset", err,
			customlogger.Tag("step", "forgot_password"))
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    "Error interno del servidor",
		})
		return
	}

	c.JSON(http.StatusOK, auth.ForgotPasswordResponse{Message: forgotPasswordGenericMessage})
}

// ResetPassword godoc
// @Summary      Reset password using an OTP code
// @Description  Validates the OTP code sent via forgot-password and updates the user's password.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      auth.ResetPasswordRequest  true  "Email, code, new password and confirmation"
// @Success      200   {object}  auth.ResetPasswordResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/auth/reset-password [post]
func (prc *passwordResetController) ResetPassword(c *gin.Context) {
	var req auth.ResetPasswordRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		customlogger.Warn(c, "password confirmation mismatch",
			customlogger.Tag("field", "confirm_password"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "las contraseñas no coinciden",
		})
		return
	}

	if msg := services.ValidatePassword(req.NewPassword); msg != "" {
		customlogger.Warn(c, "password validation failed",
			customlogger.Tag("field", "new_password"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    msg,
		})
		return
	}

	err := prc.passwordResetService.ResetPassword(c, req.Email, req.Code, req.NewPassword)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "código inválido o expirado" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, auth.ResetPasswordResponse{Message: "Contraseña actualizada correctamente"})
}
