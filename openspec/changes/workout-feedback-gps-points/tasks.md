# Tareas — workout-feedback-gps-points

## 1. Modelo y migración

- [ ] 1.1 Crear `cmd/api/domains/dbs/workout_feedback_point.go` con `WorkoutFeedbackPoint` (columna `"order"` → field `Order`; `TableName()` → `workout_feedback_points`). AutoMigrate crea la tabla.
- [ ] 1.2 Agregar en `cmd/api/infrastructure/postgresdb/postgres.go` (junto a `unique_feedback_per_set`): `CREATE UNIQUE INDEX IF NOT EXISTS uq_feedback_point_order ON workout_feedback_points (feedback_id, "order");`
- [ ] 1.3 Verificar nombres/tipos contra el contrato del frontend (`session_instance_id`, `exercise_instance_id`, `"order"`, `latitude`, `longitude`, `recorded_at`).

## 2. DTOs

- [ ] 2.1 En el dominio `workoutfeedback`: `CreatePointsRequest { points: []PointInput }` con `Order`, `SessionInstanceID`, `ExerciseInstanceID` (*int64 nullable o int64 — decidir con el contrato), `Latitude`, `Longitude`, `RecordedAt time.Time`.
- [ ] 2.2 `PointsMutationResponse { message, data: PointsMutationData { created, skipped } }`.
- [ ] 2.3 `WorkoutFeedbackPointResponse` (shape plano espejo del modelo).

## 3. DAO

- [ ] 3.1 `BulkCreatePoints(ctx, feedbackID, points []dbs.WorkoutFeedbackPoint) (int64, error)`: `clause.OnConflict{Columns: [feedback_id, order], DoNothing: true}` + `Create`; devuelve `RowsAffected`.
- [ ] 3.2 `GetPointsByFeedback(ctx, feedbackID) ([]dbs.WorkoutFeedbackPoint, error)` ordenado por `"order"`.
- [ ] 3.3 Expandir `WorkoutFeedbackDAOInterface`.

## 4. Service

- [ ] 4.1 `CreatePoints(ctx, authUserID, feedbackID, req) (*PointsResult, error)`: resuelve feedback (`GetByID`), `canAccess`, valida array no vacío, máximo 5.000, rango lat/lon, `order >= 0`, `recorded_at` parseable; delega en `BulkCreatePoints`; devuelve `{ created, skipped }`.
- [ ] 4.2 `GetPoints(ctx, authUserID, feedbackID) ([]dbs.WorkoutFeedbackPoint, error)`: resuelve feedback, `canAccess`, delega en `GetPointsByFeedback`.
- [ ] 4.3 Expandir `WorkoutFeedbackServiceInterface`.

## 5. Controller y rutas

- [ ] 5.1 Handlers `CreatePoints` (201) y `GetPoints` (200) con parsing del path param, reuso de `respondFeedbackError` y mappers a responses.
- [ ] 5.2 Swagger annotations godoc para las 2 rutas.
- [ ] 5.3 Registrar `POST /api/v1/workout-feedback/:id/points` y `GET /api/v1/workout-feedback/:id/points` en `url_mappings.go`. Registrar `:id/points` ANTES de `:id` si Gin pudiera colisionar (verificar orden).
- [ ] 5.4 Regenerar swagger.

## 6. Tests

- [ ] 6.1 DAO test: bulk insert crea, reintentar el mismo `"order"` no duplica (`skipped` correcto), listado ordenado.
- [ ] 6.2 Service test: validación (array vacío, lat/lon fuera de rango, order negativo, feedback inexistente, no autorizado → matrices atleta/reportante/owner/no-owner), created/skipped calculados.
- [ ] 6.3 Controller test: 201, 400 (validación), 403 (no autorizado), 404 (feedback inexistente), 200 list.
- [ ] 6.4 `go test ./...` del módulo en verde; cobertura del lote >= 85%.

## 7. Docs

- [ ] 7.1 `docs/` swagger regenerado.
- [ ] 7.2 Nota en CLAUDE.md del backend sobre la tabla nueva (FK opaca a workout_feedback, idempotencia por `(feedback_id, "order")`).