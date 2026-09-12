# Tasks

## Tasks

### 1. Modelos, constants y migración

- [ ] `cmd/api/domains/dbs/{exercise,session,session_exercise,training_plan,plan_day}.go` (D1).
- [ ] `cmd/api/domains/constants/{exercise_kind,exercise_intensity,muscle_group,session_exercise_role,plan_day_kind}.go` (D2), mismo patrón `TeamUserRole`.
- [ ] `cmd/api/domains/trainingplan/location.go` (D4, struct `Location` compartida).
- [ ] AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- [ ] `go build ./...` verde.

### 2. DTOs de dominio

- [ ] `cmd/api/domains/exercise/{exercise_request.go,exercise_response.go}`.
- [ ] `cmd/api/domains/session/{session_request.go,session_response.go}` (incluye `SessionExerciseRequest`/`Response` embebido).
- [ ] `cmd/api/domains/trainingplan/{training_plan_request.go,training_plan_response.go}` (incluye `PlanDayRequest`/`Response` embebido, usa `Location` de la task 1).

### 3. DAOs

- [ ] `daos/exercise_dao.go` (+ `_test.go` Postgres real): `Create`, `FindByID` (excluye `deleted_at`), `FindByOwner`, `Update`, `SoftDelete`.
- [ ] `daos/session_dao.go` (+ test): igual shape que exercise_dao.
- [ ] `daos/session_exercise_dao.go` (+ test): `Create`, `FindBySession`, `ReplaceForSession(sessionID, rows)` (borra todas + inserta el set nuevo, en una transacción).
- [ ] `daos/training_plan_dao.go` (+ test): `Create`, `FindByID`, `FindByOwner`, `Update`, `Delete` (físico).
- [ ] `daos/plan_day_dao.go` (+ test): `FindByPlan`, `ReplaceForPlan(planID, rows)` (mismo patrón que session_exercise_dao).

### 4. Servicios

- [ ] `services/exercise_service.go`: `Create`, `Update`, `Delete` (soft), `Clone`, `Get`, `List(ownerID)`. Sentinel errors (`ErrExerciseNotFound`, etc.), sin `apierror.APIError` — este dominio usa `{message}` plano (D6).
- [ ] `services/session_service.go`: mismo shape + validación de roles (D5) al crear/editar; `Clone` copia profunda de `session_exercises`.
- [ ] `services/training_plan_service.go`: mismo shape + validación de `days` (D5); `Clone` copia profunda de `plan_days`; `Delete` es físico.
- [ ] Tests unitarios (mocks de DAOs, molde `join_request_service_test.go`): validaciones de D5 + camino feliz + `Clone` de cada uno.

### 5. Controllers y rutas

- [ ] `controllers/exercise_controller.go`, `controllers/session_controller.go`, `controllers/training_plan_controller.go` — helper compartido `respondCatalogError` (D6).
- [ ] 18 rutas nuevas en `cmd/api/app/url_mappings.go`, detrás de `AuthMiddleware()`.
- [ ] Wiring en `cmd/api/app/app.go`.
- [ ] Tests de controller (httptest) por cada código de error + camino feliz.

### 6. Swagger y documentación

- [ ] Regenerar `cmd/api/docs`.
- [ ] Actualizar tabla de endpoints en `README.md`.

### 7. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` verde.
- [ ] `make coverage-with-db` — confirmar que no rompe el gate de 80%.
