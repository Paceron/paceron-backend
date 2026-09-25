# Spec — workout-feedback-gps-points

## Requirement: El sistema SHALL persistir el recorrido GPS de una serie

El sistema SHALL persistir los puntos GPS de una serie de ejercicio en la tabla `workout_feedback_points` con las columnas: `id` (BIGSERIAL PK), `feedback_id` (BIGINT NOT NULL, FK opaca al `workout_feedback` — sin constraint ni cascade), `session_instance_id` (BIGINT NOT NULL, denormalizada), `exercise_instance_id` (BIGINT NOT NULL, denormalizada), `"order"` (INTEGER NOT NULL, ordinal 0-based del punto dentro de la serie), `latitude` (DOUBLE PRECISION NOT NULL), `longitude` (DOUBLE PRECISION NOT NULL), `recorded_at` (TIMESTAMPTZ NOT NULL), `created_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW()). La tabla SHALL tener un índice único `uq_feedback_point_order` sobre `(feedback_id, "order")` que garantice "un punto por posición de serie".

### Scenario: La migración crea la tabla y el índice único

- **WHEN** se corre la migración (AutoMigrate + SQL de inicialización)
- **THEN** existe la tabla `workout_feedback_points` con las columnas de la spec y el índice único `uq_feedback_point_order`

## Requirement: El sistema SHALL crear puntos de una serie de forma idempotente

El sistema SHALL exponer `POST /api/v1/workout-feedback/:id/points` que recibe `{ "points": [ { "order", "session_instance_id", "exercise_instance_id", "latitude", "longitude", "recorded_at" } ] }`. El endpoint SHALL requerir un `workout_feedback` activo (404 si no existe o está soft-borrado) y autorizar con la misma matriz del módulo de feedback (403 si el usuario no es atleta, reportante u owner del team). La inserción SHALL ser un bulk `INSERT ... ON CONFLICT (feedback_id, "order") DO NOTHING`: reintentar el mismo bulk no duplica puntos ni falla, y la respuesta SHALL informar `{ message, data: { created, skipped } }` con los conteos reales. La validación SHALL rechazar (400): array vacío, más de 5.000 puntos, `"order"` negativo, latitud fuera de `[-90, 90]`, longitud fuera de `[-180, 180]` o `recorded_at` no parseable como RFC3339.

### Scenario: Alta de puntos válidos

- **WHEN** un usuario autorizado hace POST `:id/points` con un array de puntos válido
- **THEN** se crean los puntos, se responde 201 con `{ created: N, skipped: 0 }`, y la respuesta incluye `message`

### Scenario: Reintento del mismo bulk

- **WHEN** el mismo array de puntos (mismos `"order"`) se envía dos veces sobre el mismo feedback
- **THEN** el segundo POST no duplica filas y responde `{ created: N, skipped: N }`

### Scenario: Feedback inexistente

- **WHEN** se hace POST `:id/points` con un id de feedback inexistente o soft-borrado
- **THEN** se responde 404

### Scenario: Usuario no autorizado

- **WHEN** un usuario que no es atleta, reportante ni owner del team hace POST `:id/points`
- **THEN** se responde 403

### Scenario: Puntos inválidos

- **WHEN** el array viene vacío, un `"order"` es negativo, una latitud/longitud está fuera de rango o `recorded_at` no parsea
- **THEN** se responde 400

## Requirement: El sistema SHALL listar los puntos de una serie

El sistema SHALL exponer `GET /api/v1/workout-feedback/:id/points` que devuelve `{ data: [ { id, feedback_id, session_instance_id, exercise_instance_id, order, latitude, longitude, recorded_at } ] }` ordenado por `"order"`, con la misma autorización (atleta, reportante u owner del team) y 404 si el feedback no existe.

### Scenario: Listado autorizado

- **WHEN** un usuario autorizado consulta el recorrido de una serie con puntos
- **THEN** se responden los puntos ordenados por `"order"` en `data`

### Scenario: Feedback inexistente

- **WHEN** se consulta el recorrido de un feedback inexistente o soft-borrado
- **THEN** se responde 404