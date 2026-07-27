package mailer

import (
	"bytes"
	_ "embed"
	"html/template"
)

//go:embed templates/welcome.html
var welcomeTemplateHTML string

//go:embed templates/reset.html
var resetTemplateHTML string

//go:embed templates/invitation.html
var invitationTemplateHTML string

// WelcomeEmailData contiene las variables disponibles en el template de bienvenida.
type WelcomeEmailData struct {
	Name string
}

// PasswordResetEmailData contiene las variables disponibles en el template de recuperación de contraseña.
type PasswordResetEmailData struct {
	Name string
	Code string
}

// InvitationEmailData contiene las variables disponibles en el template de invitación.
type InvitationEmailData struct {
	Name     string
	TeamName string
}

// RenderWelcomeEmail renderiza el template HTML de bienvenida con los datos dados.
func RenderWelcomeEmail(data WelcomeEmailData) (string, error) {
	tmpl, err := template.New("welcome").Parse(welcomeTemplateHTML)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderPasswordResetEmail renderiza el template HTML de recuperación de contraseña con los datos dados.
func RenderPasswordResetEmail(data PasswordResetEmailData) (string, error) {
	tmpl, err := template.New("reset").Parse(resetTemplateHTML)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderInvitationEmail renderiza el template HTML de invitación con los datos dados.
func RenderInvitationEmail(data InvitationEmailData) (string, error) {
	tmpl, err := template.New("invitation").Parse(invitationTemplateHTML)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
