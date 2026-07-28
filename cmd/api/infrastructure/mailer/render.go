package mailer

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	texttemplate "text/template"
)

//go:embed templates/welcome.html
var welcomeTemplateHTML string

//go:embed templates/farewell.html
var farewellTemplateHTML string

//go:embed templates/reset.html
var resetTemplateHTML string

//go:embed templates/invitation.html
var invitationTemplateHTML string

// EmailType identifica cada tipo de correo que el sistema sabe enviar.
type EmailType string

const (
	EmailTypeWelcome       EmailType = "welcome"
	EmailTypeFarewell      EmailType = "farewell"
	EmailTypePasswordReset EmailType = "password_reset"
	EmailTypeInvitation    EmailType = "invitation"
)

// EmailData contiene las variables disponibles en los templates de correo.
// Cada template usa solo los campos que necesita; el resto se renderiza vacío.
type EmailData struct {
	Name     string
	Code     string
	TeamName string
}

// emailTemplate asocia un tipo de correo con su asunto y su cuerpo, ambos ya parseados.
// El asunto es un template de texto (no HTML) porque escapar entidades en una
// línea de asunto la mostraría literal en el cliente de correo.
type emailTemplate struct {
	subject *texttemplate.Template
	body    *template.Template
}

// emailTemplates es el registro de tipos de correo soportados. Para agregar un
// tipo nuevo alcanza con embeber el HTML y sumar una entrada acá.
// Los templates se parsean una sola vez al cargar el package: un error de
// parseo es un error de programación en un archivo embebido, no una condición
// de runtime, por eso se usa Must.
var emailTemplates = map[EmailType]emailTemplate{
	EmailTypeWelcome: {
		subject: mustParseSubject(EmailTypeWelcome, "Bienvenido a Paceron"),
		body:    mustParseBody(EmailTypeWelcome, welcomeTemplateHTML),
	},
	EmailTypeFarewell: {
		subject: mustParseSubject(EmailTypeFarewell, "Tu cuenta fue desactivada"),
		body:    mustParseBody(EmailTypeFarewell, farewellTemplateHTML),
	},
	EmailTypePasswordReset: {
		subject: mustParseSubject(EmailTypePasswordReset, "Recuperación de contraseña - Paceron"),
		body:    mustParseBody(EmailTypePasswordReset, resetTemplateHTML),
	},
	EmailTypeInvitation: {
		subject: mustParseSubject(EmailTypeInvitation, "Invitación a equipo {{.TeamName}} - Paceron"),
		body:    mustParseBody(EmailTypeInvitation, invitationTemplateHTML),
	},
}

func mustParseSubject(emailType EmailType, subject string) *texttemplate.Template {
	return texttemplate.Must(texttemplate.New(string(emailType) + "_subject").Parse(subject))
}

func mustParseBody(emailType EmailType, body string) *template.Template {
	return template.Must(template.New(string(emailType)).Parse(body))
}

// RenderEmail renderiza el asunto y el cuerpo HTML del tipo de correo dado.
func RenderEmail(emailType EmailType, data EmailData) (string, string, error) {
	tmpl, ok := emailTemplates[emailType]
	if !ok {
		return "", "", fmt.Errorf("mailer: tipo de email desconocido: %s", emailType)
	}

	var subjectBuf bytes.Buffer
	if err := tmpl.subject.Execute(&subjectBuf, data); err != nil {
		return "", "", fmt.Errorf("mailer: error renderizando asunto %s: %w", emailType, err)
	}

	var bodyBuf bytes.Buffer
	if err := tmpl.body.Execute(&bodyBuf, data); err != nil {
		return "", "", fmt.Errorf("mailer: error renderizando template %s: %w", emailType, err)
	}

	return subjectBuf.String(), bodyBuf.String(), nil
}
