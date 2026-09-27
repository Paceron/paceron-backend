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

- [ ] 4.1 `docs/CATALOGO_Y_CALENDARIO.md` (o doc de dominio afín): nueva sección de historial con shapes/errores verificados contra código.
- [ ] 4.2 `docs/FRONTEND_IMPACTO_INSTANCIACION.md`: sección del gap 13 cerrado (shapes, decisiones: exercise_id=instancia, catalog_exercise_id aditivo, huérfanos con nulls).
- [ ] 4.3 Verificación final: `openspec validate workout-feedback-history --strict`; gofmt en archivos tocados; `go build ./...`; `go vet ./...`; suite completa `go test ./...` con Postgres real (0 FAIL); `go clean -cache` + `make coverage-with-db` + analyzer ≥85 (gate); tildar checkboxes.
