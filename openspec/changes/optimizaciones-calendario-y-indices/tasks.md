# Tasks: optimizaciones-calendario-y-indices

El sistema DEBE cumplir lo siguiente (MUST) en todas las tasks: **ningún endpoint, DTO, shape de
respuesta ni comportamiento observable cambia** — todo es interno (constraint del usuario).
Postgres real (`docker start paceron-test-db`, env TEST_DB_HOST=localhost TEST_DB_PORT=5433
TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test). Suite verde y coverage
gate 85 con `go clean -cache` (bug de cache conocido). Comentarios mínimos. Stagear solo rutas
explícitas. Sin push/merge.

### Task 1: DAOs batch + índices

- [x] **1.1** `dbs/session_instance.go` + `dbs/session_exercise_instance.go`:
      `SessionInstanceDaoInterface` gana `FindByIDs` (patrón ExerciseInstanceDao:`id IN ?`,
      Order("id"), early-return vacío sin query) y `SessionExerciseInstanceDaoInterface` gana
      `FindBySessionInstances` (session_instance_id IN ?, Order("id"), early-return vacío).
      `dbs/team_user.go`: `TeamUserDaoInterface` gana `CountActiveByTeams(ctx, teamIDs)`
      (COUNT GROUP BY team_id, deleted_at IS NULL, early-return vacío) con struct mínima en dbs/
      (TeamMemberCount{TeamID int64, Count int64}).
- [x] **1.2** Índices con tags: `GroupUser` (idx group_id, idx user_id), `TeamUser`
      (idx team_id, idx user_id), `SessionExerciseInstance` (idx session_instance_id,
      idx exercise_instance_id), `WorkoutFeedbackPoint` (idx feedback_id). Sin backfill.
- [x] **1.3** Tests DAO Postgres real: roundtrip de los 3 métodos batch (incl. vacío sin query,
      filtro deleted_at para CountActiveByTeams) + tests de `pg_indexes` para las 7 índices
      (patrón PR #93).
- [x] **1.4** `go build ./...` + `go vet` + gofmt en archivos tocados + suite daos verde + commit.

### Task 2: builder batch de calendario

- [x] **2.1** Refactor: extraer builder puro `buildDayResponse(d, inst)` del mapping actual de
      `toCalendarDayResponse` (mapping INVARIADO); `toCalendarDayResponse` = lookup único +
      buildDayResponse. Nuevo `sessionInstanceResponsesByIds(ctx, ids)` con 3 queries batch y las
      mismas semánticas de error de hoy (instancia faltante → "sesión instancia %d no encontrada",
      link sin ejercicio → error de NewSessionResponse, huérfana rompe igual).
- [x] **2.2** Nuevo `toCalendarDayResponses(ctx, days)` y migrar SOLO los loops de colección:
      GetRange, respuestas post-commit de Stamp/Bulk/Shift, BulkClear, MemberCalendar,
      AdministeredCalendar. UpsertDay y demás callers de fila única intactos. Path `s.db == nil`
      conserva su comportamiento ( días sin detalle).
- [x] **2.3** Mocks de interfaces extendidas actualizados sin alterar aserciones.
- [x] **2.4** Test nuevo de reducción de queries (mock que cuenta llamadas: N días → 3 queries batch,
      0 FindByID) + regresión completa del paquete services (todas las suites existentes SIN ajustar
      aserciones).
- [x] **2.5** `go build ./...` + `go vet` + gofmt + suite services/daos verde con Postgres real + commit.

### Task 3: team search y CalendarSummary en batch

- [x] **3.1** team_service Search: owners con `UserDao.FindByIDs` (1 query) + `CountActiveByTeams`
      (1 query); can_receive_payments batch ya existente intacto. Respuesta byte-idéntica.
- [x] **3.2** CalendarSummary: nombres con `GroupDao.FindByIDs` (1 query) + map. Comportamiento igual.
- [x] **3.3** Tests: regresión de Search (mocks ajustados a interfaces extendidas, aserciones intactas)
      + si ya existe test de CalendarSummary sin tocar. `go build/vet/gofmt` + suite verde + commit.

### Task 4: docs + verificación final

- [x] **4.1** `docs/CATALOGO_Y_CALENDARIO.md`: nota corta de optimización interna (builder batch,
      índices; sin cambio de contrato). `docs/DEUDA_TECNICA_Y_PENDIENTES.md`: marcar resuelto el
      ítem GetRange N+1 y la deuda del feedback_point sin índice (si están).
- [x] **4.2** `openspec validate optimizaciones-calendario-y-indices --strict` → valid;
      gofmt/build/vet limpios; Swagger SIN regenerar (nada cambió); suite completa 0 FAIL.
- [x] **4.3** Coverage: `go clean -cache` + `make coverage-with-db` + analyzer → gate 85 PASS,
      `.testcoverage.yml` sin diff vs develop. Tildar checkboxes y commit final.
