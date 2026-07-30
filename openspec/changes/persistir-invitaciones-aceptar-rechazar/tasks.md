## 1. Modelo y constantes

- [x] 1.1 `domains/constants/invitation_status.go`: `InvitationStatusPending/Accepted/Rejected`
- [x] 1.2 `domains/dbs/invitation.go`: modelo `Invitation`
- [x] 1.3 Agregar `&dbs.Invitation{}` a `AutoMigrate` en `infrastructure/postgresdb/postgres.go`

## 2. DAO

- [x] 2.1 `daos/invitation_dao.go`: `Create`, `FindByID`, `FindPendingByTeamAndInvitee`, `FindPendingByTeamID`, `UpdateStatus`
- [x] 2.2 Tests mínimos (`daos/invitation_dao_test.go`), mismo patrón que `team_user_dao_test.go`

## 3. DTOs

- [x] 3.1 `domains/invitation/invitation_request.go`: `RespondInvitationRequest{UserID}`
- [x] 3.2 `domains/invitation/invitation_response.go`: `InvitationResponse`, `RespondInvitationResponse`

## 4. Service

- [x] 4.1 `InviteRunner`: agregar chequeo "¿ya es miembro?" (409) y "¿ya tiene invitación pendiente?" (409), persistir antes de mandar el mail
- [x] 4.2 `ListPendingInvitations(teamID)`: filtra vencidas (chequeo perezoso)
- [x] 4.3 `AcceptInvitation(invitationID, userID)`: valida pertenencia/estado/expiración, crea `TeamUser` (rol corredor) antes de marcar aceptada, no duplica alta si ya es miembro
- [x] 4.4 `RejectInvitation(invitationID, userID)`: misma validación, solo actualiza estado
- [x] 4.5 Tests en `services/invitation_service_test.go` para los 4 métodos (éxito, not found, wrong user, already responded, expired, dao errors)

## 5. Controller

- [x] 5.1 `ListPendingInvitations`, `AcceptInvitation`, `RejectInvitation` en `controllers/invitation_controller.go`
- [x] 5.2 Mapeo de errores a HTTP status (404/403/409/500)
- [x] 5.3 Tests en `controllers/invitation_controller_test.go`

## 6. Rutas y wiring

- [x] 6.1 `app/app.go`: `invitationDao := daos.NewInvitationDao(db)`, pasar a `NewInvitationService`
- [x] 6.2 `app/url_mappings.go`: `GET /api/v1/teams/:id/invitations`, `POST /api/v1/invitations/:id/accept`, `POST /api/v1/invitations/:id/reject`

## 7. Documentación

- [x] 7.1 Regenerar Swagger
- [x] 7.2 Actualizar tabla de endpoints en `README.md`
- [x] 7.3 Requests Bruno (`team bruno collections/18_listar_invitaciones_pendientes.yml`, `19_aceptar_invitacion.yml`, `20_rechazar_invitacion.yml`)

## 8. Verificación

- [x] 8.1 `go build ./...`, `go vet ./...`, `go test ./...` — todo verde
- [ ] 8.2 Probar manualmente end-to-end (invitar, listar pendientes, aceptar, confirmar alta en `team_users`, rechazar otra)
