## Why

El registro de actividad en vivo del frontend (ver `paceron-frontend/docs/superpowers/specs/2026-09-24-live-session-recording-design.md`) graba cada serie de ejercicio durante la sesión y, al finalizar, popula el backend con un `workout_feedback` por serie. Las series con GPS (carreras/excursiones) además acumulan el recorrido punto a punto (sampleo ~1s), lógico en SQLite en el dispositivo. Hoy `workout_feedback` solo tiene `distance_meters` agregado — no hay dónde persistir el recorrido: el `route_summary` GEOGRAPHY/postgis del change `workout-feedback-api` quedó postergado y no es un formato de serie (son puntos discretos, no una ruta en linestring). Necesitamos una tabla de puntos y dos endpoints para subirlos y leerlos, alineados al contrato que el frontend ya genera.

## What Changes

- **Tabla nueva `workout_feedback_points`**: `id BIGSERIAL PK`, `feedback_id BIGINT NOT NULL` (FK opaca al `workout_feedback` activo — no hay constraint ni cascade: si el feedback se soft-borrase, los puntos se conservan como historial), `session_instance_id BIGINT NOT NULL` y `exercise_instance_id BIGINT NOT NULL` (denormalizadas a propósito para consultar por sesión/ejercicio sin join), `"order" INT NOT NULL` (ordinal 0-based de la serie dentro del set), `latitude DOUBLE PRECISION NOT NULL`, `longitude DOUBLE PRECISION NOT NULL`, `recorded_at TIMESTAMPTZ NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. Índice único `uq_feedback_point_order (feedback_id, "order")` → **la idempotencia de reintentar un bulk la da la DB**, no el código: `INSERT ... ON CONFLICT (feedback_id, "order") DO NOTHING` y se informan `created`/`skipped`.
- **`POST /api/v1/workout-feedback/:id/points`** — bulk upsert idempotente de puntos de una serie. Requiere feedback activo, autoriza con la **misma matriz del módulo de feedback** (atleta, reportante u owner del team — mismas reglas de `canAccess`), valida que el array no venga vacío y que cada punto tenga `"order" >= 0`, `latitude` en `[-90,90]`, `longitude` en `[-180,180]` y `recorded_at` parseable. Responde `{ message, data: { created, skipped } }`.
- **`GET /api/v1/workout-feedback/:id/points`** — lista el recorrido de esa serie (para el historial futuro, Gap 12 del frontend), ordenada por `"order"`. Misma autorización.
- **Nada nuevo sobre sets**: la idempotencia del propio `POST /workout-feedback` ya la cubre `unique_feedback_per_set` (409).
- `dgswagger` se regenera (dos rutas nuevas).

## Capabilities

### New Capabilities

- `workout-feedback-points`: persistencia y consulta del recorrido GPS de una serie de `workout_feedback`, con alta idempotente por `(feedback_id, "order")` y autorización heredada de la matriz del módulo de feedback.

## Impact

- Modelos: `domains/dbs/workout_feedback_points.go` (GORM, AutoMigrate crea la tabla; índice único en `postgres.go` junto a `unique_feedback_per_set`).
- DTOs: `domains/workoutfeedback/workout_feedback.go` (requests/responses de points) o archivo hermano del dominio.
- DAO: `daos/workout_feedback_dao.go` (`BulkCreatePoints`, `GetPointsByFeedback`) + interfaz.
- Service: `services/workout_feedback_service.go` (`CreatePoints`, `GetPoints`) + interfaz, reusando `canAccess`.
- Controller: `controllers/workout_feedback_controller.go` (handlers + swagger annotations) + registros en `app/url_mappings.go`.
- Swagger: `docs/` regenerado.
- Docs: `docs/BACKEND_API_GAPS.md` del frontend (cierra el hueco de persistencia de recorrido del Gap 12, parcial), mensaje en `CLAUDE.md` del frontend si aplica.
- Tests: DAO (insert con conflict/dedup, listado), service (validación + matriz de auth), controller (201/400/403/404) — `testify`, patrón de `workout_feedback_*_test.go`. Cobertura del repo se mantiene >= 85%.

## Non-Goals

- Edición/borrado de puntos individuales: los puntos son un log de la serie, no un recurso editable; se reemplazan/consultan completos. El `DELETE /workout-feedback/:id` (baja lógica) no toca los puntos.
- Backfill/conciliación de distancias: `distance_meters` del feedback y los puntos del recorrido se cargan por canales distintos y pueden divergir (el frontend decide la fuente de verdad de la muestra).
- PostGIS/`route_summary`: sigue postergado (requiere la extensión, fuera de alcance de Supabase/CI).