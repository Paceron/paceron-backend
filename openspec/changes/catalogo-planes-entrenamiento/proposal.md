## Why

El frontend tiene el módulo completo de catálogo de entrenamiento (ejercicios, sesiones, planes) corriendo 100% contra mocks in-memory desde hace meses — `services/exercises.js`, `services/sessions.js` y `services/trainingPlans.js` ya tienen las rutas REST esperadas escritas, pero no hay ningún endpoint real detrás (Gap 4 de `paceron-frontend/docs/BACKEND_API_GAPS.md`). Desde 2026-09-07 esto está forzado con `FORCE_MOCKS = true` en cada archivo porque pegarle a las rutas inexistentes en un build real causaba un logout espurio.

La spec completa del contrato (entidades, validaciones, endpoints) ya está cerrada del lado frontend en `paceron-frontend/docs/BACKEND_TRAINING_PLANS_SPEC.md` — este change la implementa tal cual, sin reabrir decisiones de producto ya tomadas ahí (fechadas, ej. "sin caducidad propia del plan" 2026-09-10, "`description` de ejercicio opcional" 2026-09-11).

Este es el primero de dos changes secuenciales sobre la misma rama — el catálogo va primero porque el calendario (`calendario-asignacion-grupos`) referencia `Session`/`TrainingPlan` como si ya existieran.

## What Changes

- **Nuevo dominio de catálogo reusable**, propiedad de un entrenador (`owner_id`), no atado a ningún equipo:
  - `Exercise`: `kind`/`intensity`/`muscle_group` totalmente independientes entre sí (sin combinación obligatoria por tipo), `description` opcional.
  - `Session`: contiene N `SessionExercise` (tabla propia, no array embebido) con `role` (`warmup`/`main`/`cooldown`) — regla: al menos 1 ejercicio de cada rol.
  - `TrainingPlan`: contiene entre 2 y 31 `PlanDay` secuenciales (`sequence_no`, sin atarse a día de semana), cada uno `rest`/`other`/`training`.
- **Endpoints CRUD + clone** para las 3 entidades de nivel superior (`/exercises`, `/sessions`, `/training-plans`), sin paginación (filtro simple por `owner_id`).
- **Borrado lógico** (`deleted_at`) de `Exercise`/`Session` — quedan resolviendo correctamente en sesiones/planes que ya las referencian, pero no aparecen en listados nuevos. Decisión de esta implementación (la spec de frontend lo deja abierto), consistente con el resto del repo (`teams`, `invitations`, etc.).
- **`PUT` de `Session`/`TrainingPlan` reemplaza el conjunto hijo entero** (`exercises`/`days`), no hace patch fila por fila — el frontend siempre reenvía la lista completa.
- **`clone`** de las 3 entidades: copia profunda con sufijo `" (copia)"` en el nombre, sin heredar usos (una sesión clonada no hereda las asignaciones de calendario de la original — eso lo cubre el próximo change).

## Capabilities

### New Capabilities

- `exercise-catalog`: CRUD + clone de ejercicios del catálogo de un entrenador.
- `session-catalog`: CRUD + clone de sesiones, con su lista de `SessionExercise` embebida en la respuesta y reemplazada completa en cada `PUT`.
- `training-plan-catalog`: CRUD + clone de planes de entrenamiento, con sus `PlanDay` embebidos y validados (2-31 filas, `sequence_no` sin huecos ni repetidos, combinación `kind`/`other_name`/`session_id` coherente).

## Non-Goals

- **Calendario/asignación a un grupo** — es el segundo change de esta rama (`calendario-asignacion-grupos`), depende de que este exista primero.
- **`video_url` con carga real** — el campo existe reservado en `Exercise`, siempre `null`, sin endpoint de subida (fuera de alcance, spec frontend §6).
- **`holdSeconds`** de elongación — no existe todavía, spec frontend §6.
- **Unidad alternativa de `speed_kph`** (min/km) — se evaluó y descartó del lado frontend, no se agrega acá.
- **Endpoints de batch** (clonar/eliminar N de una) — el frontend repite la request individual por ítem, no hay ni se espera `bulk-delete`/similar para el catálogo (distinto del calendario, que sí tiene sus propios `bulk`/`bulk-clear` en el próximo change).
- **Gap 5** (`GET /tiers/{id}/permissions`) — gap real y abierto, pero de un dominio no relacionado; no se toca en esta rama.

## Impact

- **Schema/DB**: tablas nuevas `exercises`, `sessions`, `session_exercises`, `training_plans`, `plan_days`. Aditivo. AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- **Dominios/DTOs**: `cmd/api/domains/dbs/{exercise,session,session_exercise,training_plan,plan_day}.go`; `cmd/api/domains/constants/{exercise_kind,exercise_intensity,muscle_group,session_exercise_role,plan_day_kind}.go`; `cmd/api/domains/{exercise,session,trainingplan}/` (request/response).
- **DAOs**: `cmd/api/daos/{exercise,session,session_exercise,training_plan,plan_day}_dao.go`.
- **Servicios**: `cmd/api/services/{exercise,session,training_plan}_service.go` — sin delegate, cada uno resuelve su propia composición de DAOs (mismo criterio que `invitation_service.go`, nada acá compone lógica de 2 services distintos).
- **Controllers/rutas**: `cmd/api/controllers/{exercise,session,training_plan}_controller.go`, rutas nuevas en `cmd/api/app/url_mappings.go`, todas detrás de `AuthMiddleware()`.
- **Swagger**: regenerar `cmd/api/docs`.
- **Tests**: DAOs con Postgres real (`testutils.SetupTestDB`), services/controllers con mocks — mismo molde que `join_request_service_test.go`/`join_request_controller_test.go` de la feature de búsqueda de equipos.
