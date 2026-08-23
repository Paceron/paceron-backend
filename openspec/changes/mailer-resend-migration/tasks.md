## 1. Config

- [x] 1.1 Reemplazar `config.SMTP`/`MySMTP`/`loadSMTPConfig()` por `config.MailerConfig`/`MyMailer`/`loadMailerConfig()`, leyendo `RESEND_API_KEY`/`RESEND_FROM_ADDRESS`
- [x] 1.2 Actualizar `config_test.go` (`TestLoadMailerConfig`, `TestLoadMailerConfig_Empty`)
- [x] 1.3 Sacar import `strconv` de `config.go` (quedó sin uso tras sacar el parseo de `SMTP_PORT`)

## 2. Cliente Resend

- [x] 2.1 Reescribir `infrastructure/mailer/options.go`: `WithAPIKey`, `WithFrom`, `WithLogger(httpclient.Logger)`
- [x] 2.2 Reescribir `infrastructure/mailer/mailer.go`: `Client` envuelve `*httpclient.Client` (`WithBaseURL("https://api.resend.com")`, `WithHeader("Authorization", "Bearer "+apiKey)`, timeout 8s, retry 2 intentos); `New` valida `apiKey`/`from` requeridos
- [x] 2.3 `Send` arma `resendEmailRequest` (`from`, `to`, `subject`, `html`, `attachments[].content_id` para el logo embebido, verificado contra la doc de Resend — `content` en base64, `content_id` para referenciar `cid:` desde el HTML)
- [x] 2.4 Sacar dependencia `github.com/wneessen/go-mail` (`go mod tidy`)

## 3. Wiring

- [x] 3.1 `app.go`: reemplazar `mailer.WithHost/WithPort/WithCredentials` por `mailer.WithAPIKey(config.MyMailer.APIKey)` + `mailer.WithFrom(config.MyMailer.From)`

## 4. Tests

- [x] 4.1 Reescribir tests de construcción del cliente (`TestNew_BuildsClient`, `TestNew_MissingAPIKeyReturnsError`, `TestNew_MissingFromReturnsError`)
- [x] 4.2 Test de integración real (`TestSendEmail_RealEmail_Integration`) pasa a skipear por `RESEND_API_KEY` en vez de credenciales Gmail
- [x] 4.3 Tests de renderizado (`TestRenderEmail_*`) sin cambios — no dependen del transporte
- [x] 4.4 `go build ./...` / `go vet ./...` / `go test ./...` verdes

## 5. Env vars y docs

- [x] 5.1 `.env.example`: sacar sección SMTP/Gmail, agregar `RESEND_API_KEY`/`RESEND_FROM_ADDRESS`
- [x] 5.2 `render.yaml` (ambos services): mismo swap de keys
- [x] 5.3 `docs/ENVIRONMENTS.md`: actualizar checklist de valores a cargar por service

## 6. Verificación manual (a cargo del usuario)

- [ ] 6.1 Cuenta Resend + dominio verificado (SPF/DKIM)
- [ ] 6.2 `RESEND_API_KEY`/`RESEND_FROM_ADDRESS` cargadas en `.env` local y en ambos services de Render
- [ ] 6.3 Local: disparar un registro real, confirmar que llega el mail de bienvenida con el logo embebido correcto
- [ ] 6.4 Staging (`paceron-backend-develop.onrender.com`): mismo chequeo post-deploy — confirma que el fix resuelve el problema que era específico de Render
- [ ] 6.5 Confirmar que un fallo simulado (API key inválida) no rompe el flujo principal, solo loguea
