## Why

Cuando un usuario da de baja su cuenta (`PATCH /api/v1/users/:id/status` con `status: "inactive"`, desde el flujo self-service "Borrar cuenta" del frontend), el sistema no envía ninguna confirmación por correo. La infraestructura de mailer ya existe (`agregar-envio-mails-smtp`) y solo está conectada al flujo de registro; se necesita reutilizarla para confirmar la desactivación.

## What Changes

- Nuevo template HTML de despedida (`templates/farewell.html`), embebido en el binario, mismo sistema visual que `welcome.html`
- Nuevo método `RenderFarewellEmail` en `infrastructure/mailer/render.go`
- Nuevo método `SendFarewellEmail` agregado a `MailerInterface` e implementado en `*Client`
- `services/user_service.go`: `userService` gana dependencia `mailer.MailerInterface`; `ChangeStatus` envía el mail de despedida cuando el usuario transiciona a `inactive` (best-effort, no bloqueante, con guarda contra reenvíos redundantes en transiciones `inactive → inactive`)
- `app/app.go`: se reordena la construcción del mailer para que ocurra antes del flujo de usuario, y se inyecta en `NewUserService`

## Capabilities

### New Capabilities
- `farewell-mailer`: Envío de correo de despedida al desactivar una cuenta de usuario, conectado a `PATCH /api/v1/users/:id/status`.

### Modified Capabilities
- `user-status-change`: además de cambiar el estado, ahora intenta enviar un email de despedida cuando la transición es hacia `inactive` (best-effort, no bloqueante).

## Impact

- **Modificado**: `infrastructure/mailer/mailer.go`, `infrastructure/mailer/render.go`, `services/user_service.go`, `app/app.go`
- **Nuevo**: `infrastructure/mailer/templates/farewell.html`
- **Tests**: `user_service_test.go` y `opt_service_test.go` (15 call-sites de `NewUserService` actualizados a `NewUserService(mockDao, nil)`); tests dedicados con mock de `MailerInterface` quedan diferidos, mismo criterio que `agregar-envio-mails-smtp`
- **Swagger**: no aplica (no cambia el contrato de `PATCH /api/v1/users/:id/status`)
- **Nota de seguridad conocida, fuera de alcance**: este endpoint no tiene autenticación/autorización propia; el mail de despedida se disparará para cualquier caller que lo invoque, exactamente igual que el resto del comportamiento actual del endpoint. No se corrige acá.
