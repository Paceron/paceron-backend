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
// TestRenderEmail_ReferencesEmbeddedLogo protege el header con marca: cada
// template debe apuntar al logo embebido por Content-ID, no a una imagen
// remota ni al viejo texto plano "Paceron" en el header.
func TestRenderEmail_ReferencesEmbeddedLogo(t *testing.T) {
	for _, emailType := range allEmailTypes {
		t.Run(string(emailType), func(t *testing.T) {
			_, html, err := RenderEmail(emailType, EmailData{})

			require.NoError(t, err)
			assert.Contains(t, html, "cid:"+logoContentID)
		})
	}
}

// TestLogoAssets_EmbedsExpectedFile protege el path del go:embed: si el asset
// se mueve o se borra, esto falla en tests en vez de romper el envío recién en
// producción (el error solo aparece ahí cuando msg.EmbedFromEmbedFS corre).
func TestLogoAssets_EmbedsExpectedFile(t *testing.T) {
	data, err := logoAssets.ReadFile("assets/paceron-logo.png")

	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestEmailTemplates_AllTypesRegistered(t *testing.T) {
	for _, emailType := range allEmailTypes {
		tmpl, ok := emailTemplates[emailType]
		require.True(t, ok, "tipo %s no registrado en emailTemplates", emailType)
		assert.NotNil(t, tmpl.subject, "tipo %s sin asunto parseado", emailType)
		assert.NotNil(t, tmpl.body, "tipo %s sin template parseado", emailType)
	}
}

// --- Construcción del cliente ---

func TestNew_BuildsClient(t *testing.T) {
	client, err := New(
		WithAPIKey("re_test_key"),
		WithFrom("Paceron <no-reply@paceron.com>"),
	)

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.httpClient, "el httpClient de Resend debe construirse en New")
	assert.Equal(t, "re_test_key", client.apiKey)
	assert.Equal(t, "Paceron <no-reply@paceron.com>", client.from)
}

func TestNew_MissingAPIKeyReturnsError(t *testing.T) {
	_, err := New(WithFrom("Paceron <no-reply@paceron.com>"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RESEND_API_KEY")
}

func TestNew_MissingFromReturnsError(t *testing.T) {
	_, err := New(WithAPIKey("re_test_key"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "from address")
}

// TestSendEmail_UnknownTypeDoesNotSend verifica que un tipo inválido falle en el
// renderizado, sin intentar llamar a la API de Resend.
func TestSendEmail_UnknownTypeDoesNotSend(t *testing.T) {
	client, err := New(
		WithAPIKey("re_test_key"),
		WithFrom("Paceron <no-reply@paceron.com>"),
	)
	require.NoError(t, err)

	err = client.SendEmail(context.Background(), "dest@test.com", EmailType("no-existe"), EmailData{Name: "Juan"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tipo de email desconocido")
}

// --- Envío real (requiere RESEND_API_KEY) ---

// TestSendEmail_RealEmail_Integration envía correos reales vía la API de Resend
// usando las credenciales configuradas en el entorno. Se skipea automáticamente
// si RESEND_API_KEY no está seteada, para que `go test ./...` siga funcionando en
// máquinas sin credenciales configuradas.
//
// Para ejecutar este test manualmente:
//  1. Completar RESEND_API_KEY y RESEND_FROM_ADDRESS en .env (dominio verificado en Resend)
//  2. go test ./cmd/api/infrastructure/mailer/... -run Integration -v
//  3. Verificar la bandeja de entrada del address configurado como from
func TestSendEmail_RealEmail_Integration(t *testing.T) {
	if config.MyMailer.APIKey == "" {
		t.Skip("RESEND_API_KEY no configurada, saltando test de integración")
	}

	client, err := New(
		WithAPIKey(config.MyMailer.APIKey),
		WithFrom(config.MyMailer.From),
	)
	require.NoError(t, err)

	data := EmailData{Name: "Juan", Code: "123456", TeamName: "Los Pumas"}
	for _, emailType := range allEmailTypes {
		t.Run(string(emailType), func(t *testing.T) {
			err := client.SendEmail(context.Background(), config.MyMailer.From, emailType, data)
			assert.NoError(t, err)
		})
	}
}
