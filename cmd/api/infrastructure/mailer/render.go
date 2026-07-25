package mailer

import (
	"bytes"
	_ "embed"
	"html/template"
)

//go:embed templates/welcome.html
var welcomeTemplateHTML string

// WelcomeEmailData contiene las variables disponibles en el template de bienvenida.
type WelcomeEmailData struct {
	Name string
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

//go:embed templates/farewell.html
var farewellTemplateHTML string

// FarewellEmailData contiene las variables disponibles en el template de despedida.
type FarewellEmailData struct {
	Name string
}

// RenderFarewellEmail renderiza el template HTML de despedida con los datos dados.
func RenderFarewellEmail(data FarewellEmailData) (string, error) {
	tmpl, err := template.New("farewell").Parse(farewellTemplateHTML)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
