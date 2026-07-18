## 1. Setup

- [x] 1.1 Agregar dependencia `github.com/wneessen/go-mail@v0.8.1` al go.mod (`go get github.com/wneessen/go-mail@v0.8.1`)
- [x] 1.2 Agregar struct `SMTP` y variable `MySMTP` a `config/config.go`, con función `loadSMTPConfig()` invocada desde `initLocal()`, `initProd()` e `initTest()`
- [x] 1.3 Agregar tests para `loadSMTPConfig()` en `config/config_test.go` (caso con todas las variables seteadas, caso de puerto por defecto)
- [x] 1.4 Actualizar `.env.example` agregando la sección `# SMTP` con `GMAIL_USER`, `GMAIL_APP_PASSWORD`, `SMTP_HOST`, `SMTP_PORT`

## 2. Template HTML de bienvenida

- [x] 2.1 Crear `infrastructure/mailer/templates/welcome.html` con layout de tabla, CSS inline, colores de marca de Paceron y variable `{{.Name}}`
- [x] 2.2 Verificar visualmente el HTML antes de continuar

## 3. Renderizado de templates

- [x] 3.1 Crear `infrastructure/mailer/render.go` con `//go:embed templates/welcome.html`, struct `WelcomeEmailData { Name string }` y función `RenderWelcomeEmail(data WelcomeEmailData) (string, error)` usando `html/template`

## 4. Cliente SMTP

- [x] 4.1 Crear `infrastructure/mailer/options.go` con `type Option func(*Client)` y builders `WithHost`, `WithPort`, `WithCredentials`, `WithLogger`
- [x] 4.2 Crear `infrastructure/mailer/mailer.go` con struct `Client`, constructor `New(opts ...Option) (*Client, error)` y método `Send(ctx context.Context, to, subject, htmlBody string) error` usando `github.com/wneessen/go-mail` (puerto 587, `TLSMandatory`, `SMTPAuthPlain`, `DialAndSendWithContext`)
- [x] 4.3 Definir interfaz local `Logger` en `mailer.go` con métodos `Info`/`Warn`/`Error` (mismo shape que `httpclient.Logger`), usada de forma nil-checked, sin `fmt.Println` en ningún punto

## 5. Prueba end-to-end standalone

- [x] 5.1 Crear `infrastructure/mailer/mailer_test.go` con `TestRenderWelcomeEmail_NoSMTPRequired` (corre siempre, sin red)
- [x] 5.2 Agregar `TestSend_RealEmail_Integration` en el mismo archivo: construye el `Client` desde `config.MySMTP`, renderiza el template, envía un correo real a `GMAIL_USER`, con `t.Skip` automático si `config.MySMTP.User`/`AppPassword` están vacíos

## 6. Verificación de la capacidad standalone

- [x] 6.1 Ejecutar `go build ./...` y confirmar que compila sin errores
- [x] 6.2 Ejecutar `go vet ./...` y confirmar que no hay warnings
- [x] 6.3 Ejecutar `go test ./...` sin variables SMTP configuradas y confirmar que todo pasa (con `TestSend_RealEmail_Integration` mostrando `SKIP`)
- [ ] 6.4 Completar `GMAIL_USER`, `GMAIL_APP_PASSWORD`, `SMTP_HOST`, `SMTP_PORT` en `.env` con credenciales reales (contraseña de aplicación de Gmail, no la contraseña de la cuenta) — a cargo del usuario
- [ ] 6.5 Ejecutar `go test ./cmd/api/infrastructure/mailer/... -run TestSend_RealEmail_Integration -v` con credenciales reales y confirmar que pasa
- [ ] 6.6 Verificar manualmente en la bandeja de entrada de `GMAIL_USER` que el correo llegó, con el asunto correcto, el nombre interpolado y los colores de marca visibles

## 7. Conectar al flujo de registro

- [x] 7.1 Agregar `MailerInterface` (`Send` + `SendWelcomeEmail`) en `infrastructure/mailer/mailer.go`, implementado por `*Client`
- [x] 7.2 Agregar método `SendWelcomeEmail(ctx, to, name string) error` en `mailer.go` que renderiza `welcome.html` y llama a `Send`
- [x] 7.3 Modificar `services/auth_service.go`: agregar campo `mailer mailer.MailerInterface` a `authService`, actualizar `NewAuthService` para recibirlo, invocar `SendWelcomeEmail` tras crear el usuario en `Register` (nil-checked, error solo logueado, nunca bloquea el registro)
- [x] 7.4 Actualizar los 17 call-sites de `NewAuthService(mockDao)` en `auth_service_test.go` a `NewAuthService(mockDao, nil)`
- [x] 7.5 Wire en `app.go`: construir el mailer client con `config.MySMTP` (+ logger adapter reutilizado de `customlogger.NewHTTPClientLogger()`), inyectarlo en `services.NewAuthService(authDao, mailerClient)`
- [x] 7.6 Verificar `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 7.7 (Diferido) Escribir tests dedicados con mock de `MailerInterface`: `Register` invoca `SendWelcomeEmail` en caso feliz, `Register` sigue devolviendo éxito cuando `SendWelcomeEmail` falla
- [ ] 7.8 Con el backend corriendo y credenciales reales, registrar un usuario real vía `POST /api/v1/auth/register` y confirmar que llega el mail de bienvenida sin afectar la respuesta 201
