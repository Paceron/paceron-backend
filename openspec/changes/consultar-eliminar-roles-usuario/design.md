## Context

`daos/user_role_dao.go` ya expone `FindByUserID(ctx, userID) ([]dbs.UserRole, error)` y `SoftDelete(ctx, id) error`, sin ningún consumidor. `services/user_role_service.go` solo tiene `AssignRole`. El controller solo expone `AssignRole`. Se completa la capa de servicio/controller reusando el DAO existente, sin tocarlo.

## Goals / Non-Goals

**Goals:**
- Exponer `RemoveRole` reusando el DAO ya existente.
- Simetría con el POST: DELETE identifica por `role_id`, no por el id interno de la asignación.

**Non-Goals:**
- Cambiar el DAO o el modelo `UserRole`.
- Modificar `AssignRole`.
- Exponer un `GET` de listado — ver Decisión 3 (evaluado y descartado, redundante con `/api/v1/auth/permissions?user_id=`).

## Decisions

### 1. DELETE identifica por `role_id`, resolviendo el id interno vía `FindByUserAndRole`
**Por qué**: el cliente ya conoce `role_id` (lo usó para asignar el rol vía POST) — no tiene por qué conocer el id interno de la fila `user_roles`. `RemoveRole` internamente llama `FindByUserAndRole(userID, roleID)` (ya existe, usado hoy solo para detectar duplicados en `AssignRole`) para resolver el id, y recién ahí `SoftDelete(id)`.
**Alternativa descartada**: `DELETE /api/v1/users/:id/roles/:assignment_id` — más directo (un solo lookup en el DAO), pero obliga al cliente a guardar/conocer un id interno que no forma parte de su modelo mental (el cliente piensa en "sacarle el rol X", no en "borrar la asignación con id Y").

### 2. Sin cambios en `daos/user_role_dao.go`
**Por qué**: `FindByUserID` y `SoftDelete` ya cubren exactamente lo necesario. Agregar código al DAO sin necesidad violaría el principio de cambio mínimo.

### 3. Sin endpoint GET de listado — se reusa `/api/v1/auth/permissions?user_id=`
**Decisión revisada** (feedback del encargado del backend en review, 2026-07-24): el plan original tenía `GET /api/v1/users/:id/roles`. Se comparó con `permissions_query_service.GetUserPermissions` (endpoint `/api/v1/auth/permissions?user_id=`, de `seccion-permisos`) y se confirmó que usa el mismo `userRoleDao.FindByUserID` como fuente de datos, y devuelve por cada rol su `id` (= `role_id`, el dato que el cliente necesita para llamar al `DELETE` de este cambio), más nombre de rol/tier/permisos resueltos — estrictamente más útil que lo que el `GET` descartado iba a exponer.
**Por qué se descarta igual**: los únicos campos que el `GET` tenía de más — `assignment_id` (`UserRole.ID` interno), `tier_id` sin resolver, `assignment_date`, `status` — no tienen ningún consumidor hoy. Mantenerlo sería una API duplicada sin backing real.
**Alternativa descartada**: mantenerlo igual, justificando por los campos crudos — se descarta porque "puede servir en el futuro" no es una razón suficiente hoy (YAGNI); si aparece un consumidor real de esos campos, se reevalúa.

### 4. El rol "corredor" no se puede dar de baja — hardcodeado por nombre
**Por qué**: "corredor" es el rol base que todo usuario de la app tiene por defecto (decisión del usuario, 2026-07-24) — no tiene sentido de negocio que se pueda remover. Se hardcodea el nombre como constante (`protectedRoleName = "corredor"`), mismo criterio que `defaultTierName = "base"` ya usado en este archivo — no se agrega un flag "protegido" al modelo `Role` porque hoy solo existe un caso concreto, y agregar schema/migración para un único valor sería sobre-ingeniería.
**Alternativa descartada**: columna `is_protected` en `dbs.Role` — más flexible a futuro (soportaría más roles protegidos sin tocar código), pero requiere migración y tocar los endpoints de gestión de roles (`role_controller.go`) para exponerlo/setearlo; se descarta por ahora, se puede migrar a esto si aparece un segundo rol protegido.
**Nota**: el chequeo va en el service (`RemoveRole`), antes de buscar la asignación — corta rápido sin necesidad de que el rol esté siquiera asignado al usuario. Mapea a HTTP 403 (Forbidden) en el controller, distinto del 404 de "no asignado".

## Risks / Trade-offs

- `RemoveRole` hace 2-3 queries (`FindByID` del rol + `FindByUserAndRole` + `SoftDelete`) en vez de 1 — aceptable, no es un endpoint de alta frecuencia, y evita exponer el id interno de la asignación al cliente.
- Si en el futuro se necesita más de un rol protegido, hardcodear nombres uno por uno escala mal — migrar a un flag en el modelo si eso pasa (ver alternativa descartada arriba).
