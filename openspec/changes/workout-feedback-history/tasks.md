# Tasks: workout-feedback-history

## Global Constraints (aplican a todas las tasks)

- Rama: `feature/workout-feedback-history` desde develop actualizado.
- Postgres real para tests DAO: `docker start paceron-test-db` (:5433), env `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Coverage gate **85** (nuevo umbral): al finalizar, `go clean -cache` + `make coverage-with-db` + analyzer `go-test-coverage` ≥85; nunca bajar el umbral ni tocar `.testcoverage.yml`.
- Comentarios de código mínimos (solo lo esencial no-obvio).
- Stagear SOLO rutas explícitas del task (jamás `git add -A`; `.superpowers/` nunca se commitea).
- No push/merge: el usuario pushea y mergea. Commits Conventional Commits.
- Swagger: `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal`.
- `GET /workout-feedback/search` NO se modifica (ningún cambio en su handler/service/DAO/search filters).
- Módulo Go: `simple-arq-golang` (imports internos usan ese path).

### Task 1: DAO de historial (queries + pools + tests)

- [x] 1.1 Crear struct de filtros `WorkoutFeedbackHistoryFilters` (scope athlete o team, TeamID, GroupID, DateFrom/DateTo, ExerciseInstanceID, SetNumber, AthleteFilterUserID) y fila `WorkoutFeedbackHistoryRow` (campos del design D2) — ubicación según D2.
- [x] 1.2 Agregar a `WorkoutFeedbackDAO` interface + impl: `HistorySearch` (joins D1, WHERE dinámico `deleted_at IS NULL`, ORDER BY whitelisted con desempate por `wf.id`), `HistoryCount`, `HistoryAvailableAthletes`, `HistoryAvailableExercises` (solo primer nivel; ver D5).
- [x] 1.3 Tests DAO contra Postgres real (fixture: 2 equipos, grupos, día con instancia, feedbacks normal + huérfano sin día + sin team + soft-deleted): filtros 1er/2do nivel, sort whitelist + order + desempate id, paginación offset/limit, count sin paginación, pools ignorando 2do nivel, huérfano incluido con group null, deleted_at excluido.
- [x] 1.4 Build + `go test ./cmd/api/daos` con DB real verde.

### Task 2: Service de historial (autorización + validación + armado de response)

- [x] 2.1 `WorkoutFeedbackService`: `AthleteHistory(ctx, callerID, targetID, query)` y `AdministeredHistory(ctx, callerID, targetID, query)` según D3/D4 (sentinels de error nuevos o reuso: `ErrWorkoutFeedbackForbidden`, `ErrTeamNotFound`).
- [x] 2.2 Validaciones D4 (fechas pareadas, group requiere team, page/page_size, sort/order whitelist) → errores con código HTTP via `apierror`/patrón del paquete.
- [x] 2.3 Armado del response DTO D6: mapear filas → ítems, nombres en batch (users/teams/groups), pools del DAO, total.
- [x] 2.4 Tests service (mocks de DAO): matriz de autorización, validaciones 400, response shape, pools passthrough.

### Task 3: Controller + rutas + Swagger

- [x] 3.1 Handlers en `WorkoutFeedbackController` con parsing de query (patrón member-calendar: parse helpers con errores 400 tipados), llamada al service, mapeo de errores a códigos (403/404/400), JSON 200 con response.
- [x] 3.2 Rutas en `url_mappings.go` + wiring `app.go`.
- [x] 3.3 Anotaciones Swagger (params + codes 200/400/403/404) y regenerar docs.
- [x] 3.4 Tests controller con mock del service (convención del paquete): query parsing, 200 body, 400/403/404.

### Task 4: Documentación + verificación final

- [x] 4.1 `docs/CATALOGO_Y_CALENDARIO.md` (o doc de dominio afín): nueva sección de historial con shapes/errores verificados contra código.
- [x] 4.2 `docs/FRONTEND_IMPACTO_INSTANCIACION.md`: sección del gap 13 cerrado (shapes, decisiones: exercise_id=instancia, catalog_exercise_id aditivo, huérfanos con nulls).
- [x] 4.3 Verificación final: `openspec validate workout-feedback-history --strict`; gofmt en archivos tocados; `go build ./...`; `go vet ./...`; suite completa `go test ./...` con Postgres real (0 FAIL); `go clean -cache` + `make coverage-with-db` + analyzer ≥85 (gate); tildar checkboxes.

### Task 5: Ajuste post-feedback — familia de catálogo (dedupe de pool + filtro)

- [x] 5.1 DAO: `HistoryAvailableExercises` dedupeado por familia (`DISTINCT ON COALESCE(ei.source_exercise_id, ei.id)`, representante = instancia id menor), con test de pool dedupeado.
- [x] 5.2 DAO: filtro `exercise_id` con semántica de familia (`ei.source_exercise_id = X OR (ei.source_exercise_id IS NULL AND wf.assigned_exercise_id = X)`), join condicional en `HistoryCount`, con tests (source matchea todas sus instancias, legacy matchea por instancia, instancia con origen no matchea como instancia).
- [x] 5.3 Spec delta + design + docs actualizados a la semántica de familia; `openspec validate --strict`; suite con Postgres real verde.

### Task 6: Ajuste post-feedback — `session_instance_id` en los ítems del historial

- [ ] 6.1 DAO: `SessionInstanceID` en `WorkoutFeedbackHistoryRow` + `wf.assigned_session_id AS session_instance_id` en el SELECT de `HistorySearch`, con test de shape de fila.
- [ ] 6.2 DTO + service: campo `session_instance_id` en `WorkoutFeedbackHistoryItem` (siempre presente, no nullable: es la FK opaca del feedback) + mapeo en `historyItems`, con test de shape.
- [ ] 6.3 Docs (§8.9 + §9), spec delta, `swag init`, `openspec validate --strict`; suite con Postgres real verde.
