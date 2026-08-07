package mailer

import (
	"context"
	"embed"
	"fmt"

	mail "github.com/wneessen/go-mail"
)

//go:embed assets/paceron-logo.png
var logoAssets embed.FS

// logoContentID es el Content-ID con el que los templates referencian el logo
// embebido (`<img src="cid:paceron-logo">`). CID embebido en vez de imagen
// remota o base64 inline: es el mecanismo que los clientes de correo
// (Gmail, Outlook, Apple Mail) cargan de forma confiable.
const logoContentID = "paceron-logo"

// Logger define el contrato de logging que Client usa, mismo shape que
// infrastructure/httpclient.Logger para poder reusar el mismo adapter.
type Logger interface {
	Info(
		ctx context.Context,
		message string,
		fields map[string]any,
	)

	Warn(
		ctx context.Context,
		message string,
		fields map[string]any,
	)

	Error(
		ctx context.Context,
		message string,
		err error,
		fields map[string]any,
	)
}

// MailerInterface permite mockear el envío de correos en los consumidores.
type MailerInterface interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
	SendEmail(ctx context.Context, to string, emailType EmailType, data EmailData) error
}

// Client envía correos electrónicos vía SMTP.
type Client struct {
	host     string
	port     int
	username string
	password string

	logger Logger

	// smtpClient es la única instancia del cliente SMTP, compartida por todos
	// los envíos. Ver New para el detalle de por qué es seguro reutilizarla.
	smtpClient *mail.Client
}

// New construye un Client de mailer aplicando las opciones dadas.
//
// El cliente SMTP subyacente se construye una sola vez acá y se reutiliza en
// cada envío, en lugar de instanciarse por correo. Es seguro compartirlo entre
// goroutines: go-mail abre y cierra una conexión propia dentro de cada
// DialAndSendWithContext (no guarda la conexión en el struct) y protege el
// acceso a su configuración con un RWMutex interno.
func New(opts ...Option) (*Client, error) {
	client := &Client{
		port: 587,
	}

	for _, opt := range opts {
		opt(client)
	}

	smtpClient, err := mail.NewClient(
		client.host,
		mail.WithPort(client.port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(client.username),
		mail.WithPassword(client.password),
		mail.WithTLSPolicy(mail.TLSMandatory),
	)
	if err != nil {
		return nil, fmt.Errorf("mailer: error creando smtp client: %w", err)
	}
	client.smtpClient = smtpClient

	return client, nil
}

// Send envía un correo HTML ya renderizado a un único destinatario.
func (c *Client) Send(ctx context.Context, to, subject, htmlBody string) error {
	msg := mail.NewMsg()
	if err := msg.From(c.username); err != nil {
		c.logError(ctx, "error seteando remitente", err)
		return fmt.Errorf("mailer: error seteando remitente: %w", err)
	}
	if err := msg.To(to); err != nil {
		c.logError(ctx, "error seteando destinatario", err)
		return fmt.Errorf("mailer: error seteando destinatario: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, htmlBody)

	if err := msg.EmbedFromEmbedFS("assets/paceron-logo.png", &logoAssets, mail.WithFileContentID(logoContentID)); err != nil {
		c.logError(ctx, "error embebiendo logo", err)
		return fmt.Errorf("mailer: error embebiendo logo: %w", err)
	}

	if err := c.smtpClient.DialAndSendWithContext(ctx, msg); err != nil {
		c.logError(ctx, "error enviando email", err)
		return fmt.Errorf("mailer: error enviando email: %w", err)
	}

	c.logInfo(ctx, "email enviado exitosamente", to)
	return nil
}

// SendEmail renderiza el template del tipo de correo indicado y lo envía.
// Es el único punto de entrada para mandar correos con template: para sumar un
// tipo nuevo alcanza con registrarlo en emailTemplates (ver render.go), sin
// tocar esta función.
func (c *Client) SendEmail(ctx context.Context, to string, emailType EmailType, data EmailData) error {
	subject, htmlBody, err := RenderEmail(emailType, data)
	if err != nil {
		c.logError(ctx, "error renderizando email", err)
		return err
	}

	return c.Send(ctx, to, subject, htmlBody)
}

func (c *Client) logInfo(ctx context.Context, message, to string) {
	if c.logger == nil {
		return
	}
	c.logger.Info(ctx, message, map[string]any{"to": to})
}

func (c *Client) logError(ctx context.Context, message string, err error) {
	if c.logger == nil {
		return
	}
	c.logger.Error(ctx, message, err, nil)
}
