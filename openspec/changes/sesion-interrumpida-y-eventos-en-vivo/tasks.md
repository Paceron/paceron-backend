# Tasks: sesion-interrumpida-y-eventos-en-vivo

El sistema DEBE cumplir lo siguiente (MUST): el orden de etapas es el de las tasks; cada etapa termina verde (build + suite del paquete) antes de avanzar.

## Global Constraints (aplican a TODAS las tasks)

- Rama `feature/sesion-interrumpida-y-eventos-en-vivo` (ya creada desde develop). NO pushear, NO mergear: el usuario lo hace.
- Postgres real para tests DAO/servicio: `docker start paceron-test-db` (:5433) + env `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Módulo `simple-arq-golang` en imports internos. Conventional Commits. Stagear SOLO rutas explícitas del task (NUNCA `git add -A` / `git add .`; nada bajo `.superpowers/` se commitea jamás).
- Comentarios de código mínimos (solo invariantes/trampas no obvias, breves).
- Coverage gate 85: al final, `go clean -cache` + `make coverage-with-db` + analyzer `go run github.com/vladopajic/go-test-coverage/v2@latest --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out`. NO tocar `.testcoverage.yml`.
- Swagger: `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal` cuando se toque controller con anotaciones.
- NO modificar `GET /workout-feedback/search`, ni Search/DAO search, ni nada fuera de lo listado por task.
- Lecciones de tests previas: `WorkoutFeedback.Create` directo en tests necesita `var media pgtype.TextArray; media.Set(nil); MediaURLs: media`; subtests de feedback necesitan instancia propia cada uno (unique_feedback_per_set); fixtures de día de calendario NO comparten `(group_id, date)` (unique index).

---

### Task 1: Gap 25 — fix ventana de membresía por fecha (hot)

**Files:**
- Modify: `cmd/api/daos/group_user_dao.go` (helper `activeGroupMemberWhere`)
- Test: `cmd/api/daos/group_user_dao_membership_window_test.go` (nuevo)

**Interfaces:**
- Consumes: nada nuevo (helpers existentes del paquete daos).
- Produces: helper corregido `activeGroupMemberWhere(query, sessionDate)` con comparación por fecha (`date_start::date <= ?::date`, `date_end::date >= ?::date`) — Team/attendance/bulk lo consumen sin cambio de firma.

- [x] **1.1 Escribir test de repro ANTES del fix** (debe FALLAR contra el código actual): fixture con grupo + corredores con membresía `date_start = hoy con hora` (p. ej. `time.Now()`), fecha de sesión = hoy; asertar que el corredor: (a) aparece en `FindGroupRosterWithAttendance`, (b) `IsActiveGroupMember` → true, (c) `MissingGroupMembers` → no lo lista. Correr y confirmar que falla (roster lo excluye hoy).
- [x] **1.2 Fix del helper**: comparación por fecha en `activeGroupMemberWhere` (cast `::date` en ambos lados de las dos comparaciones).
- [x] **1.3 Correr el test de repro → PASS**, más los tests DAO preexistentes de attendance/membresía (regresión).
- [x] **1.4** `go build ./...`, `go vet ./cmd/api/daos`, gofmt limpio. Commit: `fix(attendance): ventana de membresia por fecha de calendario (Gap 25)`.

### Task 2: Gap 19 — estado interrupted

**Files:**
- Modify: `cmd/api/domains/runnersession/runner_session.go` (msg constant, doc del request)
- Modify: `cmd/api/daos/runner_session_dao.go` (reemplazar `Finish` por `UpdateStatus`)
- Modify: `cmd/api/services/runner_session_service.go`
- Modify: `cmd/api/controllers/runner_session_controller.go` (solo anotación Swagger del PATCH + doc)
- Test: `cmd/api/services/runner_session_service_test.go`, `cmd/api/controllers/runner_session_controller_test.go`, `cmd/api/daos/runner_session_dao_test.go` (ampliar existentes)

**Interfaces:**
- Consumes: DAO actual (`GetBySessionAndAthlete`, `Create`, `SessionInstanceExists`, `resolveAthlete`).
- Produces: `UpdateStatus(ctx, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error` en `RunnerSessionDAOInterface` (reemplaza a `Finish`); constantes de estado `"wip"|"finished"|"interrupted"`; msg `MsgRunnerSessionInterrupted`.

- [x] **2.1** Domain: constante `MsgRunnerSessionInterrupted = "sesión marcada como interrumpida"`; doc de `RunnerStatusRequest` (admite finished|interrupted).
- [x] **2.2** DAO: reemplazar `Finish` por `UpdateStatus(ctx, id, to, fromStatuses, endDate)` con guard `WHERE id = ? AND status IN ?` (anti-transición ilegal a nivel SQL); actualizar mocks.
- [x] **2.3** Service `RunnerStatus`: validar `status ∈ {finished, interrupted}` (400 si otro); idempotencia (status actual == destino → devolver fila sin cambios); `finished` actual → si status pedido es `interrupted` → 400 "no se puede interrumpir una sesión ya finalizada"; `interrupted→finished` permitido con `end_date = now` del server; transición legal → `UpdateStatus` + re-GET. Mantener `resolveAthlete` y el flujo de 404.
- [x] **2.4** Tests service (mock DAO): matriz transiciones (wip→finished, wip→interrupted, interrupted→finished con end_date nueva, interrupted→interrupted idempotente, finished→interrupted 400, status inválido 400, 404). Tests DAO Postgres real: `UpdateStatus` desde wip y desde interrupted, guard `finished` inamovible. Tests controller: 400 finished→interrupted, 200 interrupted (msg constante), Swagger anotación PATCH actualizada + regenerar (`swag init ...`).
- [x] **2.5** `go build`, `go vet`, suite `./cmd/api/{daos,services,controllers}` verde. Commit: `feat(runner-session): estado interrupted en la sesion del corredor (Gap 19)`.

### Task 3: Gap 23 — photo_url en search/batch

**Files:**
- Modify: `cmd/api/domains/user/search_response.go`
- Modify: `cmd/api/services/user_service.go` (Search y BatchLookup)
- Test: `cmd/api/services/user_service_test.go`

**Interfaces:**
- Consumes: `buildMediaURL(key *string, updatedAt *time.Time) *string` (services/media_url.go).
- Produces: `SearchResultItem.PhotoURL *string json:"photo_url"` (nullable, sin omitempty).

- [x] **3.1** DTO + mapeo en `Search` y `BatchLookup` con `buildMediaURL(u.PhotoKey, u.PhotoUpdatedAt)`.
- [x] **3.2** Tests: usuario con foto (URL con `?v=` de photo_updated_at) y sin foto (null), en search y batch. Suite `./cmd/api/services` verde.
- [x] **3.3** Commit: `feat(users): photo_url en sugerencias y batch lookup (Gap 23)`.

### Task 4: Gap 26 — apertura/cierre de sesión presencial

**Files:**
- Modify: `cmd/api/domains/dbs/group_calendar_day.go` (2 columnas)
- Modify: `cmd/api/daos/group_calendar_day_dao.go` (`FindBySessionInstanceID` + update de opened/closed)
- Modify: `cmd/api/domains/instance/instance_response.go` (3 campos omitempty) y mapeo del detalle
- Modify: `cmd/api/services/calendar_service.go` (detalle resuelve día por instancia)
- Modify: `cmd/api/services/runner_session_service.go` + `cmd/api/controllers/runner_session_controller.go` (hooks D7, gate D9, notifier D10)
- Modify: `cmd/api/app/app.go` (notifier wiring runner session controller)
- Test: DAO tests, service tests, controller tests (nuevos + extensión)

**Interfaces:**
- Consumes: Task 2 (`RunnerSessionService` actualizado), `realtime.Notifier` (Gap 18), `HasInstanceAccess` (Gap 14).
- Produces: `GroupCalendarDay.PresencialOpenedAt/PresencialClosedAt *time.Time`; `FindBySessionInstanceID(ctx, sessionInstanceID) (*dbs.GroupCalendarDay, error)`; sentinels runner-session `ErrRunnerSessionNotOpen` (409, code `session_not_opened`) y `ErrRunnerSessionClosed` (409, code `session_closed`) con Code slug en el controller; 3 campos omitempty en detalle; evento `update:session_state`.

- [x] **4.1** Modelo + DAO: columnas nullable en `GroupCalendarDay`; `FindBySessionInstanceID` (1:1 por diseño, nil si no hay día); método para setear opened/closed (Updates con guard de NULL, ej. `SET presencial_opened_at = ? WHERE id = ? AND presencial_opened_at IS NULL`); tests DAO Postgres real.
- [x] **4.2** Hooks D7 en el flujo de runner session: resolver día por instancia (`FindBySessionInstanceID`); si día training+presencial y auth es owner del team del grupo → Create setea opened_at si NULL; PATCH finished del owner setea closed_at si NULL; `interrupted` no cierra; falla del update del día sube el error (post-escritura del runner session, D7). Owner check con el DAO de teams (`IsTeamOwner` del team del grupo — el día trae `group_id`, el grupo trae `team_id`; reutilizar consultas existentes, sin N+1).
- [x] **4.3** Gate D9: en `Create` de corredor no-owner sobre día presencial: `closed_at != NULL` → 409 `session_closed`; `opened_at == NULL` → 409 `session_not_opened` (slugs en `Code` del APIError, controller); owner exento; días no presenciales sin gate. Orden: 404 instancia → 403 atleta ajeno → gate.
- [x] **4.4** Detalle D8: 3 campos omitempty en el DTO de instancia; el path de detalle (`SessionInstanceDetail`) los resuelve con `FindBySessionInstanceID`; paths de calendario NO los setean (nil → ausentes del JSON). Test: detalle presencial abierto (true + timestamps), cerrada (false + closed_at), huérfana/no presencial (ausentes), y una respuesta de calendario que NO los incluya.
- [x] **4.5** WS D10: notifier en runner session controller; al abrir y al cerrar emitir `update:session_state` con `{presencial_open, opened_at, closed_at}` (estado post-write) al canal `session:{id}`, sin exclusión; wiring `app.go` con notifier nil-safe; tests: emisión en apertura (hub real), emisión en cierre, nil notifier sin panic, corredor no emite.
- [x] **4.6** Tests service/controller del gate y hooks (fixture owner + grupo + día presencial: abrir → corredor entra; cerrar → 409 session_closed; antes de abrir → 409 session_not_opened; día no presencial sin gate; corredor ajeno 403 prevalece).
- [x] **4.7** Swagger del detalle (anotación con campos nuevos) + regenerar; suite completa de los paquetes tocados verde. Commit: `feat(calendar): apertura y cierre de sesion presencial por el entrenador (Gap 26)`.

### Task 5: Gap 27 — relay dirigido

**Files:**
- Modify: `cmd/api/realtime/hub.go` (targeting por userID)
- Modify: `cmd/api/realtime/connection.go` (decidir destino según payload.to)
- Test: `cmd/api/realtime/hub_test.go`, `cmd/api/realtime/connection_test.go`

**Interfaces:**
- Consumes: hub/client existentes (D2/D4/D5 del change `ws-gateway-sesiones`).
- Produces: entrega dirigida por `payload.to` numérico en presence/control (sin cambio de protocolo para el resto).

- [x] **5.1** Hub: método para entregar a un solo usuario suscripto al canal (todas sus conexiones), misma mecánica no-bloqueante de `Broadcast` (reutilizar, no duplicar).
- [x] **5.2** Connection: en relay de presence/control, extraer `to` del payload ya unmarshalado; numérico → dirigido; `"all"`/ausente/otro tipo → actual. Frame completo con `to` dentro del payload.
- [x] **5.3** Tests: dirigido a user con 2 conexiones (solo esas reciben), resto de la sala en silencio; sin `to`/`to:"all"` → actual; `to` no suscripto → nadie lo recibe, conexión viva; `to` string no-"all" → va a todos. `-race`.
- [x] **5.4** Commit: `feat(realtime): entrega dirigida por payload.to en presence y control (Gap 27)`.

### Task 6: Gap 28 — evento de asistencia

**Files:**
- Modify: `cmd/api/daos/attendance_dao.go` (Register devuelve fila; `BulkUpsertManual` RETURNING extendido con filas)
- Modify: `cmd/api/services/attendance_service.go` (Register/Bulk/Delete exponen filas o user_id+session)
- Modify: `cmd/api/controllers/attendance_controller.go` (notifier wiring + emisión D13)
- Modify: `cmd/api/app/app.go` (notifier wiring)
- Test: `cmd/api/daos/attendance_dao_test.go`, `cmd/api/services/attendance_service_test.go`, `cmd/api/controllers/attendance_controller_test.go`

**Interfaces:**
- Consumes: Task 4 (notifier pattern ya wired en app para runner session), `realtime.Notifier` (Gap 18).
- Produces: emisión `update:attendance_event` en Register(created)/Bulk(per-user)/Delete; `Register` devuelve la fila creada; `BulkUpsertManual` devuelve filas afectadas (counts derivados en Go, `BulkSaveResult` intacto como respuesta HTTP).

- [x] **6.1** DAO: `Register` (service layer) devuelve la fila creada (id + created_at); `BulkUpsertManual` RETURNING `user_id, id, created_at, (xmax=0) AS inserted` y devuelve filas con flag inserted; counts creados/actualizados derivados en Go (respuesta `BulkSaveResult` sin cambio).
- [x] **6.2** Service: `Register` expone fila (solo emite el controller si created=true); `BulkSaveAttendance` devuelve filas por usuario; `DeleteAttendance` lee la fila antes de borrar y expone `user_id` + `training_session_id`. Ajustar interfaces y mocks.
- [x] **6.3** Controller: notifier nil-safe; emitir `update:attendance_event` con payload fila exacta (`{user_id, status, source, registered_at, attendance_id}`; delete → `not_confirmed` + nulls) al canal `session:{training_session_id}` — el `training_session_id` del attendance ES el session instance id (misma FK opaca). Register solo created=true; Bulk un evento por fila; Delete siempre con éxito.
- [x] **6.4** Tests: DAO (RETURNING, counts derivados = mismos números que hoy), service (filas expuestas), controller (emisión QR, bulk N eventos, delete evento de baja, idempotente 200 sin evento, nil notifier sin panic). Suite verde.
- [x] **6.5** Commit: `feat(attendance): evento WS al registrar y borrar asistencia (Gap 28)`.

### Task 7: Docs + verificación final

**Files:**
- Modify: `docs/CATALOGO_Y_CALENDARIO.md` (§8.x: open/close, gate, detalle campos nuevos, eventos WS)
- Modify: `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (sección nueva: 6 gaps, shapes verificados contra código)
- Modify: `docs/DEUDA_TECNICA_Y_PENDIENTES.md` (persistencia de mensajes WS críticos deferida; normalización de escritura date_start no requerida)
- Modify: `docs/REALTIME_WS.md` (update:session_state, update:attendance_event, to dirigido)
- Modify: `openspec/changes/sesion-interrumpida-y-eventos-en-vivo/tasks.md` (tildes finales)

**Interfaces:**
- Consumes: todo lo implementado en Tasks 1-6.
- Produces: docs fieles al código + rama verificada.

- [x] **7.1** Docs con shapes 1:1 contra los DTOs/código real (JSON de docs matchean tags json). Cruzar referencias entre docs.
- [x] **7.2** `openspec validate sesion-interrumpida-y-eventos-en-vivo --strict` → valid; gofmt en archivos tocados; `go build ./...`; `go vet ./...`.
- [x] **7.3** Suite completa `go test -count=1 ./...` con Postgres real → 0 FAIL.
- [x] **7.4** Coverage: `go clean -cache` + `make coverage-with-db` + analyzer → gate 85 PASS, `.testcoverage.yml` sin diff vs develop. Si el número sale raro (~70%), re-correr con cache limpio antes de investigar.
- [x] **7.5** Tildar checkboxes restantes y commit: `docs(sesion-live): cerrar gaps 19/23/25/26/27/28 en la documentacion`.
