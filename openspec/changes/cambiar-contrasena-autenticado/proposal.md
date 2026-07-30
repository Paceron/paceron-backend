## Why

El único flujo de cambio de contraseña que existe hoy es forgot/reset (OTP por mail, pensado para cuando el usuario no puede loguearse). Falta el caso normal: cambiar la contraseña desde el perfil, con sesión activa, sabiendo la contraseña actual.

## What Changes

- Nuevo endpoint `PATCH /api/v1/users/:id/password`: recibe contraseña actual + nueva + confirmación, verifica la actual (mismo patrón que `X-Current-Password` de `UserController.Update`), valida fortaleza de la nueva, la persiste.
- Nueva columna `password_changed_at` (nullable) en `dbs.User` — se setea en este flujo **y también** en el flujo existente de forgot/reset (`password_reset_service.ResetPassword`), para que el dato quede completo desde cualquier camino de cambio de contraseña.
- No se agrega ningún middleware ni enforcement de sesiones — ver design.md.

## Capabilities

### New Capabilities
- `change-password`: cambio de contraseña autenticado, verificando la contraseña actual.

### Modified Capabilities
- `password-recovery` (de `recuperacion-contrasena`): `ResetPassword` también setea `password_changed_at`, sin cambiar su contrato/comportamiento externo.

## Impact

- **Nuevo**: `domains/user/change_password_request.go`, `change_password_response.go`
- **Modificado**: `domains/dbs/user.go` (columna nueva), `services/user_service.go` (+test), `controllers/user_controller.go` (+test), `services/password_reset_service.go` (+test, una línea), `app/url_mappings.go`
- **Swagger**: 1 endpoint nuevo, regenerar docs
- **Sin cambios**: `app/app.go` (userService ya tiene todo lo que necesita inyectado)

### Objetivo

Permitir que un usuario logueado cambie su contraseña desde el perfil, sabiendo la actual, sin pasar por el flujo de recuperación por mail.

### Alcance

- `PATCH /api/v1/users/:id/password`
- Columna `password_changed_at`, poblada desde este endpoint y desde el reset por OTP existente

### No alcance

- Middleware de validación de JWT / cualquier enforcement de sesión sobre `password_changed_at` — se dejó explícitamente para más adelante (ver design.md). Hoy la app no valida el JWT en ningún endpoint, este cambio no lo introduce.
- Endpoint `/refresh` o `/logout` — `/logout` es la próxima feature, después de esta, fuera de este cambio.
- Notificación por mail al cambiar contraseña — no pedido.

### Métrica de éxito

- Un usuario logueado puede cambiar su contraseña sabiendo la actual, sin pasar por el flujo OTP.
- `password_changed_at` queda poblado sea cual sea el camino usado para cambiar la contraseña (perfil o recuperación).
