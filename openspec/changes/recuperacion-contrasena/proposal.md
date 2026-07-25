## Why

El backend de Paceron tiene login y registro implementados, pero no hay forma de que un usuario recupere el acceso a su cuenta si olvida la contraseña — hoy la única opción es contactar soporte manualmente. Se necesita un flujo de autoservicio de recuperación de contraseña, reusando la infraestructura de mailer ya existente (conectada hoy solo al registro).

## What Changes

- Nuevo endpoint `POST /api/v1/auth/forgot-password`: recibe un email, genera un código numérico de 6 dígitos, lo persiste con expiración de 10 minutos, y lo envía por mail. Responde siempre el mismo mensaje genérico, exista o no el email, para no filtrar qué emails están registrados.
- Nuevo endpoint `POST /api/v1/auth/reset-password`: recibe email + código + nueva contraseña + confirmación, valida el código y actualiza la contraseña del usuario.
- Nueva tabla `password_reset_tokens` (modelo GORM nuevo, no se tocan columnas de `users`).
- Nuevo servicio dedicado `password_reset_service.go` (no se mezcla con `authService`).
- Nuevo DAO `password_reset_dao.go`.
- Nuevo controller `password_reset_controller.go`.
- Nuevo método de mailer `SendPasswordResetEmail` + template `templates/reset.html`.
- Sin variables de entorno nuevas (el código se tipea a mano, no hay link ni base URL).

## Capabilities

### New Capabilities
- `password-recovery`: Recuperación de contraseña vía código OTP de 6 dígitos enviado por mail, con expiración, límite de intentos, y protección contra enumeración de usuarios.

### Modified Capabilities
<!-- No se modifican capacidades existentes; se reusa infraestructura de mailer sin cambiar su contrato actual (SendWelcomeEmail no cambia). -->

## Impact

- **Nuevo modelo**: `domains/dbs/password_reset_token.go`
- **Nuevo DAO**: `daos/password_reset_dao.go` (+ test)
- **Nuevo servicio**: `services/password_reset_service.go` (+ test)
- **Nuevo controller**: `controllers/password_reset_controller.go` (+ test)
- **Mailer**: `infrastructure/mailer/mailer.go`, `render.go` (nuevos métodos), `templates/reset.html` (nuevo)
- **Migración**: `infrastructure/postgresdb/postgres.go` (agregar `PasswordResetToken` a `AutoMigrate`)
- **Wiring**: `app/app.go` (inyección de las nuevas capas), `app/url_mappings.go` (2 rutas nuevas)
- **Swagger**: nuevos endpoints, regenerar docs
- **Sin cambios de API pública existente**: login/register no cambian de contrato

### Objetivo

Permitir que un usuario recupere el acceso a su cuenta de forma autónoma cuando olvida su contraseña, sin intervención de soporte, manteniendo el mismo nivel de seguridad que el resto del flujo de autenticación.

### Alcance

- Endpoints `POST /api/v1/auth/forgot-password` y `POST /api/v1/auth/reset-password`.
- Modelo, DAO, servicio, controller y mailer nuevos para sostener el flujo OTP.

### No alcance

- Recuperación de contraseña vía link/token en URL — se descartó a favor de código OTP (decisión del usuario).
- Rate limiting a nivel de IP/red para `forgot-password` — se mitiga con expiración corta (10 min) y límite de intentos (5) sobre el código, no con throttling de requests. Si el volumen de abuso lo justifica, se evalúa en un cambio futuro.
- Notificación de "tu contraseña fue cambiada" post-reset — no pedido, se puede sumar después reusando el mismo mailer.
- Cambios a `authService`/`Login`/`Register` — quedan intactos.
- **Pendiente anotado para una iteración futura**: mejorar el estilo visual de los mails (bienvenida y recuperación) para incluir el logo de Paceron y la tipografía de marca correcta — hoy usan solo el nombre "Paceron" en texto plano con los colores de marca, sin logo ni fuente custom. No se hace en este cambio.

### Métrica de éxito

- Un usuario que olvida su contraseña puede recuperar el acceso completo (pedir código, recibirlo por mail, resetear, loguear con la nueva contraseña) sin intervención manual.
- El endpoint `forgot-password` nunca revela si un email está registrado o no.
- Un código no puede ser adivinado por fuerza bruta dentro de su ventana de validez (bloqueo a los 5 intentos fallidos).
