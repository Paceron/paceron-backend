## ADDED Requirements

<!--
No hay requirement de "consultar roles" (GET) — evaluado y descartado en review
(2026-07-24): redundante con GET /api/v1/auth/permissions?user_id=, que ya
expone role_id (lo necesario para el DELETE de abajo) con más contexto
(nombre de rol/tier/permisos). Ver design.md, Decisión 3.
-->

### Requirement: Dar de baja un rol asignado a un usuario
El sistema SHALL aceptar solicitudes DELETE a `/api/v1/users/:id/roles/:role_id` y SHALL invalidar (soft-delete) la asignación activa de ese rol para ese usuario, identificando por `role_id`.

#### Scenario: Baja exitosa
- **WHEN** se envía una solicitud DELETE a `/api/v1/users/:id/roles/:role_id` para una asignación activa existente
- **THEN** el sistema marca la asignación como eliminada lógicamente y responde HTTP 200 con un mensaje de confirmación (mismo patrón que `DeleteRoleResponse`, no 204 sin cuerpo)

#### Scenario: Rol no asignado al usuario
- **WHEN** se envía una solicitud DELETE a `/api/v1/users/:id/roles/:role_id` para un rol que el usuario no tiene asignado activamente
- **THEN** el sistema responde HTTP 404 con un mensaje indicando que el usuario no tiene asignado ese rol

### Requirement: El rol "corredor" no se puede dar de baja
El sistema SHALL rechazar cualquier solicitud DELETE a `/api/v1/users/:id/roles/:role_id` donde `role_id` corresponda al rol "corredor", sin importar el usuario ni si el rol está efectivamente asignado.

#### Scenario: Intento de dar de baja el rol corredor
- **WHEN** se envía una solicitud DELETE a `/api/v1/users/:id/roles/:role_id` donde `role_id` corresponde al rol "corredor"
- **THEN** el sistema responde HTTP 403 sin realizar ninguna baja, con un mensaje indicando que es el rol base de todo usuario

#### Scenario: Baja de otro rol no afecta a corredor
- **WHEN** un usuario tiene asignados tanto "corredor" como otro rol (ej. "entrenador"), y se da de baja el otro rol
- **THEN** la asignación de "corredor" permanece activa y sin cambios, y el usuario en sí (`dbs.User`) tampoco se modifica
