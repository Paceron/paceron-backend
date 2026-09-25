# Design — workout-feedback-gps-points

## Tabla `workout_feedback_points`

```sql
CREATE TABLE IF NOT EXISTS workout_feedback_points (
    id                   BIGSERIAL PRIMARY KEY,
    feedback_id          BIGINT NOT NULL,
    session_instance_id  BIGINT NOT NULL,
    exercise_instance_id BIGINT NOT NULL,
    "order"              INTEGER NOT NULL,
    latitude             DOUBLE PRECISION NOT NULL,
    longitude            DOUBLE PRECISION NOT NULL,
    recorded_at          TIMESTAMPTZ NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_feedback_point_order
    ON workout_feedback_points (feedback_id, "order");
```

### Decisiones

- **FK opaca a `workout_feedback`** (sin constraint ni `ON DELETE`), igual que `assigned_session_id`/`assigned_exercise_id` en `workout_feedback`: la integridad la gobierna el service (no se insertan puntos de un feedback que no existe/está activo) y un hipotético borrado del feedback no arrastra ni rompe el recorrido histórico. Consistente con la convención del repo ("FK opacas, constraints reales en un change futuro").
- **`session_instance_id`/`exercise_instance_id` denormalizadas (NOT NULL)**: el frontend las manda siempre en el payload del puntos (`buildPointsPayload`), y listar el recorrido por sesión o ejercicio sin join a `workout_feedback` es el caso de uso del historial futuro. El costo de la redundancia es nulo (datos inmutables del set).
- **`"order"` entre comillas**: `ORDER` es palabra reservada de SQL — la columna se llama `"order"` en DB y `Order` en Go/GORM (GORM ya la escapa solo por ser field `Order`).
- **`DOUBLE PRECISION` para lat/lon**: precisión ~15 dígitos, suficiente para el sampleo GPS; `geography(Point)`/postgis queda descartado (misma razón que `route_summary`, no disponible en Supabase/CI).
- **Índice único en `postgres.go`** (SQL crudo idempotente `IF NOT EXISTS`), no en la struct GORM: el repo ya define los índices no-triviales en ese archivo (`unique_feedback_per_set`, `uq_seller_connections_user_client`, …). AutoMigrate crea la tabla (columnas), el índice llega por el SQL post-migración.
- **Idempotencia por bulk hecha en la DB**: `INSERT ... ON CONFLICT (feedback_id, "order") DO NOTHING` con `RETURNING id` no se usa; en su lugar se cuenta con `RowCount - skipped` de la diff (ver DAO). Reintentar el mismo bulk nunca duplica ni errora.

## Bulk insert y conteo (DAO)

`BulkCreatePoints(ctx, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (created int64, err error)`:

```go
res := tx.Create(&points)
created = res.RowsAffected - alreadyExisting
```

La idempotencia real la da el índice único (un segundo insert del mismo `"order"` viola el unique → la lib envuelve el error en `pgconn.PgError` 23505). Para poder hacer `ON CONFLICT DO NOTHING` con GORM se usa el **cláusula `clause.OnConflict`**:

```go
tx.Clauses(clause.OnConflict{
    Columns:   []clause.Column{{Name: "feedback_id"}, {Name: "order"}},
    DoNothing: true,
}).Create(&points)
```

Con `DoNothing`, `RowsAffected` cuenta los insertados logrados. `created = RowsAffected`, `skipped = len(points) - created`. Sin transacción explícita: un solo `Create` batch es atómico en Postgres.

## Autorización

Los nuevos endpoints reusan `canAccess` tal cual: atleta (`athlete_user_id == auth`), reportante (`feedback_owner_user_id == auth`) u owner del team (`team_id` presente y `teams.owner_id == auth`). El service primero resuelve el feedback por id (`GetByID` → puede devolver `ErrWorkoutFeedbackNotFound`), autoriza, y recién entonces toca los puntos. Misma matriz que `GetByID`/`Update`/`Delete` del módulo.

## Validación de `CreatePointsRequest`

- `points` no vacío, máximo arbitrario capado en 5.000 (una serie larga a 1 punto/s ~ 1h30m; capa de saneamiento frente a bulks absurdos, no un límite de dominio).
- Por punto: `"order" >= 0` (0-based), `latitude` en `[-90,90]`, `longitude` en `[-180,180]`, `recorded_at` parseable como RFC3339.
- Errores de validación → `ErrWorkoutFeedbackInvalid` → 400.
- Feedback inexistente/soft-borrado → `ErrWorkoutFeedbackNotFound` → 404. No autorizado → `ErrWorkoutFeedbackForbidden` → 403.

## Respuestas

- `POST` → `201 { message: "puntos registrados", data: { created, skipped } }`.
- `GET` → `200 { data: [ { id, feedback_id, session_instance_id, exercise_instance_id, order, latitude, longitude, recorded_at } ] }` ordenado por `"order"`.

## Fuera de alcance

- Edición/borrado puntual de un punto.
- Diferencia entre `distance_meters` del feedback y la suma de segmentos de `workout_feedback_points` (fuentes distintas; el frontend manda ambos).