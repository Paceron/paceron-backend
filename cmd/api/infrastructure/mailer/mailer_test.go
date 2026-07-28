package mailer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
)

// allEmailTypes lista los tipos soportados, para recorrerlos en tests genéricos.
var allEmailTypes = []EmailType{
	EmailTypeWelcome,
	EmailTypeFarewell,
	EmailTypePasswordReset,
	EmailTypeInvitation,
}

// --- Renderizado de templates (no requieren SMTP, corren en cualquier entorno) ---

func TestRenderEmail_Welcome(t *testing.T) {
	subject, html, err := RenderEmail(EmailTypeWelcome, EmailData{Name: "Maria"})

	require.NoError(t, err)
	assert.Equal(t, "Bienvenido a Paceron", subject)
	assert.Contains(t, html, "Maria")
	assert.Contains(t, html, "Paceron")
	assert.Contains(t, html, "#8cc63e")
}

func TestRenderEmail_Farewell(t *testing.T) {
	subject, html, err := RenderEmail(EmailTypeFarewell, EmailData{Name: "Maria"})

	require.NoError(t, err)
	assert.Equal(t, "Tu cuenta fue desactivada", subject)
	assert.Contains(t, html, "Maria")
	assert.Contains(t, html, "Paceron")
	assert.Contains(t, html, "#8cc63e")
}

func TestRenderEmail_PasswordReset(t *testing.T) {
	subject, html, err := RenderEmail(EmailTypePasswordReset, EmailData{Name: "Maria", Code: "123456"})

	require.NoError(t, err)
	assert.Equal(t, "Recuperación de contraseña - Paceron", subject)
	assert.Contains(t, html, "Maria")
	assert.Contains(t, html, "123456")
	assert.Contains(t, html, "Paceron")
}

func TestRenderEmail_Invitation(t *testing.T) {
	_, html, err := RenderEmail(EmailTypeInvitation, EmailData{Name: "Maria", TeamName: "Los Pumas"})

	require.NoError(t, err)
	assert.Contains(t, html, "Maria")
	assert.Contains(t, html, "Los Pumas")
	assert.Contains(t, html, "Paceron")
}

// TestRenderEmail_InvitationSubjectIsDynamic cubre el único asunto parametrizado:
// se renderiza como template, no es un string fijo.
func TestRenderEmail_InvitationSubjectIsDynamic(t *testing.T) {
	subject, _, err := RenderEmail(EmailTypeInvitation, EmailData{Name: "Maria", TeamName: "Los Pumas"})

	require.NoError(t, err)
	assert.Equal(t, "Invitación a equipo Los Pumas - Paceron", subject)
}

// TestRenderEmail_SubjectIsNotHTMLEscaped protege la decisión de renderizar el
// asunto con text/template: escapar entidades ahí las mostraría literales en el
// cliente de correo (ej. "Ñandú & Cía" no debe volverse "&amp;").
func TestRenderEmail_SubjectIsNotHTMLEscaped(t *testing.T) {
	subject, _, err := RenderEmail(EmailTypeInvitation, EmailData{TeamName: "Ñandú & Cía"})

	require.NoError(t, err)
	assert.Equal(t, "Invitación a equipo Ñandú & Cía - Paceron", subject)
	assert.NotContains(t, subject, "&amp;")
}

func TestRenderEmail_UnknownTypeReturnsError(t *testing.T) {
	subject, html, err := RenderEmail(EmailType("no-existe"), EmailData{Name: "Maria"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tipo de email desconocido")
	assert.Empty(t, subject)
	assert.Empty(t, html)
}

// TestRenderEmail_AutoEscaping verifica el requisito de la spec: los datos del
// usuario se escapan, sin permitir inyección de HTML/JS en el cuerpo del correo.
func TestRenderEmail_AutoEscaping(t *testing.T) {
	for _, emailType := range allEmailTypes {
		t.Run(string(emailType), func(t *testing.T) {
			_, html, err := RenderEmail(emailType, EmailData{
				Name:     "<script>alert('xss')</script>",
				Code:     "<script>alert('xss')</script>",
				TeamName: "<script>alert('xss')</script>",
			})

			require.NoError(t, err)
			assert.NotContains(t, html, "<script>")
			assert.Contains(t, html, "&lt;script&gt;")
		})
	}
}

func TestRenderEmail_EmptyDataStillRenders(t *testing.T) {
	for _, emailType := range allEmailTypes {
		t.Run(string(emailType), func(t *testing.T) {
			subject, html, err := RenderEmail(emailType, EmailData{})

			require.NoError(t, err)
			assert.NotEmpty(t, subject)
			assert.Contains(t, html, "Paceron")
		})
	}
}

// TestEmailTemplates_AllTypesRegistered protege el registro: cada tipo declarado
// debe tener asunto y template válidos, para que agregar un tipo nuevo sin
// registrarlo falle en tests y no en producción.
func TestEmailTemplates_AllTypesRegistered(t *testing.T) {
	for _, emailType := range allEmailTypes {
		tmpl, ok := emailTemplates[emailType]
		require.True(t, ok, "tipo %s no registrado en emailTemplates", emailType)
		assert.NotNil(t, tmpl.subject, "tipo %s sin asunto parseado", emailType)
		assert.NotNil(t, tmpl.body, "tipo %s sin template parseado", emailType)
	}
}

// --- Construcción del cliente ---

func TestNew_BuildsSingleSMTPClient(t *testing.T) {
	client, err := New(
		WithHost("smtp.gmail.com"),
		WithPort(587),
		WithCredentials("user@gmail.com", "app-password"),
	)

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.smtpClient, "el cliente SMTP debe construirse una sola vez en New")
	assert.Equal(t, "smtp.gmail.com", client.host)
	assert.Equal(t, 587, client.port)
}

func TestNew_DefaultPortIs587(t *testing.T) {
	client, err := New(WithHost("smtp.gmail.com"))

	require.NoError(t, err)
	assert.Equal(t, 587, client.port)
}

// TestNew_ReusesSameSMTPClientAcrossSends verifica el punto central del refactor:
// la instancia SMTP es única y no se reconstruye por envío.
func TestNew_ReusesSameSMTPClientAcrossSends(t *testing.T) {
	client, err := New(
		WithHost("smtp.gmail.com"),
		WithCredentials("user@gmail.com", "app-password"),
	)
	require.NoError(t, err)

	first := client.smtpClient
	_ = client.Send(context.Background(), "dest@test.com", "asunto", "<p>cuerpo</p>")

	assert.Same(t, first, client.smtpClient, "Send no debe reemplazar la instancia SMTP compartida")
}

func TestNew_InvalidPortReturnsError(t *testing.T) {
	_, err := New(
		WithHost("smtp.gmail.com"),
		WithPort(-1),
		WithCredentials("user@gmail.com", "app-password"),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error creando smtp client")
}

// TestSendEmail_UnknownTypeDoesNotSend verifica que un tipo inválido falle en el
// renderizado, sin intentar abrir una conexión SMTP.
func TestSendEmail_UnknownTypeDoesNotSend(t *testing.T) {
	client, err := New(
		WithHost("smtp.gmail.com"),
		WithCredentials("user@gmail.com", "app-password"),
	)
	require.NoError(t, err)

	err = client.SendEmail(context.Background(), "dest@test.com", EmailType("no-existe"), EmailData{Name: "Juan"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tipo de email desconocido")
}

// --- Envío real (requiere credenciales SMTP) ---

// TestSendEmail_RealEmail_Integration envía correos reales usando las
// credenciales SMTP configuradas en el entorno. Se skipea automáticamente si
// las variables de entorno SMTP no están seteadas, para que `go test ./...`
// siga funcionando en máquinas sin credenciales configuradas.
//
// Para ejecutar este test manualmente:
//  1. Completar GMAIL_USER, GMAIL_APP_PASSWORD, SMTP_HOST, SMTP_PORT en .env
//  2. go test ./cmd/api/infrastructure/mailer/... -run Integration -v
//  3. Verificar la bandeja de entrada de GMAIL_USER
func TestSendEmail_RealEmail_Integration(t *testing.T) {
	if config.MySMTP.User == "" || config.MySMTP.AppPassword == "" {
		t.Skip("SMTP env vars no configuradas, saltando test de integración")
	}

	client, err := New(
		WithHost(config.MySMTP.Host),
		WithPort(config.MySMTP.Port),
		WithCredentials(config.MySMTP.User, config.MySMTP.AppPassword),
	)
	require.NoError(t, err)

	data := EmailData{Name: "Juan", Code: "123456", TeamName: "Los Pumas"}
	for _, emailType := range allEmailTypes {
		t.Run(string(emailType), func(t *testing.T) {
			err := client.SendEmail(context.Background(), config.MySMTP.User, emailType, data)
			assert.NoError(t, err)
		})
	}
}
