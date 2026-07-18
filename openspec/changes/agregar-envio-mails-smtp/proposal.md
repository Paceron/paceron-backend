## Why

El backend de Paceron no tenía ninguna capacidad de envío de correos electrónicos. La historia de usuario de registro exige "confirmación de alta con notificación inmediata (mensaje en pantalla + email de bienvenida)", así que se necesita esta infraestructura como base, conectada al flujo de registro existente.

## What Changes

- Nueva capacidad de infraestructura para enviar correos vía SMTP usando una cuenta de Gmail
- Nuevo paquete `infrastructure/mailer/` con cliente SMTP (opciones funcionales, mismo patrón que `httpclient`)
- Nuevo template HTML de bienvenida (`templates/welcome.html`) embebido en el binario, con los colores de marca de Paceron
- Nueva configuración `SMTP` en `config/config.go`, leída de variables de entorno (`GMAIL_USER`, `GMAIL_APP_PASSWORD`, `SMTP_HOST`, `SMTP_PORT`)
- Nueva dependencia: `github.com/wneessen/go-mail`
- Test de integración (`mailer_test.go`) que envía un correo real de prueba, skippeado automáticamente sin credenciales
- Actualización de `.env.example` con los nuevos placeholders de SMTP
- Conexión al flujo de registro (`POST /api/v1/auth/register`): tras crear el usuario exitosamente, se intenta enviar el email de bienvenida. Un fallo de envío se loguea pero **no** bloquea el alta del usuario (la respuesta sigue siendo 201)

## Capabilities

### New Capabilities
- `smtp-mailer`: Envío de correos electrónicos vía SMTP (Gmail) con soporte de templates HTML, conectado al flujo de registro de usuarios como notificación de bienvenida.

### Modified Capabilities
- `user-registration`: además de crear el usuario, ahora intenta enviar un email de bienvenida (best-effort, no bloqueante).

## Impact

- **Nuevo paquete de infraestructura**: `infrastructure/mailer/` (mailer.go, options.go, render.go, templates/welcome.html, mailer_test.go)
- **Nueva configuración**: `config.SMTP` / `config.MySMTP`, función `loadSMTPConfig()`
- **Nueva dependencia**: `github.com/wneessen/go-mail`
- **Nuevas variables de entorno**: `GMAIL_USER`, `GMAIL_APP_PASSWORD`, `SMTP_HOST`, `SMTP_PORT`
- **Modificado**: `services/auth_service.go` (inyección de `mailer.MailerInterface`, envío en `Register`), `app/app.go` (wiring del mailer)
- **Tests**: `mailer_test.go` (render + envío real skippeable), `config_test.go` (nuevos casos para `loadSMTPConfig`), `auth_service_test.go` (call-sites actualizados; mocks dedicados de `MailerInterface` quedan para una iteración posterior)
- **Swagger**: no aplica (no hay endpoint HTTP nuevo, `POST /api/v1/auth/register` no cambia su contrato)
