package mailer

import (
	"bytes"
	"embed"
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

//go:embed templates/invitation_response.html
var invitationResponseTemplateHTML string

//go:embed templates/team_removed.html
var teamRemovedTemplateHTML string

//go:embed templates/team_member_left.html
var teamMemberLeftTemplateHTML string

//go:embed templates/password_changed.html
var passwordChangedTemplateHTML string

// iconAssets embebe los 8 íconos de acento (uno por tipo de correo, ver eventIcons
// más abajo) — badges circulares rasterizados desde Material Community Icons con
// ImageMagick, mismo mecanismo que el logo (mailer.go).
//
//go:embed assets/icon-welcome.png assets/icon-farewell.png assets/icon-password-reset.png assets/icon-password-changed.png assets/icon-invitation.png assets/icon-invitation-response.png assets/icon-team-removed.png assets/icon-team-member-left.png
var iconAssets embed.FS

// eventIconContentID es el Content-ID compartido por el ícono de acento en todos
// los templates — no colisiona con logoContentID (el logo) porque cada envío
// adjunta como máximo un ícono de evento a la vez.
const eventIconContentID = "event-icon"

// EmailType identifica cada tipo de correo que el sistema sabe enviar.
type EmailType string

const (
	EmailTypeWelcome            EmailType = "welcome"
	EmailTypeFarewell           EmailType = "farewell"
	EmailTypePasswordReset      EmailType = "password_reset"
	EmailTypeInvitation         EmailType = "invitation"
	EmailTypeInvitationResponse EmailType = "invitation_response"
	EmailTypeTeamRemoved        EmailType = "team_removed"
	EmailTypeTeamMemberLeft     EmailType = "team_member_left"
	EmailTypePasswordChanged    EmailType = "password_changed"
)

// EmailData contiene las variables disponibles en los templates de correo.
// Cada template usa solo los campos que necesita; el resto se renderiza vacío.
type EmailData struct {
	Name     string
	Code     string
	TeamName string

	// RelatedUserName es el otro usuario relevante al evento: quien respondió una
	// invitación, o quien dejó el equipo. ResponseStatus es "aceptada"/"rechazada",
	// usado solo por EmailTypeInvitationResponse.
	RelatedUserName string
	ResponseStatus  string
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
	EmailTypeInvitationResponse: {
		subject: mustParseSubject(EmailTypeInvitationResponse, "Respuesta a tu invitación de {{.TeamName}} - Paceron"),
		body:    mustParseBody(EmailTypeInvitationResponse, invitationResponseTemplateHTML),
	},
	EmailTypeTeamRemoved: {
		subject: mustParseSubject(EmailTypeTeamRemoved, "Saliste del equipo {{.TeamName}} - Paceron"),
		body:    mustParseBody(EmailTypeTeamRemoved, teamRemovedTemplateHTML),
	},
	EmailTypeTeamMemberLeft: {
		subject: mustParseSubject(EmailTypeTeamMemberLeft, "Un corredor dejó {{.TeamName}} - Paceron"),
		body:    mustParseBody(EmailTypeTeamMemberLeft, teamMemberLeftTemplateHTML),
	},
	EmailTypePasswordChanged: {
		subject: mustParseSubject(EmailTypePasswordChanged, "Tu contraseña fue actualizada - Paceron"),
		body:    mustParseBody(EmailTypePasswordChanged, passwordChangedTemplateHTML),
	},
}

// eventIconPaths asocia cada tipo de correo con su ícono de acento embebido
// (dentro de iconAssets). Todos comparten eventIconContentID como Content-ID.
var eventIconPaths = map[EmailType]string{
	EmailTypeWelcome:            "assets/icon-welcome.png",
	EmailTypeFarewell:           "assets/icon-farewell.png",
	EmailTypePasswordReset:      "assets/icon-password-reset.png",
	EmailTypePasswordChanged:    "assets/icon-password-changed.png",
	EmailTypeInvitation:         "assets/icon-invitation.png",
	EmailTypeInvitationResponse: "assets/icon-invitation-response.png",
	EmailTypeTeamRemoved:        "assets/icon-team-removed.png",
	EmailTypeTeamMemberLeft:     "assets/icon-team-member-left.png",
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
