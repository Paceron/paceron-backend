package mailer

import (
	"context"
	"fmt"

	mail "github.com/wneessen/go-mail"
)

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
	SendWelcomeEmail(ctx context.Context, to, name string) error
	SendFarewellEmail(ctx context.Context, to, name string) error
}

// Client envía correos electrónicos vía SMTP.
type Client struct {
	host     string
	port     int
	username string
	password string

	logger Logger
}

// New construye un Client de mailer aplicando las opciones dadas.
func New(opts ...Option) (*Client, error) {
	client := &Client{
		port: 587,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Send renderiza y envía un correo HTML a un único destinatario.
func (c *Client) Send(ctx context.Context, to, subject, htmlBody string) error {
	smtpClient, err := mail.NewClient(
		c.host,
		mail.WithPort(c.port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(c.username),
		mail.WithPassword(c.password),
		mail.WithTLSPolicy(mail.TLSMandatory),
	)
	if err != nil {
		c.logError(ctx, "error creando smtp client", err)
		return fmt.Errorf("mailer: error creando smtp client: %w", err)
	}

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

	if err := smtpClient.DialAndSendWithContext(ctx, msg); err != nil {
		c.logError(ctx, "error enviando email", err)
		return fmt.Errorf("mailer: error enviando email: %w", err)
	}

	c.logInfo(ctx, "email enviado exitosamente", to)
	return nil
}

// SendWelcomeEmail renderiza el template de bienvenida y lo envía a un destinatario.
func (c *Client) SendWelcomeEmail(ctx context.Context, to, name string) error {
	html, err := RenderWelcomeEmail(WelcomeEmailData{Name: name})
	if err != nil {
		return fmt.Errorf("mailer: error renderizando template: %w", err)
	}
	return c.Send(ctx, to, "Bienvenido a Paceron", html)
}

// SendFarewellEmail renderiza el template de despedida y lo envía a un destinatario.
func (c *Client) SendFarewellEmail(ctx context.Context, to, name string) error {
	html, err := RenderFarewellEmail(FarewellEmailData{Name: name})
	if err != nil {
		return fmt.Errorf("mailer: error renderizando template: %w", err)
	}
	return c.Send(ctx, to, "Tu cuenta fue desactivada", html)
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
