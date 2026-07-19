## ADDED Requirements

### Requirement: El sistema SHALL permitir crear un tier asociado a un rol

El sistema SHALL aceptar solicitudes POST a `/api/v1/tiers` con `name` (requerido), `description` (opcional) y `role_id` (requerido). El nombre del tier SHALL ser único dentro del mismo rol.

#### Scenario: Crear tier con datos válidos
- **WHEN** se envía una solicitud POST a `/api/v1/tiers` con `name`, `role_id` válido y opcionalmente `description`
- **THEN** el sistema persiste el tier en la tabla `tiers` con el `role_name` obtenido del rol asociado, y responde con HTTP 201 y el tier creado con su `id`, `name`, `description`, `role_id`, `role_name` y `created_at`

#### Scenario: Crear tier con nombre duplicado en el mismo rol
- **WHEN** se envía una solicitud POST a `/api/v1/tiers` con un `name` que ya existe para el mismo `role_id`
- **THEN** el sistema responde con HTTP 409 y mensaje "Ya existe un tier con ese nombre para este rol"

#### Scenario: Crear tier con role_id inexistente
- **WHEN** se envía una solicitud POST a `/api/v1/tiers` con un `role_id` que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Rol no encontrado"

#### Scenario: Crear tier sin campos requeridos
- **WHEN** se envía una solicitud POST a `/api/v1/tiers` sin `name` o sin `role_id`
- **THEN** el sistema responde con HTTP 400 y mensaje "El nombre y el role_id son requeridos"

### Requirement: El sistema SHALL permitir actualizar un tier

El sistema SHALL aceptar solicitudes PUT a `/api/v1/tiers/:id` con campos opcionales `name` y `description`.

#### Scenario: Actualizar tier con datos válidos
- **WHEN** se envía una solicitud PUT a `/api/v1/tiers/:id` con campos a actualizar
- **THEN** el sistema actualiza el tier y responde con HTTP 200 y el tier actualizado con su `id`, `name`, `description`, `role_id`, `role_name` y `updated_at`

#### Scenario: Actualizar tier inexistente
- **WHEN** se envía una solicitud PUT a `/api/v1/tiers/:id` con un `id` que no existe o está eliminado lógicamente
- **THEN** el sistema responde con HTTP 404 y mensaje "Tier no encontrado"

#### Scenario: Actualizar nombre a uno duplicado en el mismo rol
- **WHEN** se envía una solicitud PUT a `/api/v1/tiers/:id` con un `name` que ya existe para el mismo rol
- **THEN** el sistema responde con HTTP 409 y mensaje "Ya existe un tier con ese nombre para este rol"

### Requirement: El sistema SHALL permitir eliminar un tier lógicamente

El sistema SHALL aceptar solicitudes DELETE a `/api/v1/tiers/:id` y realizar un soft delete estableciendo `deleted_at`.

#### Scenario: Eliminar tier existente
- **WHEN** se envía una solicitud DELETE a `/api/v1/tiers/:id` con un `id` válido
- **THEN** el sistema establece `deleted_at` en el registro y responde con HTTP 200 y mensaje "Tier eliminado correctamente"

#### Scenario: Eliminar tier inexistente
- **WHEN** se envía una solicitud DELETE a `/api/v1/tiers/:id` con un `id` que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Tier no encontrado"
