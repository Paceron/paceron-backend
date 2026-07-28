## Context

El backend no tiene ningún mecanismo de recuperación de contraseña. Login y registro ya existen (`services/auth_service.go`), con hash de contraseñas vía `bcrypt` y validación de fortaleza vía `ValidatePassword`. La infraestructura de mailer (`infrastructure/mailer/`) ya envía el email de bienvenida en el registro — se reusa el mismo cliente y el mismo patrón de opciones funcionales para el email de recuperación.

El usuario definió explícitamente: código OTP numérico de 6 dígitos por mail (no un link con token), y una tabla dedicada `password_reset_tokens` (no columnas nuevas en `users`), siguiendo el mismo criterio ya usado para `Role`/`UserRole` (relaciones en tablas propias, no en el modelo `User`).

## Goals / Non-Goals

**Goals:**
- Flujo completo de dos pasos: pedir código (`forgot-password`) y canjearlo por una contraseña nueva (`reset-password`).
- Protección contra enumeración de usuarios: la respuesta de `forgot-password` es indistinguible entre email existente/inexistente/inactivo.
- Protección contra fuerza bruta del código: expiración corta (10 min) + límite de intentos (5).
- Reuso total de: `ValidatePassword`, el cliente de mailer, el patrón de DAO con soft-delete, el patrón de servicio con mailer nil-checked.

**Non-Goals:**
- Link de recuperación vía URL/deep-link — descartado a favor de OTP.
- Rate limiting de requests a nivel de IP — se cubre con expiración + límite de intentos sobre el código, no con throttling de red.
- Reintentos automáticos de envío de mail — mismo criterio que `SendWelcomeEmail` (best-effort, un solo intento).
- Notificación adicional de "contraseña cambiada" tras un reset exitoso.

## Decisions

### 1. Tabla dedicada `password_reset_tokens`, no columnas en `users`
**Por qué**: mismo criterio arquitectónico que `Role`/`UserRole` en este repo — relaciones/eventos con su propio ciclo de vida (expiración, intentos, uso) no ensucian el modelo principal. Permite múltiples registros históricos (auditoría) sin sobrescribir campos en `User`.
**Alternativa descartada**: `reset_token_hash` + `reset_token_expires_at` en `dbs.User` — más simple pero solo permite un estado "pendiente" a la vez sin historial, y mezcla concerns de autenticación transitoria con el perfil persistente del usuario.

### 2. Servicio nuevo y dedicado (`password_reset_service.go`), no dentro de `authService`
**Por qué**: `AuthServiceInterface` hoy tiene 3 métodos enfocados (`Register`, `Login`, `GetUser`). Agregar 2 métodos más con una dependencia de DAO distinta (`PasswordResetDaoInterface`) empieza a sobrecargar una interfaz que hoy es chica y estable. Sigue el mismo criterio que `seccion-permisos` (un servicio por concern: `permission_service.go`, `tier_service.go`, etc.) en vez de amontonar responsabilidades no relacionadas en un solo servicio.
**Alternativa descartada**: agregar `RequestPasswordReset`/`ResetPassword` a `authService` — descartada por la razón de arriba; también aumenta el riesgo de regresión sobre `Login`/`Register` al tocar el mismo archivo.

### 3. Hash del código con `bcrypt`, no `sha256`
**Por qué**: un código de 6 dígitos tiene baja entropía (1.000.000 de combinaciones) — un hash rápido (sha256) sería trivialmente reversible por fuerza bruta offline si la tabla se filtrara alguna vez. Reusar `bcrypt` (mismo mecanismo que `User.Password`) mantiene consistencia con el resto del repo y evita la pregunta de por qué esta tabla usa un hash distinto. El costo de cómputo de bcrypt no es un problema de performance dado que es un endpoint de baja frecuencia.
**Alternativa descartada**: `sha256` simple — más rápido pero sin beneficio real dado que el límite de intentos (decisión 4) ya acota el riesgo de fuerza bruta online, y offline bcrypt es la defensa correcta.

### 4. Límite de 5 intentos fallidos por código + expiración de 10 minutos
**Por qué**: sin límite de intentos, un código de 6 dígitos es adivinable por fuerza bruta automatizada dentro de la ventana de expiración. Con máximo 5 intentos, la probabilidad de acertar por azar es de ~5 en 1.000.000 — impracticable. 10 minutos es suficiente para que un usuario real revise su mail y tipee el código, pero corto para acotar la ventana de intentos online.
**Alternativa descartada**: expiración más larga (30-60 min, común en flujos de link) — no aplica igual a un OTP tipeado a mano, que se consume casi inmediatamente tras recibirse; una ventana larga solo aumenta la superficie de ataque sin beneficio de UX real.

### 5. Un solo código activo por usuario — se invalidan los anteriores al pedir uno nuevo
**Por qué**: evita ambigüedad sobre "contra qué código se cuentan los intentos" si un usuario pide varios códigos seguidos, y invalida códigos viejos que pudieran haber quedado expuestos (pantalla compartida, mail reenviado, etc.).
**Implementación**: `SoftDeleteByUserID` (mismo patrón `gorm.Expr("NOW()")` que `UserRoleDao.SoftDelete`) antes de crear el nuevo registro.

### 6. `reset-password` colapsa todos los errores de validación de código a un único mensaje genérico
**Por qué**: código incorrecto, expirado, ya usado, usuario inexistente o usuario inactivo deben responder exactamente igual (`"código inválido o expirado"`, HTTP 400). Si se distinguieran (ej. 404 para "usuario no existe" vs 400 para "código incorrecto"), el endpoint se convierte en un oráculo de enumeración — contradice el objetivo de `forgot-password` de no filtrar esa información. Esto es una **excepción deliberada** al patrón de `auth_controller.go`, donde sí se usan mensajes/status distintos por condición (ej. `usuario no encontrado` → 404) — ahí no hay riesgo de enumeración porque esos endpoints no son parte de un flujo de recuperación.
**Nota**: los errores de validación de *input* del cliente (contraseñas no coinciden, contraseña débil) sí pueden ser específicos — no son información sensible sobre la cuenta.

### 7. Usuarios no-`active` no pueden resetear su contraseña
**Por qué**: mismo chequeo que ya hace `Login`. Permitir que un usuario `blocked`/`suspended` recupere acceso vía "olvidé mi contraseña" anularía el propósito de esos estados (ej. bloqueado por fraude). Se aplica el mismo principio de no-enumeración: la respuesta de `forgot-password` es igual de genérica para un usuario inactivo que para uno inexistente.

### 8. Sin variables de entorno nuevas — constantes hardcodeadas
**Por qué**: OTP-only no necesita `FRONTEND_BASE_URL` ni nada configurable por entorno; expiración y límite de intentos son decisiones de seguridad fijas, no configuración de despliegue. Sigue el patrón ya usado (`defaultTierName = "base"` en `user_role_service.go`) de constantes de negocio en código en vez de env vars innecesarias.

## Risks / Trade-offs

- **Sin rate limiting de red en `forgot-password`**: un atacante podría spamear pedidos de código a una dirección de email ajena (molestia, no compromiso de seguridad, ya que el código sigue protegido). Mitigación parcial: cada pedido nuevo invalida el anterior, así que solo el último código importa. Si se vuelve un problema real, se evalúa throttling en un cambio futuro — explícitamente fuera de alcance acá.
- **bcrypt en un path de autenticación no crítico añade latencia menor** (~100ms) tanto al generar como al validar el código — aceptable dado que no es un endpoint de alta frecuencia.
- **Un usuario que pierde acceso a su email no tiene forma de recuperar la cuenta** — limitación inherente a cualquier flujo de recuperación por email, no específica de esta implementación.
