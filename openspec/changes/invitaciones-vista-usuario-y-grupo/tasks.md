## 1. Modelo y DTOs

- [x] 1.1 `domains/dbs/invitation.go`: campo `GroupID *int64` (columna aditiva)
- [x] 1.2 `InviteRunnerRequest`: `GroupID *int64` opcional
- [x] 1.3 `InvitationResponse`: `TeamName string`, `GroupID *int64`

## 2. DAO

- [x] 2.1 `daos/invitation_dao.go`: `FindPendingByInviteeID(ctx, inviteeID)`

## 3. Service

- [x] 3.1 `InviteRunner`: valida `group_id` opcional contra el equipo (`groupDao.FindByIDAndTeamID`), 404 si no existe
- [x] 3.2 `AcceptInvitation`: llama `assignInviteeToGroup` (grupo de la invitación o principal del equipo), no bloqueante
- [x] 3.3 `ListPendingInvitationsForUser(ctx, userID)`: invitaciones pendientes de un usuario, filtra vencidas
- [x] 3.4 `GetInvitationDetail(ctx, invitationID, userID)`: detalle, valida ownership, sin restricción de estado
- [x] 3.5 `toInvitationResponse` helper compartido (evita duplicar el enriquecimiento invitee/team en 3 lugares)
- [x] 3.6 Tests: `InviteRunner` con `group_id` válido/inválido, `ListPendingInvitationsForUser` (éxito/vencidas/error), `GetInvitationDetail` (éxito/not found/wrong user), `AcceptInvitation` asignación de grupo (con `group_id`, sin `group_id` con/sin grupo principal, ya miembro del grupo)

## 4. Controller

- [x] 4.1 `ListMyInvitations`: `GET /api/v1/invitations?user_id=`
- [x] 4.2 `GetInvitationByID`: `GET /api/v1/invitations/:id?user_id=`
- [x] 4.3 Mapeo de error nuevo en `InviteRunner`: "el grupo no existe en este equipo" → 404
- [x] 4.4 Tests para los 2 endpoints nuevos + el error nuevo de `InviteRunner`

## 5. Wiring y rutas

- [x] 5.1 `app/app.go`: `invitationService` gana `groupDao`/`groupUserDao`
- [x] 5.2 `app/url_mappings.go`: `GET /api/v1/invitations`, `GET /api/v1/invitations/:id`

## 6. Documentación

- [x] 6.1 Regenerar Swagger
- [x] 6.2 Actualizar tabla de endpoints en `README.md`
- [x] 6.3 Requests Bruno (`24_invitar_usuario_a_grupo_especifico.yml`, `25_listar_mis_invitaciones.yml`, `26_detalle_invitacion.yml`)

## 7. Verificación

- [x] 7.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 7.2 Probar manualmente: invitar con `group_id`, listar mis invitaciones, ver detalle, aceptar y confirmar alta en `group_users`
