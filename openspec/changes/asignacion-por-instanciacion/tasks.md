# Tasks

> Diseño cerrado en Claude Code (2026-09-19), implementación planeada para OpenCode (`/opsx-apply asignacion-por-instanciacion`). Este checklist es un punto de partida, no una ejecución ya corrida — ajustar si al implementar aparece algo no contemplado en `design.md`.

## Tasks

### Task 1: Modelos y migración

- [x] `cmd/api/domains/dbs/{exercise_instance,session_instance,session_exercise_instance}.go` — mismo shape que `Exercise`/`Session`/`SessionExercise`, sin `owner_id`/`deleted_at`/`updated_at` (ver design.md D1).
- [x] `GroupCalendarDay`: reemplazar `SessionID *int64` por `SessionInstanceID *int64` (`gorm:"column:session_instance_id"`).
- [x] AutoMigrate de las 3 tablas nuevas en `postgresdb/postgres.go`.
- [x] SQL crudo post-AutoMigrate: `ALTER TABLE group_calendar_days DROP COLUMN IF EXISTS session_id` (mismo patrón que `presencial-time-from-to`).
- [x] `go build ./...` verde.

### Task 2: DTOs y DAOs de instancia

- [x] `cmd/api/domains/instance/` (o nombre de paquete equivalente) con los responses de instancia necesarios para que `CalendarDayResponse`/`NextSessionResponse` embeban el detalle congelado definido en design.md D9.
- [x] `daos/exercise_instance_dao.go`, `daos/session_instance_dao.go`, `daos/session_exercise_instance_dao.go`: `Create` simple (sin `Update`/`SoftDelete` — son inmutables), `Delete` físico, `FindByID`.
- [x] Método para chequear si una `SessionInstance`/`ExerciseInstance` tiene `workout_feedback` asociado (join contra `workout_feedback.assigned_session_id`/`assigned_exercise_id`) — ver design.md D3.

### Task 3: `calendar_service.go` — instanciación en vez de referencia

- [x] Extraer un helper `instantiateSession(ctx, sessionID) (*dbs.SessionInstance, error)` que resuelve `Session`+`SessionExercise`+`Exercise` del catálogo y crea la instancia completa (1 `SessionInstance` + N `SessionExerciseInstance` + N `ExerciseInstance`), reusable desde `UpsertDay`/`Bulk`/`Stamp`.
- [x] `UpsertDay`: si `kind=training`, llamar al helper en vez de guardar `req.session_id` tal cual; el orden dentro de la transacción es: crear instancia nueva → repuntear `GroupCalendarDay.session_instance_id` → chequear feedback contra la instancia vieja → si no hay, borrar vieja (`SessionExerciseInstance` → `ExerciseInstance` → `SessionInstance`, D10); si la fecha está cerrada, `422` antes de tocar nada (D8).
- [x] Mover `isCalendarDayClosed` de `session_service.go` a `calendar_service.go`, reusarla como guard (no como trigger de clonado) en `UpsertDay`/`DeleteDay`/`Bulk`/`BulkClear`/`Stamp`/`Shift` (D8 — todas las operaciones de escritura cubiertas por la spec).
- [x] `Bulk`: mismo guard por cada fecha del lote, todo o nada (422 con lista si alguna está cerrada).
- [x] `BulkClear`: mismo guard (D8) — rechazar el lote completo si alguna fecha está cerrada.
- [x] `Shift`: mismo guard sobre las filas afectadas (`date >= from_date`) — rechazar el corrimiento completo si alguna fila afectada está cerrada.
- [x] `Stamp`: mismo guard sobre las fechas resultantes del plan, instanciar por cada `PlanDay` con `kind=training`.
- [x] `DeleteDay`: bloquear sobre día cerrado; si el día tenía instancia, borrarla (mismo chequeo de feedback, D10).
- [x] `GetRange`/`NextSession`: devolver el detalle completo de la instancia embebido (D9, ya decidido — `session_instance: {id, name, description, exercises: [...]}`, no un ID bare). Actualizar `CalendarDayResponse`/`NextSessionResponse` en `domains/calendar/`.
- [x] Eliminar `GET /sessions/{id}/assigned-groups`: `CalendarServiceInterface.AssignedGroups`, `GroupCalendarDaoInterface.FindDistinctGroupsBySession`, el handler en `session_controller.go`, la ruta en `url_mappings.go` (D7).

### Task 4: Remover el mecanismo viejo

- [x] `session_service.go`: sacar `ExcludeGroupIDs`/`CloneName`/`CloneDescription` de `SessionRequest`, la rama de `Update` que los procesa, `cloneSessionInternal`'s `deepCloneExercises`/param de `exerciseDao` (volver a la firma sin eso), `isCalendarDayClosed` (se mudó, no se duplica).
- [x] `exercise_service.go`: sacar el chequeo de días cerrados agregado por `congelar-ejercicio-en-clon` — `Update` vuelve a ser edición simple, constructor vuelve a solo `exerciseDao`.
- [x] `GroupCalendarDaoInterface`: sacar `FindBySessionID`, `FindByExerciseID`, `RepointSessionForGroups`, `RepointDaysByID` (sin usuarios).
- [x] Borrar los tests correspondientes en `session_service_test.go`, `exercise_service_test.go`, `group_calendar_day_dao_test.go`, `calendar_service_test.go` (mocks de los métodos removidos).
- [x] `app/app.go`: ajustar wiring de `NewExerciseService`/`NewSessionService` a las firmas simplificadas.

### Task 5: Tests nuevos

- [x] DAO: creación/borrado de instancias, chequeo de feedback-antes-de-borrar.
- [x] Service: instanciar al asignar (mock + real-Postgres), reasignar día futuro borra instancia vieja (sin feedback) y la conserva (con feedback), guards de día cerrado en `UpsertDay`/`DeleteDay`/`Bulk`/`BulkClear`/`Stamp`/`Shift`, `stamp`/`bulk`/`bulk-clear`/`shift` rechazan el lote completo si alguna fecha o fila afectada está cerrada.

### Task 6: Documentación

- [x] Reescribir §8 de `docs/CATALOGO_Y_CALENDARIO.md` (clon por divergencia → instanciación); revisar también el resto del doc (`session_id` vivo, freeze-on-edit, `exclude_group_ids`) que describe el mecanismo viejo como vigente.
- [x] Actualizar `README.md`: quitar `GET /sessions/:id/assigned-groups` del listado de endpoints y cualquier referencia al clonado por divergencia.
- [x] Nota cruzada en `calendario-asignacion-grupos/design.md` y `congelar-ejercicio-en-clon/design.md` apuntando a este change como reemplazo.
- [x] Avisar a la sesión de frontend (cambios de contrato confirmados, no "puede cambiar"): `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` §5 (clonado por divergencia) queda obsoleto; `PUT /sessions/{id}` pierde `exclude_group_ids`/`clone_name`/`clone_description`; `GET /sessions/{id}/assigned-groups` se elimina sin reemplazo; `CalendarDayResponse`/`NextSessionResponse` cambian `session_id` por `session_instance: {id, name, description, exercises: [...]}` (D9).

### Task 7: Verificación final

- [x] `go build ./...`, `go vet ./...`, `go test ./...` verde.
- [x] `make coverage-with-db` — gate de 80%.
