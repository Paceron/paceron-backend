# Diseño — Workout Feedback API

## Context

Paceron es una app de running donde cada equipo tiene un owner (el entrenador, `teams.owner_id`) y miembros (`team_users` con `role_in_team` = `entrenador`/`corredor`). Existen catálogos de `sessions` (plantillas de entrenamiento compuestas por `session_exercises`) y `exercises`, más un calendario por grupo (`group_calendar_days`) que asigna una `session` a una fecha. Lo que NO existe aún: el concepto de **asignación** (sesión/ejercicio asignado a un corredor concreto, tables `assigned_*` que un change futuro creará) ni ningún lugar donde persistan las métricas reales de un entrenamiento realizado.

Este change agrega la tabla `workout_feedback` (spec del equipo) y el API REST que la administra: el atleta o el entrenador reporta feedback sobre una sesión/ejercicio asignado, y el entrenador puede consultarlo, editarlo y darlo de baja (soft delete). `requester_id`/`auth_user_id` y rol vienen expuestos por el `AuthMiddleware` en el contexto de Gin (`utils.GetAuthUserID`).

Stack: Go 1.26 + Gin + GORM/PostgreSQL. Arquitectura en capas (Controllers → Services → DAOs → Infrastructure), DI manual en `app.go`, `AutoMigrate` centralizado en `cmd/api/infrastructure/postgresdb/postgres.go` (con patrón de SQL crudo idempotente post-migración para constraints/índices que GORM no expresa por tags), tests con `testify` (services/controllers con mocks) y DAOs con Postgres real vía `testutils.SetupTestDB` (skippable sin `TEST_DB_HOST`).

Constraints del spec:
- La tabla usa el script SQL exacto como referencia de columnas, índices y restricción única sobre `(assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number)`.
- `route_summary` (`GEOGRAPHY(LineStringZ, 4326)`) se **posterga**: requiere la extensión PostGIS, no disponible en Supabase testing/prod ni en CI.
- Baja lógica obligatoria (`deleted_at`) y JWT en todo request.

Decisiones de scope acordadas: CRUD completo + search bajo `/api/v1`, `route_summary` postergado, y tanto atleta como entrenador pueden reportar (`feedback_owner_user_id` ≠ `athlete_user_id` es el caso "entrenador reporta por el atleta").

## Goals / Non-Goals

**Goals:**
- Tabla `workout_feedback` con todas las columnas de la spec (menos `route_summary`), los tres índices de búsqueda, el índice único de set y el CHECK de RPE.
- 5 endpoints detrás del `AuthMiddleware` con prefijo `/api/v1/workout-feedback`: create, get-by-id, search, update, delete (baja lógica).
- `feedback_owner_user_id` siempre = `auth_user_id` (el reportante real nunca viaja en el body); el atleta se infiere de `auth_user_id` salvo que un entrenador autorizado lo indique.
- Matriz de autorización basada en la membresía real (`teams.owner_id` + `team_users`), consistente con la de asistencias.
- Manejo de errores uniforme vía `APIError` (400/401/403/404/409).
- Soft-delete real: un feedback dado de baja queda oculto de get/search/update y no bloquea recrear el mismo set (índice único parcial).

**Non-Goals:**
- Crear las tablas de asignación `assigned_sessions`/`assigned_exercises` (change futuro; acá `assigned_*` son FK opacas `> 0`).
- `route_summary`/PostGIS (postergado).
- Upload de multimedia (solo se persisten URLs).
- Estadísticas, reportes, exportaciones o agregaciones.
- Push notifications al cargar feedback.
- Validar que el atleta sea miembro del equipo al momento de crear (el reporte puede ocurrir en un equipo donde la relación se resuelve igual por la matriz de entrenador; para el caso self no se exige membresía, consistente con asistencias).

## Decisions

### 1. `assigned_session_id` / `assigned_exercise_id` como FK opacas

Se modelan como `int64` `not null`, sin `ForeignKey` en GORM. Único contrato: `> 0` (validado en controller/service). Cuando el change de tablas de asignación llegue, agrega las constraints reales. Mismo criterio ya usado en `attendances.training_session_id`.

**Alternativa descartada:** crear tablas `assigned_*` mínimas. Produce migraciones y deuda sin el modelo real definido.

### 2. Índice único **parcial** en vez del `UNIQUE` plano de la spec

La spec pide `ALTER TABLE workout_feedback ADD CONSTRAINT unique_feedback_per_set UNIQUE (assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number)`. Combinado con la baja lógica obligatoria, un UNIQUE plano es contradictorio: tras soft-deletear un feedback, sería **imposible** recrear ese mismo set (el registro eliminado sigue ocupando el slot del UNIQUE para siempre). Se implementa `CREATE UNIQUE INDEX IF NOT EXISTS unique_feedback_per_set ON workout_feedback (...) WHERE deleted_at IS NULL` — el mismo patrón de índice único parcial ya usado en el repo para suscripciones vigentes (`uq_sub_ids_user_role_active`). Efecto neto: "un feedback **activo** por set".

**Alternativa descartada:** el UNIQUE plano de la spec literal. El soft-delete + UNIQUE plano es un deadlock funcional (borrado no recuperable vía API). Se documenta como desviación en proposal e Impact, y si el equipo quiere el UNIQUE plano estricto, se acepta la limitación de no poder recrear sets eliminados.

### 3. Rutas y prefijo `/api/v1`

Endpoint base `workout-feedback` (singular, snake_case, alineado con `attendance`/`payment` del repo):

- `POST /api/v1/workout-feedback` — crear (201/400/401/403/409).
- `GET /api/v1/workout-feedback/:id` — detalle (200/401/403/404).
- `GET /api/v1/workout-feedback/search` — buscar (200/400/401/403/404).
- `PUT /api/v1/workout-feedback/:id` — editar parcial (200/400/401/403/404).
- `DELETE /api/v1/workout-feedback/:id` — baja lógica (204/401/403/404).

Todos detrás del `AuthMiddleware` (registrados después de `r.Use(AuthMiddleware())`).

### 4. Capas y dependencias

Controller → `workout_feedback_service` → `workout_feedback_dao`. Sin delegates: cada endpoint usa un solo servicio. El service no conoce Gin (recibe contexto GORM vía `*gin.Context` como hacen los services del repo y construye DTOs). El DAO expone operaciones atómicas y, para la matriz de equipo, **reutiliza los chequeos que ya viven en `attendance_dao`** (TeamExists, IsTeamOwner, ExistsUserInTeamOwnedBy) — se extraen a un DAO compartido o se reimplementan; ver decisión 8.

### 5. Modelo GORM y tipos particulares

`dbs.WorkoutFeedback` con:
- `TeamID *int64` (columna nullable según spec).
- `SessionDate time.Time` con `type:date`.
- `RPE *int16`, `AvgHeartRate *int16`, `MaxHeartRate *int16`, `Cadence *int16` (SMALLINT).
- `MediaURLs` → `github.com/jackc/pgtype.TextArray` ya está en el árbol de dependencias vía `pgx/v4` (sin dep nueva); GORM lo serializa contra `TEXT[]`. El DTO de respuesta lo convierte a `[]string` para JSON limpio.
- `DeletedAt *time.Time` (baja lógica manual, patrón del repo — NO `gorm.DeletedAt`, que filtraría automáticamente y no deja control explícito).
- Índices vía tags `uniqueIndex`/`index` como en `attendance.go`.

**Riesgo residual:** `pgtype.TextArray` y GORM requieren prueba concreta. Mitigación: tarea de implementación verifica el round-trip contra Postgres real en el DAO test (se crea, se lee y se compara `[]string`); fallback sin dep nueva → columna `JSONB` (patrón `PresencialLocation`) con `datatypes.JSON`. Se deja documentado en tasks.

### 6. Validación de entrada en el service

El controller parsea IDs/parámetros (tipo/rango), el service valida semántica de negocio y el DAO solo persiste:

- `assigned_session_id`, `assigned_exercise_id` requeridos y `> 0`; `team_id` opcional `> 0`; `set_number` opcional `>= 0` (default 0 en DB y en request si falta).
- `session_date` requerida, parseada `YYYY-MM-DD`.
- `report_source` requerido, no vacío (string libre, extensible).
- Métricas numéricas opcionales `>= 0`; `rpe` `1..10` (doble control: en el service para 400 web y en la DB vía CHECK).
- `media_urls` opcional, `[]string`.
- `started_at`/`ended_at` opcionales RFC3339; si ambos vienen y `ended_at < started_at` → 400 (consistencia temporal básica).

### 7. Contratos de request/response (DTOs)

`domains/workoutfeedback/`:
- `CreateFeedbackRequest`: todos los campos persistibles menos `feedback_owner_user_id` (siempre del token). `athlete_user_id` opcional (default auth). `report_source` requerido.
- `UpdateFeedbackRequest`: mismos campos validables que create, todos opcionales; `athlete_user_id`/`feedback_owner_user_id` no editables.
- `WorkoutFeedbackResponse`: shape plano espejo de `dbs.WorkoutFeedback` con `media_urls []string`.
- `SearchResponse { Data []WorkoutFeedbackResponse }`.
- Constantes de mensajes (ej. `"feedback registrado"`).

### 8. Matriz de autorización — service decide, DAO consulta

Reutiliza el árbol del módulo de asistencias como patrón:

- **Create**: `feedback_owner_user_id = auth_user_id`. Si `athlete_user_id` viene y es distinto: solo si `ExistsUserInTeamOwnedBy(athlete, auth)` → si no, 403.
- **Get/:id**: leer el registro; si no existe o `deleted_at` → 404; si `auth` es `feedback_owner` o `athlete` → OK; si `team_id` presente y `IsTeamOwner(team, auth)` → OK; else 403.
- **Search**: 
  - sin params → `WHERE (athlete_user_id = auth OR feedback_owner_user_id = auth)` + filtros opcionales.
  - `team_id` → team debe existir (404) y `auth` ser owner (403); scope `WHERE team_id = ?`.
  - `athlete_user_id != auth` → requiere `ExistsUserInTeamOwnedBy(athlete, auth)` (403 si no); scope `WHERE athlete_user_id = ?`. Filtro adicional de `feedback_owner_user_id` si viene.
- **Update**: 404 si no existe/borrado; autoriza como Get (reportante o atleta) **o** owner del team; luego aplica campos del body.
- **Delete**: 404 si no existe/ya borrado; autoriza como Get (reportante o owner del team); setea `deleted_at`.

Los chequeos de equipo (`TeamExists`, `IsTeamOwner`, `ExistsUserInTeamOwnedBy`) ya existen en `attendance_dao`. Para no duplicarlos en dos DAOs, se **extrae** un `team_membership_dao` compartido (`daos/team_membership_dao.go`) y `attendance_dao`/`workout_feedback_dao` lo inyectan. Es refactor acotado sin cambio de comportamiento en asistencias.

**Alternativa descartada:** reimplementar los chequeos adentro de `workout_feedback_dao` (duplicación) o importar `attendance_dao` desde `workout_feedback_dao` (acoplar módulos por DAO ajeno).

### 9. Conflictos y soft-delete

- Duplicado activo en create → capturar `pgconn.PgError` code `23505` → `409 Conflict` (mensaje "ya existe un feedback para ese set"). Patrón de `attendance_dao.Create`.
- Update/Delete/get sobre borrado → `ErrFeedbackNotFound` → 404 (`deleted_at IS NOT NULL`).
- Todos los SELECT filtran `deleted_at IS NULL` explícitamente.

### 10. Errores

Errores de negocio exportados en `workout_feedback_service`: `ErrFeedbackNotFound`, `ErrFeedbackForbidden`, `ErrFeedbackDuplicate`, mapeados por el controller (vía `errors.Is`) a 404/403/409. 400 → validaciones de input. 401 → `AuthMiddleware` existente (mismo patrón que `attendance_controller`).

## Risks / Trade-offs

- [FK opacas] Sesiones/ejercicios asignados inexistentes pueden referenciarse. → Mitigación: contrato `> 0`; el change futuro de `assigned_*` agrega constraints + backfill. Queda anotado como quirk en CLAUDE.md.
- [Desviación del UNIQUE plano] El equipo pidió "estructura exacta de la spec"; el índice único parcial cambia semántica para soft-delete. → Mitigación: justificado y documentado (decisión 2); si la spec manda, es reemplazo de una línea en `postgres.go` (manteniendo la limitación).
- [Soft-delete no bloquea recreado] Permite que un mismo set "reaparezca" tras eliminar. → Considerado deliberado (es el objetivo del índice parcial). No hay historial ni auditoría de la eliminación en este change.
- [pgtype.TextArray] Compatibilidad GORM sin probar en el repo. → Mitigación: test DAO contra Postgres real en la misma tarea; fallback JSONB sin dependencias nuevas.
- [Matriz sin membresía en create self] Un atleta reporta un feedback sin team, y un entrenador puede reportar por un atleta de sus equipos. Caso "atleta de equipo ajeno" queda cubierto por `ExistsUserInTeamOwnedBy` en create; el get/search usan la matriz de equipo para evitar fuga.
- [Search sin paginación] Con muchos feedbacks puede degradarse. → Los índices del spec cubren el costo; paginado queda como mejora futura si aparece.
- [Refactor team_membership_dao] Tocar `attendance_dao` conlleva riesgo de romper su comportamiento. → Se extrae manteniendo las mismas firmas y cobertura: los tests existentes de attendance validan el refactor.

## Migration Plan

- Deploy: commit → CI verde → merge a `develop` → deploy de staging. La tabla se crea por `AutoMigrate` (agregar `dbs.WorkoutFeedback{}` al listado); el índice único parcial y el CHECK se crean como SQL crudo idempotente post-migración (mismo patrón de `postgres.go`).
- Rollback: reverter el commit; tabla nueva sin datos críticos previos. Si hubiera filas, se conservan (no se dropea automáticamente).
- Config: ninguna env var nueva (no depende de URLs externas).
- Documentación: actualizar `README.md` (tabla de endpoints) al mergear.
- Nota: registrar en `CLAUDE.md` los quirks: FK opacas de `assigned_*` y `route_summary` postergado por PostGIS.

## Open Questions

- ¿El frontend espera `rpe`, `completion_status` y `report_source` como strings libres o con vocabularios cerrados? Hoy se modelan libres (solo `rpe` con rango y `report_source` no vacío) para no inventar enums.
- ¿Se necesita auditar quién eliminó un feedback? Hoy la baja lógica solo setea `deleted_at`, sin `deleted_by`.
- ¿Confirmar la desviación del UNIQUE plano a índice único parcial (decisión 2)? Es la única decisión que no sigue la spec literal; el resto del modelo es idéntico.