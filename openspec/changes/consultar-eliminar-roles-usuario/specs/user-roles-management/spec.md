## ADDED Requirements

### Requirement: Consultar los roles asignados a un usuario
El sistema SHALL aceptar solicitudes GET a `/api/v1/users/:id/roles` y SHALL responder con la lista de asignaciones de rol activas (no eliminadas lógicamente) del usuario indicado.

#### Scenario: Usuario con roles asignados
- **WHEN** se envía una solicitud GET a `/api/v1/users/:id/roles` para un usuario con una o más asignaciones activas
- **THEN** el sistema responde HTTP 200 con la lista de asignaciones (id, role_id, tier_id, assignment_date, status)

#### Scenario: Usuario sin roles asignados
- **WHEN** se envía una solicitud GET a `/api/v1/users/:id/roles` para un usuario sin ninguna asignación activa
- **THEN** el sistema responde HTTP 200 con una lista vacía (no HTTP 404)

### Requirement: Dar de baja un rol asignado a un usuario
El sistema SHALL aceptar solicitudes DELETE a `/api/v1/users/:id/roles/:role_id` y SHALL invalidar (soft-delete) la asignación activa de ese rol para ese usuario, identificando por `role_id`.

#### Scenario: Baja exitosa
- **WHEN** se envía una solicitud DELETE a `/api/v1/users/:id/roles/:role_id` para una asignación activa existente
- **THEN** el sistema marca la asignación como eliminada lógicamente y responde HTTP 200 con un mensaje de confirmación (mismo patrón que `DeleteRoleResponse`, no 204 sin cuerpo)

#### Scenario: Rol no asignado al usuario
- **WHEN** se envía una solicitud DELETE a `/api/v1/users/:id/roles/:role_id` para un rol que el usuario no tiene asignado activamente
- **THEN** el sistema responde HTTP 404 con un mensaje indicando que el usuario no tiene asignado ese rol
