## Why

`infrastructure/mailer` envía correos vía SMTP crudo contra Gmail (puerto 587, usuario + app password). En producción (Render) esto produce timeouts intermitentes de conexión TCP (`dial tcp ...: i/o timeout`) desde el egress compartido del PaaS — sin retry ni timeout explícito, un fallo transitorio de red rompe el envío. El fix quedó a cargo del usuario en esta sesión (no de un compañero, como se pensaba antes).

En vez de parchear el SMTP actual con retry/timeout manual, se migra a **Resend** (proveedor transaccional con API HTTP, Bearer token). Al ser HTTP en vez de SMTP, el problema de raíz desaparece: se convierte en una llamada HTTPS más, como cualquier otra que ya hace el backend (ej. `exampleweatherclient`), y reutiliza la resiliencia (retry/timeout) que `infrastructure/httpclient` ya tiene — sin escribir nada nuevo.

## What Changes

- `infrastructure/mailer.Client` deja de envolver un cliente SMTP (`go-mail`) y pasa a envolver `infrastructure/httpclient.Client`, apuntando a `https://api.resend.com`.
- `Send` arma el body JSON de Resend (`from`, `to`, `subject`, `html`, logo embebido como `attachments[].content_id`) en vez de `DialAndSendWithContext`.
- `Option`s `WithHost`/`WithPort`/`WithCredentials` se reemplazan por `WithAPIKey`/`WithFrom`; `WithLogger` ahora toma `httpclient.Logger` en vez de un tipo local duplicado.
- `config.SMTP`/`MySMTP`/`loadSMTPConfig()` se reemplazan por `config.MailerConfig`/`MyMailer`/`loadMailerConfig()`, leyendo `RESEND_API_KEY`/`RESEND_FROM_ADDRESS`.
- Se elimina la dependencia `github.com/wneessen/go-mail`.
- `MailerInterface` (`Send`, `SendEmail`), los 4 `EmailType` existentes y el patrón best-effort en los call sites (`auth_service`, `user_service`, `password_reset_service`, `invitation_service`) **no cambian** — mismo contrato, ningún consumidor se entera del swap.
- Env vars: `.env.example`, `render.yaml` (ambos services), `docs/ENVIRONMENTS.md` — sacar `GMAIL_USER`/`GMAIL_APP_PASSWORD`/`SMTP_HOST`/`SMTP_PORT`, agregar `RESEND_API_KEY`/`RESEND_FROM_ADDRESS`.

## Capabilities

### Modified Capabilities
- `smtp-mailer`: el mecanismo de transporte pasa de SMTP a la API HTTP de Resend. El contrato hacia el resto del sistema (`MailerInterface`, templates, best-effort) no cambia.

## Impact

- **Modificado**: `infrastructure/mailer/{mailer.go,options.go,mailer_test.go}`, `config/{config.go,config_test.go}`, `app/app.go`.
- **Eliminado**: dependencia `github.com/wneessen/go-mail`.
- **Nuevas variables de entorno**: `RESEND_API_KEY`, `RESEND_FROM_ADDRESS` (reemplazan `GMAIL_USER`/`GMAIL_APP_PASSWORD`/`SMTP_HOST`/`SMTP_PORT`).
- **Tests**: reescritos los de construcción del cliente y el de integración real (ahora contra Resend); los de renderizado de templates no cambian (no dependen del transporte).
- **Swagger**: no aplica, sin endpoints HTTP nuevos ni modificados.
- **Acción manual del usuario**: cuenta Resend + dominio verificado + `RESEND_API_KEY` cargada en Render (ambos services) y `.env` local.
