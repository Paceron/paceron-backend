package mailer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
)

// TestRenderWelcomeEmail_NoSMTPRequired verifica el renderizado del template sin
// necesitar credenciales SMTP — corre siempre, en cualquier entorno.
func TestRenderWelcomeEmail_NoSMTPRequired(t *testing.T) {
	html, err := RenderWelcomeEmail(WelcomeEmailData{Name: "Maria"})
	require.NoError(t, err)
	assert.Contains(t, html, "Maria")
	assert.Contains(t, html, "Paceron")
	assert.Contains(t, html, "#8cc63e")
}

// TestRenderPasswordResetEmail_NoSMTPRequired verifica el renderizado del template de
// recuperación de contraseña sin necesitar credenciales SMTP — corre siempre.
func TestRenderPasswordResetEmail_NoSMTPRequired(t *testing.T) {
	html, err := RenderPasswordResetEmail(PasswordResetEmailData{Name: "Maria", Code: "123456"})
	require.NoError(t, err)
	assert.Contains(t, html, "Maria")
	assert.Contains(t, html, "123456")
	assert.Contains(t, html, "Paceron")
	assert.Contains(t, html, "#8cc63e")
}

// TestSend_RealEmail_Integration envía un correo de bienvenida real usando las
// credenciales SMTP configuradas en el entorno. Se skipea automáticamente si
// las variables de entorno SMTP no están seteadas, para que `go test ./...`
// siga funcionando en máquinas sin credenciales configuradas.
//
// Para ejecutar este test manualmente:
//  1. Completar GMAIL_USER, GMAIL_APP_PASSWORD, SMTP_HOST, SMTP_PORT en .env
//  2. go test ./cmd/api/infrastructure/mailer/... -run TestSend_RealEmail_Integration -v
//  3. Verificar la bandeja de entrada de GMAIL_USER
func TestSend_RealEmail_Integration(t *testing.T) {
	if config.MySMTP.User == "" || config.MySMTP.AppPassword == "" {
		t.Skip("SMTP env vars no configuradas, saltando test de integración")
	}

	client, err := New(
		WithHost(config.MySMTP.Host),
		WithPort(config.MySMTP.Port),
		WithCredentials(config.MySMTP.User, config.MySMTP.AppPassword),
	)
	require.NoError(t, err)

	html, err := RenderWelcomeEmail(WelcomeEmailData{Name: "Juan"})
	require.NoError(t, err)
	assert.Contains(t, html, "Juan")

	err = client.Send(
		context.Background(),
		config.MySMTP.User,
		"Paceron - Test de bienvenida",
		html,
	)
	assert.NoError(t, err)
}
