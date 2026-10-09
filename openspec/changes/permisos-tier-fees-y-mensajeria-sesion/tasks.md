# Tasks: permisos-tier-fees-y-mensajeria-sesion

## Global Constraints (todas las tareas)

- Rama `feature/tiers-fees-pagos-y-mensajeria` ya creada desde develop — NO cambiar de rama, NO push, NO merge (el usuario lo hace).
- Postgres real para tests de DAO: `docker start paceron-test-db` (:5433), env `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Coverage: `go clean -cache` antes de `make coverage-with-db` + analyzer (`go run github.com/vladopajic/go-test-coverage/v2@latest --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out`) → gate 85 PASS; `.testcoverage.yml` NUNCA se toca.
- Comentarios de código mínimos (solo invariantes no obvios, breves). Conventional Commits. Stagear SOLO rutas explícitas del task — JAMÁS `git add -A`/`git add .`; nada bajo `.superpowers/` u `openspec/` commiteado por los implementadores.
- Swagger regenerar con el comando canónico del repo: `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- Imports internos con módulo `simple-arq-golang`.
- La regla dual de acceso a instancia es `sessionInstanceDao.HasInstanceAccess` (ya existe, no re-implementar).
- El helper `sessionChannel(id)` ya existe en controllers; el patrón de broadcast nil-safe es el de `workout_feedback_controller.go` Create.

### Task 1: Gap 5 — GET /tiers/:id/permissions

**Files:** Modify: `cmd/api/controllers/tier_permission_controller.go`, `cmd/api/services/tier_permission_service.go`, `cmd/api/daos/tier_permission_dao.go` (interface + impl si falta método), `cmd/api/app/url_mappings.go`; Test: test files co-ubicados; Swagger regenerado.

- [x] **1.1** DAO/service: método para listar permisos activos de un tier con nombre (join con `permissions`), ordenado `permission_id` ASC; 404 si el tier no existe (via tierDao o reuso del patrón de GetByID), 200 `[]` sin permisos.
- [x] **1.2** Controller + ruta + Swagger (`@Success 200` con shape `{"permissions":[...]}`, `@Failure 404`), autorización autenticado (sin ownership).
- [x] **1.3** Tests: service mocks (tier inexistente 404, lista con nombres, vacía), controller (200 shape, 404). Suite verde.
- [x] **1.4** Commit: `feat(tiers): listar permisos de un tier (Gap 5)`.

### Task 2: Gap 15+17 — fee y flag en search/detail/invitaciones

**Files:** Modify: `cmd/api/domains/team/team_search.go`, `cmd/api/domains/team/team_response.go`, `cmd/api/domains/invitation/invitation_response.go`, `cmd/api/services/team_service.go`, `cmd/api/services/invitation_service.go`, `cmd/api/daos/seller_connection_dao.go` (si falta método batch), `cmd/api/controllers/team_controller.go` (si el mapeo vive ahí); Test: co-ubicados; Swagger regenerado.

- [x] **2.1** `can_receive_payments` derivado en service: owner con seller_connection `authorized` + `public_key` presente (mismo criterio que `resolveTeamSplitConfig`); search en batch (1 query con los owners de la página), detail/invitación single. Lookup falla → flag false.
- [x] **2.2** Campos en DTOs: `membership_fee` + `can_receive_payments` en `TeamSearchResult` y `InvitationResponse`; `can_receive_payments` en `TeamResponse`. Fee vigente del team (no frozen).
- [x] **2.3** Tests: search con mix de equipos (con/sin conexión MP), detail, invitación (fee vigente); flag false si el DAO falla. Suite verde.
- [x] **2.4** Commit: `feat(teams): membership_fee y can_receive_payments en search, detalle e invitaciones (Gap 15/17)`.

### Task 3: Gap 16 — mapeo de errores de CreatePreference

**Files:** Modify: `cmd/api/services/payment_service.go` (sentinels), `cmd/api/controllers/payment_controller.go` (mapper); Test: co-ubicados; Swagger regenerado si cambia doc de errores.

- [x] **3.1** Sentinels: seller no conectado (409 `SELLER_NOT_CONNECTED`), cuota/equipo no encontrada (404), inválido (400) — los mensajes actuales del service se preservan como `message`.
- [x] **3.2** Mapper en controller: resto sigue 500 genérico "Error al crear la preferencia" (sin 502). Body de error `apierror.APIError` con `status_code`/`code`/`message`.
- [x] **3.3** Tests: matriz de mapeo (409 con code, 404, 400, 500 residual) en service mocks + controller. Suite verde.
- [x] **3.4** Commit: `feat(payments): codigos de error reales en CreatePreference (Gap 16)`.

### Task 4: Gap 27 — modelos y migración

**Files:** Create: `cmd/api/domains/dbs/session_message.go`, `cmd/api/domains/dbs/session_message_recipient.go`; Modify: `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate).

- [x] **4.1** Modelos según design D5 (columnas exactas, sin FKs físicas — patrón opaque del repo; `deleted_at` NO — mensajes no se borran).
- [x] **4.2** AutoMigrate de ambas tablas; build + suite daos verde (SetupTestDB ejercita la migración).
- [x] **4.3** Commit: `feat(messages): modelo session_messages y recipients (Gap 27)`.

### Task 5: Gap 27 — DAOs de mensajes

**Files:** Create: `cmd/api/daos/session_message_dao.go`; Modify: mocks si aplica; Test: `cmd/api/daos/session_message_dao_test.go`.

- [x] **5.1** `Create` (mensaje + destinatarios en la misma tx) y `FindByID` (con destinatarios) para la validación de reply.
- [x] **5.2** `FindVisibleSince(ctx, sessionInstanceID, viewerUserID, sinceID)` — join con recipients; visibilidad emisor/all/en-lista; `id > since`; order `id` ASC.
- [x] **5.3** Tests Postgres real: visibilidad completa (emisor lo ve, all lo ven todos, recipients lo ven, ajeno no), catch-up con since, DM privado para el entrenador, orden cronológico.
- [x] **5.4** Commit: `feat(daos): queries de mensajes de sesion (Gap 27)`.

### Task 6: Gap 27 — service, controller, rutas y broadcast

**Files:** Create: DTO `cmd/api/domains/sessionmessage/` (request/response), `cmd/api/services/session_message_service.go`, `cmd/api/controllers/session_message_controller.go`; Modify: `cmd/api/app/url_mappings.go` (rutas), `cmd/api/app/app.go` (wiring notifier compartido); Test: co-ubicados; Swagger regenerado para los 2 endpoints (el frame WS no va al Swagger).

- [x] **6.1** Service: autorización dual (403/404), derivación de `sender_role` (owner del team del día → trainer), validaciones 400 del POST (type/mode/destinatarios/participantes/reply), shape de response según D8.
- [x] **6.2** GET con `since` (parse del param, 0 = todo), visibilidad delegada al DAO.
- [x] **6.3** Controller: `POST /session-instances/:id/messages` y `GET /session-instances/:id/messages` — responder 201 mensaje creado / 200 `{"messages":[...]}`; broadcast `MarshalControlMessageCreated` (frame `{"type":"control:message_created","channel":"session:{id}","payload":{"sessionMessageId":N}}`) tras POST exitoso, nil-safe, sin exclusión de emisor; notifier compartido de app.go.
- [x] **6.4** Tests: service mocks (matriz autorización + validaciones + sender_role), controller (201/200/400/403/404, broadcast con notifier real de test + nil-safety, emisor recibe aviso).
- [x] **6.5** Commit: `feat(messages): endpoints y aviso WS de mensajeria de sesion (Gap 27)`.

### Task 7: Docs + verificación final

**Files:** Modify: `docs/CATALOGO_Y_CALENDARIO.md` (sección de mensajería en vivo), `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (sección nueva con los 5 gaps cerrados, shapes verificados contra código), `docs/REALTIME_WS.md` (evento `control:message_created` en la tabla de eventos), `openspec/changes/permisos-tier-fees-y-mensajeria-sesion/tasks.md` (tildes).

- [x] **7.1** Docs con shapes 1:1 contra los DTOs reales (JSON de docs matchean tags json), referencias cruzadas entre docs.
- [x] **7.2** `openspec validate permisos-tier-fees-y-mensajeria-sesion --strict` → valid; gofmt archivos tocados; build + vet limpios.
- [x] **7.3** Suite completa `go test -count=1 ./...` con Postgres real → 0 FAIL.
- [x] **7.4** Coverage: `go clean -cache` + `make coverage-with-db` + analyzer → gate 85 PASS, `.testcoverage.yml` sin diff vs develop.
- [x] **7.5** Tildar checkboxes y commit: `docs: cerrar gaps 5/15/16/17/27 en la documentacion`.
