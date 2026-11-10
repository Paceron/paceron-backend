# Tasks — optimizaciones-lecturas-cache

## Global Constraints

- Rama `feature/optimizaciones-lecturas-cache`; no push/merge.
- Sin cambios de endpoints, DTOs, shapes ni autorización.
- Tests con Postgres real `paceron-test-db:5433` (`TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`); prohibido local/demo/fullseed.
- Comentarios mínimos; stagear solo rutas explícitas; nunca `.superpowers/`.
- Coverage gate 85; no tocar `.testcoverage.yml`.

### Task 1: Calendario y banners en batch

**Files:** `cmd/api/domains/instance/instance_response.go`, `cmd/api/services/calendar_service.go`, `cmd/api/daos/session_exercise_instance_dao.go`, `cmd/api/daos/group_calendar_day_dao.go`, tests de services/daos.
**Commit:** `perf(calendar): resolver detalle de instancias y banners en batch`

- [x] 1.1 Construir el índice de ejercicios una vez por colección y usarlo en `NewSessionResponse`; conservar el mapper de fila única.
- [x] 1.2 Agregar el DAO de links+ejercicios en una consulta, manteniendo orden y semántica de errores; migrar `sessionInstanceResponsesByIds`.
- [x] 1.3 Reducir `NextSession` a una consulta agrupada + batch de instancias/nombres, preservando “hoy cuenta” y shapes; si no es viable, dejar una query por kind y batchear solo instancias/nombres.
- [x] 1.4 Tests de conteo y paridad de respuesta de `GetRange`, member/admin calendar y `NextSession`.
- [x] 1.5 `go build ./...`, `go vet ./...`, tests focales con DB y commit.

### Task 2: Índices P1

**Files:** `cmd/api/domains/dbs/group_user.go`, `team_user.go`, `group.go`, `exercise_instance.go`, `cmd/api/daos/daos_indexes_test.go`.
**Commit:** `perf(db): indices de autorizacion y scope`

- [x] 2.1 Agregar índices compuestos/parciales para `group_users`, `team_users`, `groups.team_id` y `exercise_instances.source_exercise_id`.
- [x] 2.2 Extender `daos_indexes_test.go` y verificar `pg_indexes` real.
- [x] 2.3 `go build ./...`, `go vet ./...`, test DAO con DB y commit.

### Task 3: Push de cierre por lote

**Files:** `cmd/api/daos/push_token_dao.go`, `cmd/api/services/push_notifier.go`, `cmd/api/services/competition_service.go`, tests.
**Commit:** `perf(push): lotes acotados y timeout por chunk`

- [x] 3.1 Chunkear `FindByUserIDs` (500 IDs) y los tokens por lote con timeout propio.
- [x] 3.2 Mantener fan-out async, best-effort, payload y destinatarios; tests de lotes y no bloqueo.
- [x] 3.3 `go build ./...`, `go vet ./...`, tests focales con DB y commit.

### Task 4: Equipos e invitaciones sin N+1

**Files:** `cmd/api/daos/invitation_dao.go`, `cmd/api/services/team_service.go`, `cmd/api/services/invitation_service.go`, tests.
**Commit:** `perf(teams): listados de equipos e invitaciones en batch`

- [x] 4.1 Filtrar invitaciones vencidas en SQL y mantener orden/visibilidad.
- [x] 4.2 `filterByMember` en batch y alta de equipo con `FindByIDs` de roles.
- [x] 4.3 Tests de conteo y paridad; build/vet/tests focales con DB y commit.

### Task 5: Docs y verificación final

**Files:** `docs/DEUDA_TECNICA_Y_PENDIENTES.md`, `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (solo nota interna si aplica), `openspec/changes/optimizaciones-lecturas-cache/tasks.md`.
**Commit:** `docs(performance): cerrar optimizaciones de lecturas y cache`

- [x] 5.1 Documentar deuda restante y caching seguro/no seguro; no prometer cambios no implementados.
- [x] 5.2 `openspec validate --strict`, gofmt, build/vet, suite completa y coverage gate.
- [x] 5.3 Tildar tasks y commit.
