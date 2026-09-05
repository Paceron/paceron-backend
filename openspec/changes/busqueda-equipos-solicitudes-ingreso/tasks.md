# Tasks

## Tasks

### 1. Modelos de DB y migración

- [ ] Agregar `Visible bool` (`gorm:"column:visible;not null;default:true"`) e `IsPublic bool` (`gorm:"column:is_public;not null;default:true"`) a `cmd/api/domains/dbs/team.go`.
- [ ] Crear `cmd/api/domains/dbs/join_request.go` (`JoinRequest`, tabla `join_requests`, `Status string` reusando los valores de `constants.InvitationStatus`).
- [ ] Confirmar que `dbs.JoinRequest{}` queda en la lista de AutoMigrate (`cmd/api/infrastructure/postgresdb/postgres.go`).
- [ ] `go build ./...` verde.

### 2. Extracciones compartidas (targeted refactor)

- [ ] Crear `cmd/api/services/team_group_assignment.go`: `AssignToDefaultGroup(ctx *gin.Context, groupDao daos.GroupDaoInterface, groupUserDao daos.GroupUserDaoInterface, teamID int64, groupID *int64, userID int64)`, moviendo la lógica de `invitation_service.go:474` (`assignInviteeToGroup`).
- [ ] Actualizar `invitation_service.go` (`AcceptInvitation`) para llamar `AssignToDefaultGroup(ctx, s.groupDao, s.groupUserDao, inv.TeamID, inv.GroupID, userID)` en vez del método propio; eliminar `assignInviteeToGroup`.
- [ ] Test de `AssignToDefaultGroup` (mocks de `GroupDaoInterface`/`GroupUserDaoInterface`): con `groupID` explícito, sin `groupID` (cae a `IsMain`), sin grupo default (loguea y no rompe), ya es miembro del grupo (no duplica).
- [ ] Confirmar que los tests existentes de `AcceptInvitation` siguen verdes sin cambios de comportamiento observable.

### 3. DAOs

- [ ] `daos/join_request_dao.go` (+ `_test.go` contra Postgres real vía `testutils.SetupTestDB`): `Create`, `FindByID`, `FindPendingByTeamAndUser(teamID, runnerID)`, `FindPendingByTeam(teamID, page, pageSize)`, `FindByUser(runnerID)`, `UpdateStatus(id, status)`, `CountPendingByOwner(ownerID)`.
- [ ] Extender `daos/team_dao.go`: `SearchPublic(filters TeamSearchFilters, callerID int64, page, pageSize int) ([]dbs.Team, bool, error)` — filtra `visible = true`, `deleted_at IS NULL`, excluye equipos donde `callerID` ya es miembro activo, pide `pageSize + 1` filas para derivar `has_more` sin `COUNT(*)`.
- [ ] Mocks de `TeamDaoInterface` actualizados en todos los test files que lo implementan.

### 4. DTOs y dominios

- [ ] `domains/team/team_update_request.go`: +`Visible *bool`, +`IsPublic *bool`.
- [ ] `domains/team/team_response.go`: +`Visible bool`, +`IsPublic bool`.
- [ ] Nuevo `domains/team/team_search_request.go` (filtros + `page`) y `domains/team/team_search_response.go` (`{ teams []TeamSearchResult, has_more bool }`, con `owner_name`/conteo de miembros por resultado).
- [ ] Nuevo paquete `domains/joinrequest/`: `CreateJoinRequestResponse`, `JoinRequestResponse` (`id, team_id, team_name, runner_id, runner_name, status, created_at`), `PendingCountResponse`.

### 5. Servicios

- [ ] `services/team_service.go`: `Search(ctx, callerID int64, filters, page int) (*team.TeamSearchResponse, error)`.
- [ ] `services/join_request_service.go` (nuevo):
  - `Create(ctx, teamID, runnerID)` — valida `is_public`, cupo (D6), no-ya-miembro, no-duplicado (`FindPendingByTeamAndUser`).
  - `Cancel(ctx, requestID, callerID)` — valida dueño de la solicitud y estado `pending`.
  - `Accept(ctx, requestID, callerID)` — valida dueño del equipo y estado `pending`; si el corredor no es miembro todavía, revalida cupo y llama `ApplyTeamMembershipGate` (mismo patrón secuencial que `AcceptInvitation`, sin transacción propia); siempre `AssignToDefaultGroup(groupID: nil)`; `UpdateStatus(accepted)` como paso final independiente.
  - `Reject(ctx, requestID, callerID)` — valida dueño y `pending`; `UpdateStatus(rejected)`.
  - `ListMine(ctx, runnerID)`, `ListByTeam(ctx, teamID, callerID, page)`, `PendingCount(ctx, ownerID)`.
- [ ] Tests unitarios (mocks de DAOs + `teamUserDao`/`installDao` ya existentes para el gate): los ~13 escenarios de la tabla de códigos de error (D7) + el flujo feliz de cada operación + la revalidación de cupo en `Accept`.

### 6. Delegates y Controllers

- [ ] `delegates/join_request_delegate.go` (nuevo) — sigue el molde de `team_delegate.go`.
- [ ] `controllers/join_request_controller.go` (nuevo): handlers para las 7 rutas de join-requests, mapeo de errores a los códigos de D7.
- [ ] `controllers/team_controller.go`: handler `Search`.
- [ ] Registrar las 8 rutas en `cmd/api/app/url_mappings.go`, todas detrás de `AuthMiddleware()`.
- [ ] Tests de controller (httptest) para cada código de error + camino feliz.

### 7. Swagger y documentación

- [ ] Regenerar `cmd/api/docs` (`swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`).
- [ ] Actualizar tabla de endpoints en `README.md`.

### 8. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` en verde (repo completo).
- [ ] Prueba manual end-to-end contra la DB de testing real: crear equipo público, buscar desde otro usuario, pedir unirse, aceptar desde el entrenador, confirmar `team_user`/grupo default/estado de la solicitud.
