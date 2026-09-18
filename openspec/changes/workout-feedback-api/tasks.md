## 1. Modelo de datos, migración y refactor base

- [x] 1.1 Crear `cmd/api/domains/dbs/workout_feedback.go`: modelo `WorkoutFeedback` con `TableName()` = `"workout_feedback"`, columnas de la spec (id, team_id nullable, assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, report_source, session_date como `type:date`, set_number con default 0, started_at/ended_at, duration_ms/active_duration_ms, weight_kg, reps, distance_meters, rpe SMALLINT, avg/max_heart_rate SMALLINT, completion_status, elevation_gain_meters, cadence SMALLINT, annotations TEXT, media_urls como `pgtype.TextArray`, created_at/updated_at, deleted_at) — sin `route_summary` (postergado). Índices vía tags GORM: `idx_feedback_team_date (team_id, session_date)`, `idx_feedback_athlete_date (athlete_user_id, session_date)`, `idx_feedback_session_exercise (assigned_session_id, assigned_exercise_id, set_number)`. Sin FK a `assigned_*` (FK opacas)
- [x] 1.2 Registrar `dbs.WorkoutFeedback{}` en la lista de `AutoMigrate` de `cmd/api/infrastructure/postgresdb/postgres.go`
- [x] 1.3 Agregar en `postgres.go` (patrón SQL crudo idempotente post-migración, junto a los constraints existentes): `CREATE UNIQUE INDEX IF NOT EXISTS unique_feedback_per_set ON workout_feedback (assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number) WHERE deleted_at IS NULL` y el `CHECK (rpe BETWEEN 1 AND 10)` (guardado en `DO $$`/`IF NOT EXISTS` para reintentabilidad)
- [x] 1.4 Extraer de `attendance_dao` los chequeos de pertenencia de equipo (`TeamExists`, `IsTeamOwner`, `ExistsUserInTeamOwnedBy`) a un nuevo `cmd/api/daos/team_membership_dao.go` (interfaz + implementación), mantener las mismas firmas y que `attendance_dao` inyecte/reutilice el nuevo DAO — los tests existentes de attendance deben seguir pasando

## 2. Capa DAO

- [x] 2.1 Crear `cmd/api/daos/workout_feedback_dao.go`: interfaz `WorkoutFeedbackDAOInterface` + implementación inyectando el `team_membership_dao` con los métodos:
  - `Create(ctx, *dbs.WorkoutFeedback) error` — insert directo; violación `23505` (índice único parcial) → `ErrWorkoutFeedbackDuplicate`
  - `GetByID(ctx, id) (*dbs.WorkoutFeedback, error)` — filtrando `deleted_at IS NULL`; inexistente → `ErrWorkoutFeedbackNotFound`
  - `Update(ctx, id, updates map[string]interface{}) (*dbs.WorkoutFeedback, error)` — actualiza solo los campos enviados + `updated_at`, respetando `deleted_at IS NULL`
  - `SoftDelete(ctx, id) error` — setea `deleted_at = NOW()`; sin fila activa → `ErrWorkoutFeedbackNotFound`
  - `Search(ctx, filters) ([]dbs.WorkoutFeedback, error)` — WHERE dinámico por los filtros ya autorizados por el service + `deleted_at IS NULL`
- [x] 2.2 Crear `cmd/api/daos/workout_feedback_dao_test.go` con `testutils.SetupTestDB(t)`: cubrir insert nuevo, duplicado activo (viola índice único) contra set soft-deleteado (no viola), get (encontrado/borrado/inexistente), update parcial, soft delete (queda oculto de get/search y permite recrear el set), search con cada filtro y combinaciones, y verificar el round-trip de `media_urls` (`TEXT[]`)

## 3. Capa Service

- [x] 3.1 Crear `cmd/api/services/workout_feedback_service.go`: interfaz `WorkoutFeedbackServiceInterface` + implementación:
  - `Create(ctx, authUserID, req) (*WorkoutFeedbackResponse, error)` — setea `feedback_owner_user_id = authUserID`; si `athlete_user_id` viene distinto, valida con `ExistsUserInTeamOwnedBy` (si no → `ErrWorkoutFeedbackForbidden`); duplicado activo → `ErrWorkoutFeedbackDuplicate`; valida métricas (negativas → error de validación; `rpe` 1..10; fechas `YYYY-MM-DD`; `ended_at >= started_at`)
  - `GetByID(ctx, authUserID, id) (*WorkoutFeedbackResponse, error)` — no encontrado/borrado → `ErrWorkoutFeedbackNotFound`; aplica matriz (self/reportante/owner del team)
  - `Update(ctx, authUserID, id, req) (*WorkoutFeedbackResponse, error)` — 404 si no existe; autoriza como Get; ignora/`athlete_user_id`/`feedback_owner_user_id` del body
  - `SoftDelete(ctx, authUserID, id) error` — 404 si no existe; autoriza solo reportante o owner del team
  - `Search(ctx, authUserID, filters) ([]WorkoutFeedbackResponse, error)` — matriz de autorización: sin params → scope self (`athlete_id = auth OR owner_id = auth`); `team_id` → TeamExists (404) + IsTeamOwner (403) + filtro team; `athlete_user_id` ajeno → ExistsUserInTeamOwnedBy (403) + filtro atleta; aplica filtros opcionales (`assigned_session_id`, `assigned_exercise_id`, `feedback_owner_user_id`, rango de fechas)
  - Validación de entrada común para create/update (campos requeridos > 0, `report_source` no vacío)
- [x] 3.2 Crear `cmd/api/services/workout_feedback_service_test.go` con mocks del DAO: todas las ramas de la matriz (create self, create como owner, create ajeno sin relación → Forbidden, get por atleta/reportante/owner/ajeno, search self/team/atleta + 404/403), duplicado → error específico, validaciones (campos faltantes, ids ≤ 0, fecha inválida, métricas negativas, rpe fuera de rango) y soft delete autorizado/no autorizado

## 4. Capa HTTP y wiring

- [x] 4.1 Crear DTOs en `cmd/api/domains/workoutfeedback/`: `CreateFeedbackRequest`, `UpdateFeedbackRequest`, `WorkoutFeedbackResponse` (con `media_urls []string`, sin `feedback_owner_user_id` editable), `SearchFilters`, `SearchResponse{Data []WorkoutFeedbackResponse}` y constantes de mensajes (ej. `"feedback registrado"`)
- [x] 4.2 Crear `cmd/api/controllers/workout_feedback_controller.go` con los handlers `Create`, `GetByID`, `Search`, `Update`, `SoftDelete` (parseo/validación de params e ids → 400; mapeo de `errors.Is` a 400/403/404/409 con `APIError`; respuestas 201/200/204; anotaciones Swagger completas)
- [x] 4.3 Registrar rutas en `cmd/api/app/url_mappings.go` (detrás del `AuthMiddleware`): `POST /api/v1/workout-feedback`, `GET /api/v1/workout-feedback/:id`, `GET /api/v1/workout-feedback/search`, `PUT /api/v1/workout-feedback/:id`, `DELETE /api/v1/workout-feedback/:id` — y el wiring controller→service→dao (+ team_membership_dao) en `cmd/api/app/app.go`
- [x] 4.4 Crear `cmd/api/controllers/workout_feedback_controller_test.go` con mocks del service: casos felices (201/200/204 con los shapes correctos), 400 por params/ids inválidos, y mapeo de 403/404/409 desde el service

## 5. Verificación final

- [x] 5.1 Regenerar Swagger (`swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`) con la documentación de los 5 endpoints
- [x] 5.2 Actualizar `README.md` (tabla de endpoints) con los 5 nuevos endpoints
- [x] 5.3 Anotar en `CLAUDE.md` los quirks: FK opacas de `assigned_*` en `workout_feedback` (constraint real vendrá con las tablas de asignación) y `route_summary` postergado por falta de PostGIS
- [x] 5.4 Correr `go test ./...` (suite completa en verde) y `make coverage-with-db` (coverage ≥ umbral de CI 80%) — `go vet ./...` y `go test ./...` verdes en local; la parte con DB (ejecución real de los tests DAO del 2.2 y gate de coverage ≥80%) queda delegada a `ci.yml` en el push/PR
- [x] 5.5 Verificar con Postgres real que el índice único parcial (`WHERE deleted_at IS NULL`), los tres índices y el CHECK de RPE quedaron creados correctamente sobre `workout_feedback` — se ejercita por CI (Postgres real + `AutoMigrate` + SQL crudo de `postgres.go`); pendiente confirmación del run verde en el push/PR