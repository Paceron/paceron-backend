## ADDED Requirements

### Requirement: El sistema SHALL permitir asignar un rol a un usuario

El sistema SHALL aceptar solicitudes POST a `/api/v1/users/:id/roles` con `role_id` (requerido) y `tier_id` (opcional). Si no se provee `tier_id`, se asigna el tier por defecto "base" del rol asociado.

#### Scenario: Asignar rol con tier explícito
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` con `role_id` y `tier_id` válidos
- **THEN** el sistema persiste la asignación en la tabla `user_roles` y responde con HTTP 201 con `id`, `user_id`, `role_id`, `tier_id`, `assignment_date` y `status`

#### Scenario: Asignar rol con tier por defecto
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` con `role_id` válido y sin `tier_id`
- **THEN** el sistema busca el tier con nombre "base" para ese rol y lo asigna. Responde con HTTP 201

#### Scenario: Asignar rol ya asignado al usuario
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` con un `role_id` que ya está asignado activamente al usuario
- **THEN** el sistema responde con HTTP 409 y mensaje "El usuario ya tiene asignado este rol"

#### Scenario: Asignar rol a usuario inexistente
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` con un `id` de usuario que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Usuario no encontrado"

#### Scenario: Asignar rol inexistente
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` con un `role_id` que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Rol no encontrado"

#### Scenario: Asignar tier inexistente al rol
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` con un `role_id` y un `tier_id` que no pertenece a ese rol
- **THEN** el sistema responde con HTTP 400 y mensaje "El tier no pertenece al rol especificado"

#### Scenario: Asignar tier "base" inexistente para el rol
- **WHEN** se envía una solicitud POST a `/api/v1/users/:id/roles` sin `tier_id` y el rol no tiene un tier llamado "base"
- **THEN** el sistema responde con HTTP 400 y mensaje "El tier por defecto 'base' no existe para este rol"
