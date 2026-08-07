## 1. Infraestructura compartida

- [x] 1.1 `utils/authcontext.go`: claves de contexto (`AuthUserIDKey`, `AuthSessionIDKey`, `AuthRolesKey`) + `GetAuthUserID(c)`
- [x] 1.2 `app/middleware.go`: `AuthMiddleware()` usa las claves de `utils` en vez de constantes locales duplicadas

## 2. Rutas

- [x] 2.1 `url_mappings.go`: rutas públicas (register/login/refresh/logout/forgot/reset-password/auth-user/legacy/demo/swagger/guide) registradas antes de `r.Use(AuthMiddleware())`
- [x] 2.2 Todo el resto de las rutas (users, roles, tiers, permissions, teams, groups, team-users, group-users, invitations) registradas después — quedan protegidas

## 3. Solo-entrenador

- [x] 3.1 `team_service.Update/UpdateAddress`: agregar `callerID` + `isEntrenadorOfTeam` helper
- [x] 3.2 `team_service.Create`: `ownerID` como parámetro explícito, ya no `req.OwnerID`
- [x] 3.3 `group_service.Create/Update`: agregar `callerID` + chequeo entrenador del equipo del grupo
- [x] 3.4 `team_user_service.AddUser`: agregar `callerID` + chequeo entrenador
- [x] 3.5 `group_user_service.AddUser`: agregar `callerID` + `teamUserDao` nuevo en el constructor + chequeo entrenador
- [x] 3.6 `invitation_service.InviteRunner`: agregar `callerID`, `InviterID` pasa a ser `callerID` (antes `team.OwnerID` sin verificar)
- [x] 3.8 `invitation_service.ListPendingInvitations`: agregar `callerID` + chequeo entrenador (confirmado sin ningún chequeo antes)
- [x] 3.9 `team_user_service.GetUsersByTeam` / `group_user_service.GetUsersByGroup`: agregar `callerID` + chequeo de membresía (cualquier rol, no solo entrenador) — confirmado sin ningún chequeo antes

## 10. Activación/desactivación de entrenador (addendum post-revisión)

- [x] 10.1 `user_role_controller.AssignRole`/`RemoveRole`: agregar chequeo self-only (confirmado sin ningún chequeo antes — cualquiera gestionaba roles de cualquiera)
- [x] 10.2 `userrole.ActivateEntrenadorRequest` (password + bank_alias opcional) nuevo DTO
- [x] 10.3 `bankAliasRegex`/mensaje de formato extraídos de `user_service.go` a variables de paquete, reutilizados por la nueva validación (evita duplicar el regex)
- [x] 10.4 `user_role_service.ActivateEntrenador`: valida password (bcrypt), exige bank_alias propio o provisto, persiste el alias si viene en el body, reutiliza `AssignRole` internamente (busca el rol por nombre vía `roleDao.FindByName`)
- [x] 10.5 `user_role_service.DeactivateEntrenador`: bloquea si el usuario lidera (`RoleInTeam == "entrenador"`) algún equipo activo (`teamUserDao.FindByUserID`), si no reutiliza `RemoveRole`
- [x] 10.6 `userRoleService` gana `teamUserDao` como dependencia nueva — reordenado `app.go` para construir `teamUserDao` antes de `userRoleService`
- [x] 10.7 Controller: `ActivateEntrenador`/`DeactivateEntrenador`, ambos self-only, mapeo de errores a 400/401/403/404/409
- [x] 10.8 Rutas `POST/DELETE /api/v1/users/:id/entrenador-role`
- [x] 10.9 Tests: service (éxito, cada validación, cada error de DAO) y controller (éxito, forbidden, cada mapeo de error) para ambos métodos nuevos, más self-only en AssignRole/RemoveRole existentes
- [x] 10.10 Swagger regenerado, README y `docs/AUTH_MIGRATION.md` actualizados
- [x] 10.11 Verificación manual contra la DB de staging: asignar rol a otro usuario → 403; activar sin alias → 400; activar con password incorrecta → 401; activar OK → 201; crear equipo ahora permitido; desactivar liderando equipo activo → 409; borrar el equipo y reintentar desactivar → 200
- [x] 3.7 `team_delegate.CreateTeam`: propaga `ownerID` a `teamSvc.Create` y `groupSvc.Create`

## 4. Self o entrenador delegado

- [x] 4.1 `team_user_service.RemoveUser`: agregar `callerID` + `targetUserID`, self-o-entrenador
- [x] 4.2 `group_user_service.RemoveUser`: agregar `callerID` + `targetUserID`, self-o-entrenador (vía `teamUserDao` del equipo del grupo)

## 5. Self-only

- [x] 5.1 `user_controller.Update/ChangeStatus/ChangePassword`: chequeo `auth_user_id == :id` inline en el controller, 403 si no coincide

## 6. Controllers: fuente de identidad

- [x] 6.1 `team_controller`: `Create` usa `ownerID` del token; `Update/UpdateAddress` usan `callerID`; `Delete` usa `authUserID` en vez de `?user_id=`
- [x] 6.2 `group_controller`: `Create/Update` usan `callerID`; `Delete` usa `authUserID`; `GetAll` usa `authUserID` cuando filtra por `team_id` (ya no `?user_id=`)
- [x] 6.3 `team_user_controller`: `AddUser` pasa `callerID`; `RemoveUser` pasa `callerID` + `targetUserID`
- [x] 6.4 `group_user_controller`: ídem
- [x] 6.5 `invitation_controller`: `InviteRunner` pasa `callerID`; `ListMyInvitations`/`GetInvitationByID` usan `authUserID` (ya no `?user_id=`); `AcceptInvitation`/`RejectInvitation` usan `authUserID` (ya no body)

## 7. Domains

- [x] 7.1 `team.CreateTeamRequest`: quitar `OwnerID`
- [x] 7.2 `invitation.RespondInvitationRequest`: eliminar (ya no se usa)

## 8. Tests

- [x] 8.1 Reescribir tests de `team_service`, `group_service`, `team_user_service`, `group_user_service`, `invitation_service` para las nuevas firmas
- [x] 8.2 Agregar casos de autorización nuevos: not-entrenador (403) por cada patrón solo-entrenador; self-vs-delegado (self permitido, tercero sin rol rechazado, entrenador permitido) por cada patrón self-o-delegado
- [x] 8.3 Reescribir tests de `team_delegate`
- [x] 8.4 Reescribir tests de `team_controller`, `group_controller`, `team_user_controller`, `group_user_controller`, `invitation_controller`, `user_controller` (self-only) usando `setAuthUserID(c, ...)` en vez de query params/body
- [x] 8.5 `go build ./...` / `go vet ./...` / `go test ./...` verdes

## 9. Docs

- [x] 9.1 Regenerar Swagger
- [x] 9.2 Actualizar tabla de endpoints en `README.md` (marca 🔓 en públicos, notas de autorización)
- [x] 9.3 Crear `docs/AUTH_MIGRATION.md` para el frontend
- [x] 9.4 Verificación manual: 401 sin header en ruta protegida, 200 con token válido, `owner_id`/`?user_id=` del cliente ignorados (se resuelven del token), self-o-delegado (self OK, tercero sin rol 403, entrenador OK), self-only en users (self OK, otro usuario 403) — todo contra la DB de staging
