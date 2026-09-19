# Workout Feedback API

## Why

Los corredores de Paceron registran sus entrenamientos con métricas de rendimiento (peso, repeticiones, distancia, RPE, pulso, duración, notas, multimedia) y los entrenadores necesitan consultar y administrar ese feedback de sus equipos. Hoy no existe ningún modelo ni endpoint para persistir ese feedback: todo el conocimiento del entrenamiento queda fuera de la plataforma. Este change agrega la tabla `workout_feedback` y el API REST que la administra, respetando el script SQL definido en la spec.

## Objetivo

Habilitar el CRUD completo + búsqueda de feedback de entrenamiento: el atleta (o el entrenador en su nombre) reporta feedback de una sesión/ejercicio asignado, y el entrenador puede consultar, editar y dar de baja (soft delete) los feedbacks de los corredores de sus equipos. Todo detrás del `AuthMiddleware`, con baja lógica (`deleted_at`).

## Alcance

- Nueva tabla `workout_feedback` con la estructura exacta de la spec (columnas, índices, restricción única de "un feedback por set") y baja lógica.
- Endpoints bajo el prefijo `/api/v1` (consistente con el resto del repo): `POST /api/v1/workout-feedback`, `GET /api/v1/workout-feedback/:id`, `GET /api/v1/workout-feedback/search`, `PUT /api/v1/workout-feedback/:id`, `DELETE /api/v1/workout-feedback/:id`.
- Matriz de autorización: el atleta opera sobre su propio feedback; el entrenador (owner del equipo) opera sobre los feedbacks de los corredores de sus equipos.
- Manejo de errores estandarizado (`APIError`) y Swagger con las anotaciones de los controllers.

## No alcance

- `route_summary` (`GEOGRAPHY(LineStringZ, 4326)`): **postergado** — requiere la extensión PostGIS que todavía no está disponible en Supabase (testing/prod) ni en CI. Se agrega en un change futuro sin tocar el API.
- FK reales a `assigned_session_id` / `assigned_exercise_id`: las tablas de sesiones/ejercicios **asignados** no existen aún; se modelan como FK opacas (`BIGINT > 0`), igual que `training_sessions` en attendances. La constraint real vendrá con el change que cree esas tablas.
- CRUD de sesiones/ejercicios asignados, grupos, equipos.
- Estadísticas, reportes, exportación o agregaciones sobre el feedback.
- Push notifications al cargar feedback.
- Upload de multimedia (solo se persisten URLs en `media_urls`).
- Modelo completo de `training_sessions` / asignaciones (change futuro).

## Métrica de éxito

- `go test ./...` en verde con coverage global ≥ 80% (umbral de CI).
- Los 5 endpoints responden 400/401/403/404/409 acorde a la matriz y validaciones (tests de controller + service con mocks).
- DAO verificado contra Postgres real (`testutils.SetupTestDB`): la restricción única rechaza feedbacks duplicados del mismo set y el soft-delete los oculta de búsquedas y lectura.

## What Changes

- Nueva tabla `workout_feedback` (id, team_id nullable, assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, report_source, session_date, set_number, métricas de tiempo/carga/rendimiento/avanzadas/estado, annotations, media_urls TEXT[], timestamps de auditoría + `deleted_at`) con:
  - Restricción única de "un feedback activo por set": índice único **parcial** `unique_feedback_per_set` sobre `(assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number)` con `WHERE deleted_at IS NULL` (ver decisión en design — desviación intencional del `UNIQUE` plano de la spec para que el soft-delete no bloquee el recreado).
  - `idx_feedback_team_date (team_id, session_date)`, `idx_feedback_athlete_date (athlete_user_id, session_date)`, `idx_feedback_session_exercise (assigned_session_id, assigned_exercise_id, set_number)`.
  - `CHECK (rpe BETWEEN 1 AND 10)`.
- Modelo GORM `WorkoutFeedback` en `cmd/api/domains/dbs/` + migración en `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate + SQL crudo idempotente para índice único parcial y CHECK).
- Cinco endpoints (todos detrás del `AuthMiddleware`, prefijo `/api/v1`):
  - `POST /api/v1/workout-feedback` → crea feedback (201). `feedback_owner_user_id` se setea siempre con el `auth_user_id` del token; `athlete_user_id` distinto al autenticado solo si es entrenador de un equipo del atleta.
  - `GET /api/v1/workout-feedback/:id` → detalle por id (200 / 404).
  - `GET /api/v1/workout-feedback/search` → búsqueda con filtros (`team_id`, `athlete_user_id`, `assigned_session_id`, `assigned_exercise_id`, `session_date_from`, `session_date_to`, `feedback_owner_user_id`) y matriz de autorización.
  - `PUT /api/v1/workout-feedback/:id` → edición parcial del feedback (200 / 404 / 403).
  - `DELETE /api/v1/workout-feedback/:id` → baja lógica (204 / 404 / 403).
- Nueva capability `workout-feedback` con sus requirements: persistencia/índices, validaciones de entrada, matriz de autorización (self vs. entrenador/owner), idempotencia de la restricción única (409 sobre duplicado activo) y soft-delete.
- Documentación Swagger con las anotaciones de los controllers (regenerada, no editada a mano).

## Capabilities

### New Capabilities

- `workout-feedback`: persistencia y administración de feedback de entrenamiento (CRUD + búsqueda) con baja lógica, validaciones y matriz de autorización self/entrenador.

### Modified Capabilities

- (ninguna — no se cambian requirements de capabilities existentes)

## Impact

**Código nuevo:**
- `cmd/api/domains/dbs/workout_feedback.go` — modelo GORM de la tabla `workout_feedback`.
- `cmd/api/domains/workoutfeedback/` — DTOs de request/response del dominio (create/update/search/get).
- `cmd/api/domains/constants/workout_feedback_*.go` — constantes del dominio si aplican (mensajes de error, scopes de búsqueda).
- `cmd/api/daos/workout_feedback_dao.go` — acceso a datos (create con captura de UNIQUE, get, update, soft delete, search con WHERE dinámico, chequeos de ownership de equipo).
- `cmd/api/services/workout_feedback_service.go` — lógica de negocio (validaciones de métricas, matriz de autorización, mapeo DTO→modelo).
- `cmd/api/controllers/workout_feedback_controller.go` — handlers HTTP de los cinco endpoints + tests.
- `cmd/api/app/url_mappings.go` + `app.go` — wiring de rutas y dependencias.

**Código modificado:**
- `cmd/api/infrastructure/postgresdb/postgres.go` — registro del modelo en `AutoMigrate` + SQL crudo para índice único parcial y CHECK.
- `cmd/api/docs/` — regeneración de Swagger (no se edita a mano).
- `README.md` — tabla de endpoints (nuevos endpoints).

**Decisiones de diseño relevantes:**
- `assigned_session_id` / `assigned_exercise_id` como FK opacas: no existen tablas de asignación aún; el change futuro que las cree agregará las constraints.
- La restricción única de la spec se implementa como índice único **parcial** (`WHERE deleted_at IS NULL`) para que el soft-delete no impida recrear el mismo set — ver design.
- `route_summary` no se implementa (sin PostGIS); queda anotado como quirk pendiente en CLAUDE.md.