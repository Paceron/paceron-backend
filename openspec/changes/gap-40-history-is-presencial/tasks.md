# Tasks — Gap 40

## Global Constraints

- Rama `feature/gap-40-history-is-presencial`, base `develop` actualizado.
- Cambio aditivo: no cambiar filtros, paginación, pools, endpoints ni otros DTOs.
- Tests con Postgres real: `docker start paceron-test-db`, `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Comentarios mínimos; stagear solo rutas explícitas; sin `git add -A`; nada bajo `.superpowers/`.
- No push/merge: el usuario pushea y mergea.
- Swagger canónico: `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- Gate de coverage 85 con `go clean -cache` + `make coverage-with-db` + analyzer.

### Task 1: Dato de presencial en fila, DTO y query

**Files:**
- Modify: `cmd/api/domains/dbs/workout_feedback_history.go`
- Modify: `cmd/api/domains/workoutfeedback/workout_feedback_history.go`
- Modify: `cmd/api/daos/workout_feedback_dao.go`
- Modify: `cmd/api/services/workout_feedback_service.go`
- Test: `cmd/api/daos/workout_feedback_history_dao_test.go`

- [ ] 1.1 Agregar `IsPresencial bool` a `WorkoutFeedbackHistoryRow` y a `WorkoutFeedbackHistoryItem` (`json:"is_presencial"` sin `omitempty`).
- [ ] 1.2 Agregar `COALESCE(gcd.is_presencial, false) AS is_presencial` al `SELECT` de `HistorySearch` y mapear el campo en `historyItems`.
- [ ] 1.3 Verificar que `HistoryCount` y los pools no cambian.
- [ ] 1.4 `go build ./...` y `go vet ./cmd/api/...` verdes.

### Task 2: Tests de contrato

**Files:**
- Modify: `cmd/api/daos/workout_feedback_history_dao_test.go`
- Modify: `cmd/api/services/workout_feedback_service_history_test.go`

- [ ] 2.1 Test DAO contra Postgres: feedback de día presencial → `is_presencial=true`; feedback de día async → `false`; huérfano → `false`; el resto de los campos y el orden no cambian.
- [ ] 2.2 Test de service: el shape de `AthleteHistory` y `AdministeredHistory` incluye `is_presencial` en cada ítem y conserva los filtros actuales.
- [ ] 2.3 Correr tests focales y suite completa con Postgres real; 0 FAIL.

### Task 3: Documentación, Swagger y verificación final

**Files:**
- Modify: `docs/CATALOGO_Y_CALENDARIO.md`
- Modify: `docs/FRONTEND_IMPACTO_INSTANCIACION.md`
- Modify: `cmd/api/docs/docs.go`, `swagger.json`, `swagger.yaml` (regenerados)

- [ ] 3.1 Documentar `is_presencial` en §8.9 del catálogo y §9 del impacto frontend, incluyendo huérfanos y valor `false`.
- [ ] 3.2 Regenerar Swagger y verificar el campo en los tres artefactos.
- [ ] 3.3 `openspec validate gap-40-history-is-presencial --strict`, gofmt, `git diff --check`, suite completa y coverage ≥85.
- [ ] 3.4 Commit conventional y reporte de verificación.
