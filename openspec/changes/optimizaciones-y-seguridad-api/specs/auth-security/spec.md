# Spec Delta: auth-security (capability nueva)

> Cambio: `optimizaciones-y-seguridad-api`. Capability nueva — no hay spec principal previa.

## ADDED Requirements

### Requirement: Revocación de sesiones al cambiar la contraseña

El sistema DEBE cumplir lo siguiente (MUST):

- Cambiar la contraseña (autenticado) y restablecerla (flujo OTP) revoca TODOS los refresh tokens activos del usuario.
- Un refresh token previamente emitido queda inutilizable después de la revocación (rechazo con el error genérico existente).
- El access token vigente sigue válido hasta su expiración (JWT sin DB — limitación estructural conocida).

#### Scenario: Cambio de contraseña mata las sesiones

- **WHEN** el usuario cambia su contraseña
- **THEN** todos sus refresh tokens activos quedan revocados y un refresh posterior es rechazado

#### Scenario: Reset por OTP mata las sesiones

- **WHEN** el usuario restablece su contraseña con el código OTP
- **THEN** todos sus refresh tokens activos quedan revocados

### Requirement: Límite de tamaño de body

El sistema DEBE cumplir lo siguiente (MUST):

- Todo request pasa por un límite de tamaño de body global (middleware, antes del routing) — bodies gigantes son rechazados antes de ser bindeados/parseados.
- Los límites por-endpoint más estrictos ya existentes (fotos/ícono) se conservan.

#### Scenario: Body gigante rechazado temprano

- **WHEN** llega un request con body mayor al límite global
- **THEN** el request se rechaza (código 413 o el 400 del binding) sin cargar el body completo en memoria

### Requirement: Rate limit en rutas públicas

El sistema DEBE cumplir lo siguiente (MUST):

- Las rutas públicas sensibles (login, register, refresh, forgot-password, reset-password, auth/user) están sujetas a un rate limiter por IP en memoria.
- Excedido el límite → `429` con cuerpo de error genérico.
- Los webhooks de integración (Mercado Pago) y el websocket NO pasan por el limiter (las reentregas de MP no deben ser rechazadas).

#### Scenario: Brute force acotado

- **WHEN** una IP excede el límite de requests en las rutas públicas
- **THEN** recibe `429` hasta que su ventana se libere

#### Scenario: Webhooks sin limiter

- **WHEN** Mercado Pago reentrega un webhook varias veces seguidas
- **THEN** ninguna entrega es rechazada por el rate limiter
