## ADDED Requirements

### Requirement: Cambiar contraseña estando autenticado
El sistema SHALL aceptar solicitudes PATCH a `/api/v1/users/:id/password` con contraseña actual, nueva contraseña y confirmación, y SHALL actualizar la contraseña únicamente si la contraseña actual es correcta.

#### Scenario: Cambio exitoso
- **WHEN** se envía una solicitud PATCH a `/api/v1/users/:id/password` con la contraseña actual correcta y una nueva contraseña válida que coincide con su confirmación
- **THEN** el sistema actualiza la contraseña del usuario (hasheada con bcrypt), setea `password_changed_at` a la fecha/hora actual, y responde HTTP 200

#### Scenario: Contraseña actual incorrecta
- **WHEN** se envía una solicitud PATCH a `/api/v1/users/:id/password` con una contraseña actual que no coincide con el hash almacenado
- **THEN** el sistema responde HTTP 401 sin modificar la contraseña

#### Scenario: Usuario inexistente
- **WHEN** se envía una solicitud PATCH a `/api/v1/users/:id/password` para un `:id` que no corresponde a ningún usuario
- **THEN** el sistema responde HTTP 404

#### Scenario: Nueva contraseña y confirmación no coinciden
- **WHEN** `new_password` es distinto de `confirm_password`
- **THEN** el sistema responde HTTP 400, sin llegar a verificar la contraseña actual contra la base

#### Scenario: Nueva contraseña no cumple las reglas de fortaleza
- **WHEN** `new_password` no cumple las reglas de `ValidatePassword` (longitud, mayúscula, minúscula, dígito)
- **THEN** el sistema responde HTTP 400, sin modificar la contraseña

#### Scenario: Nueva contraseña igual a la actual
- **WHEN** `new_password` es igual a la contraseña actual del usuario
- **THEN** el sistema responde HTTP 400 con un mensaje indicando que la nueva contraseña debe ser distinta a la actual, sin modificar nada

### Requirement: password_changed_at se puebla desde cualquier camino de cambio de contraseña
El sistema SHALL setear `password_changed_at` tanto al cambiar la contraseña estando autenticado como al completar el flujo de recuperación por OTP (`reset-password`).

#### Scenario: Reset por OTP también setea el campo
- **WHEN** un usuario completa exitosamente el flujo `POST /api/v1/auth/reset-password`
- **THEN** el sistema setea `password_changed_at` a la fecha/hora actual, igual que en el cambio autenticado
