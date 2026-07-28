package mailer

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
)

//go:embed templates/welcome.html
var welcomeTemplateHTML string

//go:embed templates/farewell.html
var farewellTemplateHTML string

// EmailType identifica cada tipo de correo que el sistema sabe enviar.
type EmailType string

const (
	EmailTypeWelcome  EmailType = "welcome"
	EmailTypeFarewell EmailType = "farewell"
)

// EmailData contiene las variables disponibles en los templates de correo.
type EmailData struct {
	Name string
}

// emailTemplate asocia un tipo de correo con su asunto y su template ya parseado.
type emailTemplate struct {
	subject  string
	template *template.Template
}

// emailTemplates es el registro de tipos de correo soportados. Para agregar un
// tipo nuevo alcanza con embeber el HTML y sumar una entrada acá.
// Los templates se parsean una sola vez al cargar el package: un error de
// parseo es un error de programación en un archivo embebido, no una condición
// de runtime, por eso se usa template.Must.
var emailTemplates = map[EmailType]emailTemplate{
	EmailTypeWelcome: {
		subject:  "Bienvenido a Paceron",
		template: template.Must(template.New(string(EmailTypeWelcome)).Parse(welcomeTemplateHTML)),
	},
	EmailTypeFarewell: {
		subject:  "Tu cuenta fue desactivada",
		template: template.Must(template.New(string(EmailTypeFarewell)).Parse(farewellTemplateHTML)),
	},
}

// RenderEmail renderiza el asunto y el cuerpo HTML del tipo de correo dado.
func RenderEmail(emailType EmailType, data EmailData) (string, string, error) {
	tmpl, ok := emailTemplates[emailType]
	if !ok {
		return "", "", fmt.Errorf("mailer: tipo de email desconocido: %s", emailType)
	}

	var buf bytes.Buffer
	if err := tmpl.template.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("mailer: error renderizando template %s: %w", emailType, err)
	}

	return tmpl.subject, buf.String(), nil
}
