package mailer

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"time"

	"simple-arq-golang/cmd/api/infrastructure/httpclient"
)

//go:embed assets/paceron-logo.png
var logoAssets embed.FS

// logoContentID es el Content-ID con el que los templates referencian el logo
// embebido (`<img src="cid:header-mark">`). CID embebido en vez de imagen
// remota o base64 inline: es el mecanismo que los clientes de correo
// (Gmail, Outlook, Apple Mail) cargan de forma confiable. Nombre neutro
// ("header-mark", no "logo") y filename a tono — descartando la hipótesis de que
// la palabra "logo" en el nombre influye en si Gmail lo trata como adjunto.
const logoContentID = "header-mark"
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
// el HTML vía `cid:paceron-logo`). Sin ícono de evento — eso es exclusivo de
// SendEmail, que conoce el EmailType.
func (c *Client) Send(ctx context.Context, to, subject, htmlBody string) error {
	return c.send(ctx, to, subject, htmlBody, nil)
}

// SendEmail renderiza el template del tipo de correo indicado y lo envía, con el
// ícono de acento correspondiente (ver eventIconPaths en render.go) además del
// logo. Es el único punto de entrada para mandar correos con template: para sumar
// un tipo nuevo alcanza con registrarlo en emailTemplates y (opcionalmente)
// eventIconPaths, sin tocar esta función.
func (c *Client) SendEmail(ctx context.Context, to string, emailType EmailType, data EmailData) error {
	subject, htmlBody, err := RenderEmail(emailType, data)
	if err != nil {
		c.logError(ctx, "error renderizando email", err)
		return err
	}

	return c.send(ctx, to, subject, htmlBody, c.eventIconAttachment(ctx, emailType))
}

// eventIconAttachment resuelve el attachment del ícono de acento para un tipo de
// correo. nil si el tipo no tiene ícono registrado o si falla la lectura — un
// ícono faltante no debe impedir el envío del correo.
func (c *Client) eventIconAttachment(ctx context.Context, emailType EmailType) *resendAttachment {
	path, ok := eventIconPaths[emailType]
	if !ok {
		return nil
	}
	iconBytes, err := iconAssets.ReadFile(path)
	if err != nil {
		c.logError(ctx, "error leyendo ícono de evento embebido", err)
		return nil
	}
	return &resendAttachment{
		Filename:    filepath.Base(path),
		Content:     base64.StdEncoding.EncodeToString(iconBytes),
		ContentID:   eventIconContentID,
		ContentType: "image/png",
	}
}

// send arma y envía el email vía la API de Resend, con el logo de Paceron siempre
// embebido y, opcionalmente, el ícono de acento del tipo de correo.
func (c *Client) send(ctx context.Context, to, subject, htmlBody string, eventIcon *resendAttachment) error {
	logoBytes, err := logoAssets.ReadFile(logoAssetPath)
	if err != nil {
		c.logError(ctx, "error leyendo logo embebido", err)
		return fmt.Errorf("mailer: error leyendo logo embebido: %w", err)
	}

	attachments := []resendAttachment{
		{
			Filename:    "header-mark.png",
			Content:     base64.StdEncoding.EncodeToString(logoBytes),
			ContentID:   logoContentID,
			ContentType: "image/png",
		},
	}
	if eventIcon != nil {
		attachments = append(attachments, *eventIcon)
	}

	body := resendEmailRequest{
		From:        c.from,
		To:          []string{to},
		Subject:     subject,
		HTML:        htmlBody,
		Attachments: attachments,
	}

	if err := c.httpClient.Post(ctx, resendSendPath, body, nil); err != nil {
		c.logError(ctx, "error enviando email", err)
		return fmt.Errorf("mailer: error enviando email: %w", err)
	}

	c.logInfo(ctx, "email enviado exitosamente", to)
	return nil
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
