## 1. Asset

- [x] 1.1 Componer `paceron-logo.png` (ícono + wordmark monocromático, `Orbitron_700Bold`, `skewX(-15deg)`) con ImageMagick desde `paceron-frontend/assets/`
- [x] 1.2 Verificar contraste sobre el fondo verde del header (`#8cc63e`) antes de finalizar el asset
- [x] 1.3 Guardar en `cmd/api/infrastructure/mailer/assets/paceron-logo.png`

## 2. Backend

- [x] 2.1 `mailer.go`: `go:embed assets/paceron-logo.png` en un `embed.FS`, constante `logoContentID`
- [x] 2.2 `Client.Send`: embeber el logo por Content-ID en cada envío vía `msg.EmbedFromEmbedFS` + `mail.WithFileContentID`
- [x] 2.3 Los 4 templates (`welcome.html`, `farewell.html`, `reset.html`, `invitation.html`): reemplazar el `<span>Paceron</span>` del header por `<img src="cid:paceron-logo">`

## 3. Tests

- [x] 3.1 `TestRenderEmail_ReferencesEmbeddedLogo`: cada template renderizado contiene `cid:paceron-logo`
- [x] 3.2 `TestLogoAssets_EmbedsExpectedFile`: el `go:embed` resuelve el archivo esperado
- [x] 3.3 `go build`/`go vet`/`go test ./...` verdes
- [x] 3.4 Verificación manual: `TestSendEmail_RealEmail_Integration` corrido con credenciales SMTP reales (`.env` local) — 4 correos reales enviados sin error con el logo embebido
