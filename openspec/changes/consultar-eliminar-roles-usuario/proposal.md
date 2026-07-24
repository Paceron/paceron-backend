## Why

`seccion-permisos` implementó la asignación de roles a usuarios (`POST /api/v1/users/:id/roles`), pero deliberadamente no incluyó consultar qué roles tiene un usuario ni darlos de baja — el DAO (`UserRoleDaoInterface`) ya expone `FindByUserID` y `SoftDelete`, pero nunca se usaron desde un endpoint. Sin esto, un rol asignado por error no se puede revertir sin acceso directo a la base de datos.

## What Changes

- Nuevo endpoint `DELETE /api/v1/users/:id/roles/:role_id`: da de baja (soft-delete) la asignación de un rol a un usuario, identificando por `role_id` (simétrico al POST existente).
- Se agrega 1 método a `UserRoleServiceInterface`/`UserRoleController` ya existentes — no se crean capas nuevas de DAO.
- **`GET /api/v1/users/:id/roles` se evaluó y se descartó** (revisión del encargado del backend, 2026-07-24) — ver "Decisión: por qué no hay GET" más abajo.

## Capabilities

### New Capabilities
<!-- No aplica -->

### Modified Capabilities
- `user-role-assignment` (de `seccion-permisos`): se amplía con consulta y baja de roles asignados; la asignación (POST) no cambia.

## Impact

- **Modificado**: `services/user_role_service.go` (+ test), `controllers/user_role_controller.go` (+ test)
- **Rutas nuevas**: `app/url_mappings.go`
- **Sin cambios**: `daos/user_role_dao.go` (ya tenía los métodos necesarios), modelos GORM, `app/app.go` (el servicio/controller ya estaban wireados)
- **Swagger**: 1 endpoint nuevo, regenerar docs

### Decisión: por qué no hay GET

El plan original incluía `GET /api/v1/users/:id/roles`. El encargado del backend marcó en review que es redundante con `GET /api/v1/auth/permissions?user_id=` (de `seccion-permisos`), que ya usa el mismo `userRoleDao.FindByUserID` internamente y devuelve, por cada rol, su `id` (el `role_id`, justo lo que necesita el `DELETE` de este mismo cambio), nombre de rol, nombre de tier y permisos — un superset más útil de lo que este `GET` iba a exponer.

Lo único que este `GET` tenía de más eran campos crudos sin consumidor real: `assignment_id` interno (`UserRole.ID`), `tier_id` sin resolver (vs. nombre de tier), `assignment_date` y `status`. Ningún cliente/feature de hoy necesita esos campos — no había backing real para justificarlo, así que se sacó. Queda solo el `DELETE`, que sí es capacidad nueva genuina (no existía en ningún lado). Si en el futuro aparece un consumidor real que necesite esos campos crudos, se reevalúa entonces.

### Alcance

- `DELETE /api/v1/users/:id/roles/:role_id`
- Listar roles de un usuario se hace vía el endpoint ya existente `GET /api/v1/auth/permissions?user_id=`, no se duplica acá.

### No alcance

- Modificar el endpoint de asignación (`POST`) — queda igual.
- Hard-delete de asignaciones — se mantiene soft-delete, mismo criterio que el resto de las entidades de `seccion-permisos`.
- Auto-asignación de "corredor" en el registro — no existe hoy, queda fuera de este cambio (el rol solo se protege de ser removido, no se asigna automáticamente).

### Métrica de éxito

- Se puede dar de baja un rol asignado por error sin acceso directo a la base de datos.
- El rol "corredor" nunca puede darse de baja vía la API, sin importar el usuario.
- No queda un endpoint nuevo duplicando lo que `GET /api/v1/auth/permissions?user_id=` ya resuelve.
