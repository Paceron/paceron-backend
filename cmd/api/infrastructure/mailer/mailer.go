package mailer

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/infrastructure/httpclient"
)

//go:embed assets/paceron-logo.png
var logoAssets embed.FS

// logoContentID es el Content-ID con el que los templates referencian el logo
// embebido (`<img src="cid:paceron-logo">`). CID embebido en vez de imagen
// remota o base64 inline: es el mecanismo que los clientes de correo
// (Gmail, Outlook, Apple Mail) cargan de forma confiable.
const logoContentID = "paceron-logo"
const logoAssetPath = "assets/paceron-logo.png"

const resendBaseURL = "https://api.resend.com"
const resendSendPath = "/emails"

// MailerInterface permite mockear el envío de correos en los consumidores.
type MailerInterface interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
	SendEmail(ctx context.Context, to string, emailType EmailType, data EmailData) error
}

// Client envía correos electrónicos vía la API HTTP de Resend.
//
// Antes usaba SMTP crudo contra Gmail — se migró porque el egress compartido de
// Render tenía timeouts de conexión TCP intermitentes contra el puerto 587, sin
// retry. Al ser HTTP, el envío es una llamada más contra infrastructure/httpclient,
// que ya trae retry/timeout/circuit-breaker sin escribir resiliencia nueva acá.
type Client struct {
	apiKey string
	from   string
	logger httpclient.Logger

	httpClient *httpclient.Client
}

// New construye un Client de mailer aplicando las opciones dadas.
func New(opts ...Option) (*Client, error) {
	client := &Client{}

	for _, opt := range opts {
		opt(client)
	}

	if client.apiKey == "" {
		return nil, fmt.Errorf("mailer: RESEND_API_KEY requerida")
	}
	if client.from == "" {
		return nil, fmt.Errorf("mailer: from address requerido")
	}

	httpOpts := []httpclient.Option{
		httpclient.WithBaseURL(resendBaseURL),
		httpclient.WithHeader("Authorization", "Bearer "+client.apiKey),
		httpclient.WithTimeout(8 * time.Second),
		httpclient.WithRetry(2, 500*time.Millisecond),
	}
	if client.logger != nil {
		httpOpts = append(httpOpts, httpclient.WithLogger(client.logger))
	}
	client.httpClient = httpclient.New(httpOpts...)

	return client, nil
}

type resendAttachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentID   string `json:"content_id,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

type resendEmailRequest struct {
	From        string             `json:"from"`
	To          []string           `json:"to"`
	Subject     string             `json:"subject"`
	HTML        string             `json:"html"`
	Attachments []resendAttachment `json:"attachments,omitempty"`
}

// Send envía un correo HTML ya renderizado a un único destinatario, con el logo
// de Paceron embebido como attachment inline (referenciado por content_id desde
// el HTML vía `cid:paceron-logo`).
func (c *Client) Send(ctx context.Context, to, subject, htmlBody string) error {
	logoBytes, err := logoAssets.ReadFile(logoAssetPath)
	if err != nil {
		c.logError(ctx, "error leyendo logo embebido", err)
		return fmt.Errorf("mailer: error leyendo logo embebido: %w", err)
	}

	body := resendEmailRequest{
		From:    c.from,
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
		Attachments: []resendAttachment{
			{
				Filename:    "paceron-logo.png",
				Content:     base64.StdEncoding.EncodeToString(logoBytes),
				ContentID:   logoContentID,
				ContentType: "image/png",
			},
		},
	}

	if err := c.httpClient.Post(ctx, resendSendPath, body, nil); err != nil {
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
