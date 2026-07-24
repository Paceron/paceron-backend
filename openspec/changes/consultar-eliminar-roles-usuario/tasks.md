## 1. Servicio

- [x] ~~1.1 Agregar `GetUserRoles`~~ — revertido, ver tarea 5
- [x] 1.2 Agregar `RemoveRole(ctx, userID, roleID int64) error` a `UserRoleServiceInterface` y su implementación: `FindByUserAndRole` (nil → "el usuario no tiene asignado este rol"), luego `SoftDelete(assignment.ID)`
- [x] 1.3 Tests en `services/user_role_service_test.go`: `RemoveRole` éxito, rol no asignado, error de DAO
- [x] 1.4 Bloquear la baja del rol "corredor" (rol base de todo usuario, decisión del usuario 2026-07-24): constante `protectedRoleName = "corredor"`, chequeo en `RemoveRole` vía `roleDao.FindByID` antes de buscar la asignación. Tests: rol protegido rechazado, error de `roleDao.FindByID`

## 2. Controller

- [x] ~~2.1 Agregar `GetRoles`~~ — revertido, ver tarea 5
- [x] 2.2 Agregar `RemoveRole(c *gin.Context)`: parsea `:id` y `:role_id`, llama `RemoveRole`, 200 con `RemoveRoleResponse{Message}` en éxito (mismo patrón que `DeleteRoleResponse`, no 204), 404 si no estaba asignado
- [x] 2.3 Tests en `controllers/user_role_controller_test.go` para `RemoveRole`
- [x] 2.4 Mapear el error de rol protegido a HTTP 403 (Forbidden), distinto del 404 de "no asignado". Test dedicado

## 3. Rutas y documentación

- [x] 3.1 Agregar `DELETE /api/v1/users/:id/roles/:role_id` en `app/url_mappings.go`
- [x] 3.2 Regenerar Swagger, actualizar tabla de endpoints en `README.md`
- [x] 3.3 Agregar request a las colecciones Bruno (`remove role v1`)

## 4. Verificación

- [x] 4.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [x] 4.2 Probar manualmente (curl, 2026-07-24): asignar rol, listar (`GET /api/v1/users/:id/roles` — versión previa a la tarea 5), dar de baja, listar de nuevo (ya no aparece), intentar baja de "corredor" (403), confirmar user sin cambios. Flujo confirmado funcionando

## 5. Revertir GET redundante (feedback de review, 2026-07-24)

- [x] 5.1 Sacar `GetUserRoles` de `UserRoleServiceInterface`/`userRoleService` (`services/user_role_service.go`) y sus tests
- [x] 5.2 Sacar `GetRoles` de `UserRoleController`/`userRoleController` (`controllers/user_role_controller.go`) y sus tests
- [x] 5.3 Sacar la ruta `GET /api/v1/users/:id/roles` de `app/url_mappings.go`
- [x] 5.4 Sacar el request Bruno `get user roles v1` (ambas carpetas)
- [x] 5.5 Actualizar `README.md`, regenerar Swagger, actualizar `proposal.md`/`design.md`/`specs/*/spec.md` documentando la decisión y por qué no hay backing real para mantenerlo
- [x] 5.6 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde tras el revert
