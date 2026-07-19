## ADDED Requirements

### Requirement: El sistema SHALL permitir crear un permiso

El sistema SHALL aceptar solicitudes POST a `/api/v1/permissions` con un nombre (requerido) y una descripción (opcional). El nombre del permiso SHALL ser único en el sistema.

#### Scenario: Crear permiso con datos válidos
- **WHEN** se envía una solicitud POST a `/api/v1/permissions` con `name` válido y opcionalmente `description`
- **THEN** el sistema persiste el permiso en la tabla `permissions` y responde con HTTP 201 y el permiso creado con su `id`, `name`, `description` y `created_at`

#### Scenario: Crear permiso con nombre duplicado
- **WHEN** se envía una solicitud POST a `/api/v1/permissions` con un `name` que ya existe en la tabla `permissions` (no eliminado lógicamente)
- **THEN** el sistema responde con HTTP 409 y mensaje "El nombre del permiso ya existe"

#### Scenario: Crear permiso sin nombre
- **WHEN** se envía una solicitud POST a `/api/v1/permissions` sin campo `name` o con `name` vacío
- **THEN** el sistema responde con HTTP 400 y mensaje "El nombre es requerido"

### Requirement: El sistema SHALL permitir actualizar un permiso

El sistema SHALL aceptar solicitudes PUT a `/api/v1/permissions/:id` con campos opcionales `name` y `description`.

#### Scenario: Actualizar permiso con datos válidos
- **WHEN** se envía una solicitud PUT a `/api/v1/permissions/:id` con campos a actualizar
- **THEN** el sistema actualiza el permiso y responde con HTTP 200 y el permiso actualizado con su `id`, `name`, `description` y `updated_at`

#### Scenario: Actualizar permiso inexistente
- **WHEN** se envía una solicitud PUT a `/api/v1/permissions/:id` con un `id` que no existe o está eliminado lógicamente
- **THEN** el sistema responde con HTTP 404 y mensaje "Permiso no encontrado"

#### Scenario: Actualizar nombre a uno duplicado
- **WHEN** se envía una solicitud PUT a `/api/v1/permissions/:id` con un `name` que ya existe en otro permiso
- **THEN** el sistema responde con HTTP 409 y mensaje "El nombre del permiso ya existe"

### Requirement: El sistema SHALL permitir eliminar un permiso lógicamente

El sistema SHALL aceptar solicitudes DELETE a `/api/v1/permissions/:id` y realizar un soft delete estableciendo `deleted_at`.

#### Scenario: Eliminar permiso existente
- **WHEN** se envía una solicitud DELETE a `/api/v1/permissions/:id` con un `id` válido
- **THEN** el sistema establece `deleted_at` en el registro y responde con HTTP 200 y mensaje "Permiso eliminado correctamente"

#### Scenario: Eliminar permiso inexistente
- **WHEN** se envía una solicitud DELETE a `/api/v1/permissions/:id` con un `id` que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Permiso no encontrado"
