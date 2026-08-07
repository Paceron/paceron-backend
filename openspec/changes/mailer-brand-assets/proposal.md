## Why

Los 4 templates de mail (`welcome`, `farewell`, `reset`, `invitation`) tenían un encabezado con solo el texto plano "Paceron" — nunca se aplicó la marca real (ícono + wordmark en `Orbitron_700Bold`) que ya usa el frontend. Quedó documentado como deuda diferida ([[project_email_brand_pending]]) hasta que la superficie de templates estuviera estable; ya lo está (4 templates, sin features de mail nuevas en el corto plazo).

## What Changes

- Se compone un logo único (ícono `paceron-symbol-transparent.png` + wordmark "paceron" en `Orbitron_700Bold`, `skewX(-15deg)`) tomando los assets fuente del frontend (`paceron-frontend/assets/`), vía ImageMagick, como PNG transparente (`cmd/api/infrastructure/mailer/assets/paceron-logo.png`).
- **Decisión de color, distinta al diseño original de la nota diferida**: el wordmark del frontend tiene la letra "a" en verde marca (`#8cc63e`) sobre fondo neutro. El header de los mails tiene fondo verde sólido (`#8cc63e`) — la "a" verde se volvía invisible ahí. Se resolvió manteniendo el header verde (no se toca el diseño existente) y componiendo el wordmark en un solo color oscuro (`#111518`, igual al texto que reemplaza), sin el acento verde en la "a". El ícono mantiene sus colores originales (ya tenía buen contraste sobre verde).
- El logo se embebe vía Content-ID (`msg.EmbedFromEmbedFS` + `WithFileContentID`, `go-mail`) en cada envío (`Client.Send`), no como imagen remota ni base64 inline — es el mecanismo que los clientes de correo (Gmail, Outlook, Apple Mail) cargan de forma confiable.
- Los 4 templates reemplazan el `<span>Paceron</span>` del header por `<img src="cid:paceron-logo">`.

## Capabilities

### Modified Capabilities
- Ninguna capability formal de OpenSpec documentaba el diseño visual de los templates — es un cambio de presentación sobre `infrastructure/mailer`, no de comportamiento/contrato (mismos `EmailType`, mismo `EmailData`, mismo asunto).

## Impact

- **Nuevo**: `cmd/api/infrastructure/mailer/assets/paceron-logo.png` (PNG transparente, 551x95px, ~16KB)
- **Modificado**: `mailer.go` (`go:embed` del asset, `Send` embebe el logo por CID en cada envío), los 4 templates HTML (header)
- **Tests**: `TestRenderEmail_ReferencesEmbeddedLogo` (cada template referencia `cid:paceron-logo`), `TestLogoAssets_EmbedsExpectedFile` (el `go:embed` resuelve el archivo)
- **Verificación manual**: `TestSendEmail_RealEmail_Integration` corrido con credenciales SMTP reales — los 4 tipos de correo se enviaron sin error con el logo embebido (confirma que el MIME multipart es válido, Gmail lo aceptó)
