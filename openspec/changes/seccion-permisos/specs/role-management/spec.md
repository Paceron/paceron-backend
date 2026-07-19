## ADDED Requirements

### Requirement: El sistema SHALL permitir crear un rol

El sistema SHALL aceptar solicitudes POST a `/api/v1/roles` con un nombre (requerido) y una descripción (opcional). El nombre del rol SHALL ser único en el sistema.

#### Scenario: Crear rol con datos válidos
- **WHEN** se envía una solicitud POST a `/api/v1/roles` con `name` válido y opcionalmente `description`
- **THEN** el sistema persiste el rol en la tabla `roles` y responde con HTTP 201 y el rol creado con su `id`, `name`, `description` y `created_at`

#### Scenario: Crear rol con nombre duplicado
- **WHEN** se envía una solicitud POST a `/api/v1/roles` con un `name` que ya existe en la tabla `roles` (no eliminado lógicamente)
- **THEN** el sistema responde con HTTP 409 y mensaje "El nombre del rol ya existe"

#### Scenario: Crear rol sin nombre
- **WHEN** se envía una solicitud POST a `/api/v1/roles` sin campo `name` o con `name` vacío
- **THEN** el sistema responde con HTTP 400 y mensaje "El nombre es requerido"

### Requirement: El sistema SHALL permitir actualizar un rol

El sistema SHALL aceptar solicitudes PUT a `/api/v1/roles/:id` con campos opcionales `name` y `description`.

#### Scenario: Actualizar rol con datos válidos
- **WHEN** se envía una solicitud PUT a `/api/v1/roles/:id` con campos a actualizar
- **THEN** el sistema actualiza el rol y responde con HTTP 200 y el rol actualizado con su `id`, `name`, `description` y `updated_at`

#### Scenario: Actualizar rol inexistente
- **WHEN** se envía una solicitud PUT a `/api/v1/roles/:id` con un `id` que no existe o está eliminado lógicamente
- **THEN** el sistema responde con HTTP 404 y mensaje "Rol no encontrado"

#### Scenario: Actualizar nombre a uno duplicado
- **WHEN** se envía una solicitud PUT a `/api/v1/roles/:id` con un `name` que ya existe en otro rol
- **THEN** el sistema responde con HTTP 409 y mensaje "El nombre del rol ya existe"

### Requirement: El sistema SHALL permitir eliminar un rol lógicamente

El sistema SHALL aceptar solicitudes DELETE a `/api/v1/roles/:id` y realizar un soft delete estableciendo `deleted_at`.

#### Scenario: Eliminar rol existente
- **WHEN** se envía una solicitud DELETE a `/api/v1/roles/:id` con un `id` válido
- **THEN** el sistema establece `deleted_at` en el registro y responde con HTTP 200 y mensaje "Rol eliminado correctamente"

#### Scenario: Eliminar rol inexistente
- **WHEN** se envía una solicitud DELETE a `/api/v1/roles/:id` con un `id` que no existe
- **THEN** el sistema responde con HTTP 404 y mensaje "Rol no encontrado"
