## Why

`seccion-permisos` implementó la asignación de roles a usuarios (`POST /api/v1/users/:id/roles`), pero deliberadamente no incluyó consultar qué roles tiene un usuario ni darlos de baja — el DAO (`UserRoleDaoInterface`) ya expone `FindByUserID` y `SoftDelete`, pero nunca se usaron desde un endpoint. Sin esto, un rol asignado por error no se puede revertir sin acceso directo a la base de datos.

## What Changes

- Nuevo endpoint `GET /api/v1/users/:id/roles`: lista los roles activos de un usuario.
- Nuevo endpoint `DELETE /api/v1/users/:id/roles/:role_id`: da de baja (soft-delete) la asignación de un rol a un usuario, identificando por `role_id` (simétrico al POST existente).
- Se agregan 2 métodos a `UserRoleServiceInterface`/`UserRoleController` ya existentes — no se crean capas nuevas de DAO.

## Capabilities

### New Capabilities
<!-- No aplica -->

### Modified Capabilities
- `user-role-assignment` (de `seccion-permisos`): se amplía con consulta y baja de roles asignados; la asignación (POST) no cambia.

## Impact

- **Modificado**: `services/user_role_service.go` (+ test), `controllers/user_role_controller.go` (+ test)
- **Rutas nuevas**: `app/url_mappings.go`
- **Sin cambios**: `daos/user_role_dao.go` (ya tenía los métodos necesarios), modelos GORM, `app/app.go` (el servicio/controller ya estaban wireados)
- **Swagger**: 2 endpoints nuevos, regenerar docs

### Objetivo

Permitir consultar y revertir asignaciones de rol de un usuario, completando el ciclo CRUD que `seccion-permisos` dejó parcial a propósito.

### Alcance

- `GET /api/v1/users/:id/roles`
- `DELETE /api/v1/users/:id/roles/:role_id`

### No alcance

- Modificar el endpoint de asignación (`POST`) — queda igual.
- Hard-delete de asignaciones — se mantiene soft-delete, mismo criterio que el resto de las entidades de `seccion-permisos`.

### Métrica de éxito

- Se puede consultar la lista de roles activos de cualquier usuario.
- Se puede dar de baja un rol asignado por error sin acceso directo a la base de datos.
