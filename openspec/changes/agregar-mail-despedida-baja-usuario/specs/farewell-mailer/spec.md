## ADDED Requirements

### Requirement: Template HTML de despedida parametrizado
El sistema SHALL proveer un template HTML embebido de correo de despedida, parametrizado con el nombre del usuario, usando los colores de marca de Paceron.

#### Scenario: Renderizado exitoso con nombre
- **WHEN** se invoca `RenderFarewellEmail` con `FarewellEmailData{ Name: "Juan" }`
- **THEN** el HTML resultante SHALL contener el nombre "Juan" interpolado en el saludo

#### Scenario: Auto-escaping de caracteres especiales
- **WHEN** se invoca `RenderFarewellEmail` con un nombre que contiene caracteres HTML especiales
- **THEN** el HTML resultante SHALL tener esos caracteres escapados, sin permitir inyección de HTML/JS

### Requirement: Envío de email de despedida al desactivar una cuenta
El sistema SHALL intentar enviar un email de despedida al usuario cuando su estado transiciona a `inactive` vía `PATCH /api/v1/users/:id/status`, sin que un fallo de envío afecte la respuesta del cambio de estado.

#### Scenario: Transición a inactive dispara el envío
- **WHEN** un usuario con estado distinto de `inactive` es actualizado a `status=inactive` exitosamente
- **THEN** el sistema SHALL invocar `SendFarewellEmail` con el email y nombre del usuario

#### Scenario: Fallo de envío no bloquea el cambio de estado
- **WHEN** la transición a `inactive` es exitosa pero el envío del email de despedida falla
- **THEN** el sistema SHALL responder igualmente `200 OK` con el nuevo estado, y SHALL loguear el error de envío sin exponerlo en la respuesta

#### Scenario: Transición redundante no reenvía el correo
- **WHEN** un usuario que ya tiene `status=inactive` recibe nuevamente `PATCH /api/v1/users/:id/status` con `{"status": "inactive"}`
- **THEN** el sistema SHALL responder `200 OK` (comportamiento idempotente existente) pero NO SHALL invocar `SendFarewellEmail`

#### Scenario: Transición a un estado distinto de inactive no envía el correo
- **WHEN** un usuario cambia de estado a `pause`, `blocked`, `suspended` o `active`
- **THEN** el sistema SHALL NOT invocar `SendFarewellEmail`
