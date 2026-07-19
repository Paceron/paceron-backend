## ADDED Requirements

### Requirement: El sistema SHALL permitir asignar un permiso a un tier

El sistema SHALL aceptar solicitudes POST a `/api/v1/tiers/:id/permissions` con `permission_id` (requerido). El permiso SHALL existir y no estar eliminado lógicamente.

#### Scenario: Asignar permiso a tier con datos válidos
- **WHEN** se envía una solicitud POST a `/api/v1/tiers/:id/permissions` con `permission_id` válido
- **THEN** el sistema persiste la asignación en la tabla `tier_permissions` y responde con HTTP 201 con `id`, `tier_id`, `permission_id` y `asignation_date`

#### Scenario: Asignar permiso ya existente al tier
- **WHEN** se envía una solicitud POST a `/api/v1/tiers/:id/permissions` con un `permission_id` que ya está asignado a ese tier
- **THEN** el sistema responde con HTTP 409 y mensaje "El permiso ya está asignado a este tier"

#### Scenario: Asignar permiso a tier inexistente
- **WHEN** se envía una solicitud POST a `/api/v1/tiers/:id/permissions` con un `id` de tier que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Tier no encontrado"

#### Scenario: Asignar permiso inexistente a tier
- **WHEN** se envía una solicitud POST a `/api/v1/tiers/:id/permissions` con un `permission_id` que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Permiso no encontrado"

### Requirement: El sistema SHALL permitir desasignar un permiso de un tier

El sistema SHALL aceptar solicitudes DELETE a `/api/v1/tiers/:id/permissions/:permission_id` y realizar un soft delete.

#### Scenario: Desasignar permiso existente del tier
- **WHEN** se envía una solicitud DELETE a `/api/v1/tiers/:id/permissions/:permission_id` con una asignación válida
- **THEN** el sistema establece `deleted_at` en el registro y responde con HTTP 200 y mensaje "Permiso desasignado del tier correctamente"

#### Scenario: Desasignar permiso no asignado al tier
- **WHEN** se envía una solicitud DELETE a `/api/v1/tiers/:id/permissions/:permission_id` con una asignación que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Asignación no encontrada"
