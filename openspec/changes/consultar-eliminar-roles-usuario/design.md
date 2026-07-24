## Context

`daos/user_role_dao.go` ya expone `FindByUserID(ctx, userID) ([]dbs.UserRole, error)` y `SoftDelete(ctx, id) error`, sin ningún consumidor. `services/user_role_service.go` solo tiene `AssignRole`. El controller solo expone `AssignRole`. Se completa la capa de servicio/controller reusando el DAO existente, sin tocarlo.

## Goals / Non-Goals

**Goals:**
- Exponer `GetUserRoles`/`RemoveRole` reusando el DAO ya existente.
- Simetría con el POST: DELETE identifica por `role_id`, no por el id interno de la asignación.

**Non-Goals:**
- Cambiar el DAO o el modelo `UserRole`.
- Modificar `AssignRole`.

## Decisions

### 1. DELETE identifica por `role_id`, resolviendo el id interno vía `FindByUserAndRole`
**Por qué**: el cliente ya conoce `role_id` (lo usó para asignar el rol vía POST) — no tiene por qué conocer el id interno de la fila `user_roles`. `RemoveRole` internamente llama `FindByUserAndRole(userID, roleID)` (ya existe, usado hoy solo para detectar duplicados en `AssignRole`) para resolver el id, y recién ahí `SoftDelete(id)`.
**Alternativa descartada**: `DELETE /api/v1/users/:id/roles/:assignment_id` — más directo (un solo lookup en el DAO), pero obliga al cliente a guardar/conocer un id interno que no forma parte de su modelo mental (el cliente piensa en "sacarle el rol X", no en "borrar la asignación con id Y").

### 2. Sin cambios en `daos/user_role_dao.go`
**Por qué**: `FindByUserID` y `SoftDelete` ya cubren exactamente lo necesario. Agregar código al DAO sin necesidad violaría el principio de cambio mínimo.

## Risks / Trade-offs

- `RemoveRole` hace 2 queries (`FindByUserAndRole` + `SoftDelete`) en vez de 1 — aceptable, no es un endpoint de alta frecuencia, y evita exponer el id interno de la asignación al cliente.
