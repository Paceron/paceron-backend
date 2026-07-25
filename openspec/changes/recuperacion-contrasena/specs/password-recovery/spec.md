## ADDED Requirements

### Requirement: Solicitud de código de recuperación sin filtrar existencia de usuario
El sistema SHALL aceptar solicitudes POST a `/api/v1/auth/forgot-password` con un email, y SHALL responder siempre con el mismo mensaje genérico HTTP 200, exista o no el email registrado, y esté o no activo el usuario asociado.

#### Scenario: Email registrado y activo
- **WHEN** se envía una solicitud POST a `/api/v1/auth/forgot-password` con el email de un usuario existente y activo
- **THEN** el sistema genera un código numérico de 6 dígitos, lo persiste con expiración de 10 minutos, lo envía por mail, y responde HTTP 200 con el mensaje genérico

#### Scenario: Email no registrado
- **WHEN** se envía una solicitud POST a `/api/v1/auth/forgot-password` con un email que no corresponde a ningún usuario
- **THEN** el sistema responde exactamente el mismo HTTP 200 con el mismo mensaje genérico, sin enviar ningún mail

#### Scenario: Email de usuario no activo
- **WHEN** se envía una solicitud POST a `/api/v1/auth/forgot-password` con el email de un usuario en estado distinto de `active` (inactive, pause, blocked, suspended)
- **THEN** el sistema responde exactamente el mismo HTTP 200 con el mismo mensaje genérico, sin enviar ningún mail

#### Scenario: Solicitud previa pendiente invalidada por una nueva
- **WHEN** un usuario activo solicita un nuevo código mientras ya tiene uno pendiente (no expirado, no usado)
- **THEN** el sistema invalida el código anterior antes de generar y enviar el nuevo

### Requirement: Restablecimiento de contraseña vía código OTP
El sistema SHALL aceptar solicitudes POST a `/api/v1/auth/reset-password` con email, código, nueva contraseña y confirmación, y SHALL actualizar la contraseña únicamente si el código es válido, no expirado, no usado, y pertenece al email indicado.

#### Scenario: Restablecimiento exitoso
- **WHEN** se envía una solicitud POST a `/api/v1/auth/reset-password` con un código válido, no expirado, y una nueva contraseña que cumple las reglas de validación y coincide con su confirmación
- **THEN** el sistema actualiza la contraseña del usuario (hasheada con bcrypt), marca el código como usado, y responde HTTP 200

#### Scenario: Código incorrecto incrementa el contador de intentos
- **WHEN** se envía una solicitud POST a `/api/v1/auth/reset-password` con un código que no coincide con el código activo del usuario
- **THEN** el sistema incrementa el contador de intentos del código activo y responde HTTP 400 con un mensaje genérico de "código inválido o expirado"

#### Scenario: Código invalidado tras exceder el máximo de intentos
- **WHEN** el contador de intentos de un código llega a 5 intentos fallidos
- **THEN** el sistema invalida el código (no puede volver a usarse aunque no haya expirado) y cualquier intento posterior responde el mismo error genérico

#### Scenario: Código expirado
- **WHEN** se envía una solicitud POST a `/api/v1/auth/reset-password` con un código cuya expiración (10 minutos desde su creación) ya pasó
- **THEN** el sistema responde HTTP 400 con el mismo mensaje genérico de "código inválido o expirado"

#### Scenario: Email sin código activo o usuario inexistente/inactivo
- **WHEN** se envía una solicitud POST a `/api/v1/auth/reset-password` con un email sin ningún código pendiente, o que no corresponde a un usuario existente y activo
- **THEN** el sistema responde HTTP 400 con el mismo mensaje genérico de "código inválido o expirado" (indistinguible de un código incorrecto)

#### Scenario: Nueva contraseña y confirmación no coinciden
- **WHEN** se envía una solicitud POST a `/api/v1/auth/reset-password` con `new_password` distinto de `confirm_password`
- **THEN** el sistema responde HTTP 400 con un mensaje específico indicando que las contraseñas no coinciden (esta validación no distingue enumeración, es un error de input del cliente)

#### Scenario: Nueva contraseña no cumple las reglas de fortaleza
- **WHEN** se envía una solicitud POST a `/api/v1/auth/reset-password` con un código válido pero una `new_password` que no cumple las reglas de `ValidatePassword` (longitud, mayúscula, minúscula, dígito)
- **THEN** el sistema responde HTTP 400 con el mensaje de validación correspondiente, sin actualizar la contraseña
