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

// WelcomeEmailData contiene las variables disponibles en el template de bienvenida.
type WelcomeEmailData struct {
	Name string
}

// PasswordResetEmailData contiene las variables disponibles en el template de recuperación de contraseña.
type PasswordResetEmailData struct {
	Name string
	Code string
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
