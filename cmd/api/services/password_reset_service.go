package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

const (
	otpExpiryDuration = 10 * time.Minute
	otpMaxAttempts    = 5
	otpCodeLength     = 6
)

type PasswordResetServiceInterface interface {
	RequestPasswordReset(ctx *gin.Context, email string) error
	ResetPassword(ctx *gin.Context, email, code, newPassword string) error
}

type passwordResetService struct {
	authDao          daos.AuthDaoInterface
	userDao          daos.UserDaoInterface
	passwordResetDao daos.PasswordResetDaoInterface
	mailer           mailer.MailerInterface
}

func NewPasswordResetService(
	authDao daos.AuthDaoInterface,
	userDao daos.UserDaoInterface,
	passwordResetDao daos.PasswordResetDaoInterface,
	mailerClient mailer.MailerInterface,
) PasswordResetServiceInterface {
	return &passwordResetService{
		authDao:          authDao,
		userDao:          userDao,
		passwordResetDao: passwordResetDao,
		mailer:           mailerClient,
	}
}

// RequestPasswordReset genera y envía un código de recuperación si el email corresponde
// a un usuario activo. Devuelve nil en todos los casos "esperables" (usuario inexistente o
// no activo) a propósito — el controller siempre responde el mismo mensaje genérico, para
// no filtrar si un email está registrado.
func (s *passwordResetService) RequestPasswordReset(ctx *gin.Context, email string) error {
	userDB, err := s.authDao.FindByEmail(ctx, email)
	if err != nil {
		customlogger.Error(ctx, "error finding user for password reset request", err,
			customlogger.Tag("step", "forgot_password_find_user"))
		return fmt.Errorf("error al procesar la solicitud")
	}
	if userDB == nil {
		customlogger.Warn(ctx, "password reset requested for non-existent email",
			customlogger.Tag("step", "forgot_password_user_not_found"))
		return nil
	}
	if userDB.Status != string(constants.UserStatusActive) {
		customlogger.Warn(ctx, "password reset requested for non-active user",
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("status", userDB.Status),
			customlogger.Tag("step", "forgot_password_inactive_user"))
		return nil
	}

	if err := s.passwordResetDao.SoftDeleteByUserID(ctx, userDB.ID); err != nil {
		customlogger.Error(ctx, "error invalidating previous reset codes", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "forgot_password_invalidate_previous"))
		return fmt.Errorf("error al procesar la solicitud")
	}

	code, err := generateOTPCode(otpCodeLength)
	if err != nil {
		customlogger.Error(ctx, "error generating OTP code", err,
			customlogger.Tag("step", "forgot_password_generate_code"))
		return fmt.Errorf("error al procesar la solicitud")
	}

	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		customlogger.Error(ctx, "error hashing OTP code", err,
			customlogger.Tag("step", "forgot_password_hash_code"))
		return fmt.Errorf("error al procesar la solicitud")
	}

	token := &dbs.PasswordResetToken{
		UserID:    userDB.ID,
		CodeHash:  string(codeHash),
		ExpiresAt: time.Now().Add(otpExpiryDuration),
	}
	if err := s.passwordResetDao.Create(ctx, token); err != nil {
		customlogger.Error(ctx, "error creating password reset token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "forgot_password_create_token"))
		return fmt.Errorf("error al procesar la solicitud")
	}

	customlogger.Info(ctx, "password reset code issued",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.TagMethod("RequestPasswordReset"))

	if s.mailer != nil {
		if err := s.mailer.SendPasswordResetEmail(ctx, userDB.Email, userDB.Name, code); err != nil {
			customlogger.Error(ctx, "error sending password reset email", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
				customlogger.Tag("step", "forgot_password_send_email"))
		}
	}

	return nil
}

// ResetPassword valida el código OTP y, si es correcto, actualiza la contraseña del usuario.
// Cualquier condición de "código inválido" (usuario inexistente/no activo, sin código activo,
// código incorrecto, código expirado, intentos agotados) devuelve el mismo error genérico
// para no convertir el endpoint en un oráculo de enumeración. La validación de que
// newPassword sea una contraseña fuerte y coincida con su confirmación se hace en el
// controller (mismo patrón que Register), acá solo se valida el código y se persiste.
func (s *passwordResetService) ResetPassword(ctx *gin.Context, email, code, newPassword string) error {
	genericErr := fmt.Errorf("código inválido o expirado")

	userDB, err := s.authDao.FindByEmail(ctx, email)
	if err != nil {
		customlogger.Error(ctx, "error finding user for password reset", err,
			customlogger.Tag("step", "reset_password_find_user"))
		return fmt.Errorf("error al restablecer la contraseña")
	}
	if userDB == nil || userDB.Status != string(constants.UserStatusActive) {
		customlogger.Warn(ctx, "password reset attempt for invalid user",
			customlogger.Tag("step", "reset_password_invalid_user"))
		return genericErr
	}

	tokenDB, err := s.passwordResetDao.FindActiveByUserID(ctx, userDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error finding active reset token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "reset_password_find_token"))
		return fmt.Errorf("error al restablecer la contraseña")
	}
	if tokenDB == nil || time.Now().After(tokenDB.ExpiresAt) {
		customlogger.Warn(ctx, "password reset attempt with no active or expired token",
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "reset_password_no_active_token"))
		return genericErr
	}

	if err := bcrypt.CompareHashAndPassword([]byte(tokenDB.CodeHash), []byte(code)); err != nil {
		newAttempts := tokenDB.Attempts + 1
		if incErr := s.passwordResetDao.IncrementAttempts(ctx, tokenDB.ID); incErr != nil {
			customlogger.Error(ctx, "error incrementing reset token attempts", incErr,
				customlogger.Tag("token_id", fmt.Sprintf("%d", tokenDB.ID)),
				customlogger.Tag("step", "reset_password_increment_attempts"))
		}
		if newAttempts >= otpMaxAttempts {
			if delErr := s.passwordResetDao.SoftDeleteByUserID(ctx, userDB.ID); delErr != nil {
				customlogger.Error(ctx, "error invalidating reset token after max attempts", delErr,
					customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
					customlogger.Tag("step", "reset_password_lockout"))
			}
		}
		customlogger.Warn(ctx, "password reset attempt with wrong code",
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "reset_password_wrong_code"))
		return genericErr
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		customlogger.Error(ctx, "error hashing new password", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "reset_password_hash_new_password"))
		return fmt.Errorf("error al restablecer la contraseña")
	}
	userDB.Password = string(hashedPassword)

	if err := s.userDao.Update(ctx, userDB); err != nil {
		customlogger.Error(ctx, "error updating user password", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "reset_password_update_user"))
		return fmt.Errorf("error al restablecer la contraseña")
	}

	if err := s.passwordResetDao.MarkUsed(ctx, tokenDB.ID); err != nil {
		customlogger.Error(ctx, "error marking reset token as used", err,
			customlogger.Tag("token_id", fmt.Sprintf("%d", tokenDB.ID)),
			customlogger.Tag("step", "reset_password_mark_used"))
	}

	customlogger.Info(ctx, "password reset successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.TagMethod("ResetPassword"))

	return nil
}

func generateOTPCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}
