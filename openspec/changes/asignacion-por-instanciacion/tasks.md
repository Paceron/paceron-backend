# Tasks

> Diseño cerrado en Claude Code (2026-09-19), implementación planeada para OpenCode (`/opsx-apply asignacion-por-instanciacion`). Este checklist es un punto de partida, no una ejecución ya corrida — ajustar si al implementar aparece algo no contemplado en `design.md`.

## Tasks

### 1. Modelos y migración

- [ ] `cmd/api/domains/dbs/{exercise_instance,session_instance,session_exercise_instance}.go` — mismo shape que `Exercise`/`Session`/`SessionExercise`, sin `owner_id`/`deleted_at`/`updated_at` (ver design.md D1).
- [ ] `GroupCalendarDay`: reemplazar `SessionID *int64` por `SessionInstanceID *int64` (`gorm:"column:session_instance_id"`).
- [ ] AutoMigrate de las 3 tablas nuevas en `postgresdb/postgres.go`.
- [ ] SQL crudo post-AutoMigrate: `ALTER TABLE group_calendar_days DROP COLUMN IF EXISTS session_id` (mismo patrón que `presencial-time-from-to`).
- [ ] `go build ./...` verde.

### 2. DTOs y DAOs de instancia

- [ ] `cmd/api/domains/instance/` (o nombre de paquete equivalente) con responses de instancia si hacen falta expuestas (evaluar si el frontend necesita ver el detalle de una instancia o le alcanza con lo que ya devuelve `GroupCalendarDayResponse`).
- [ ] `daos/exercise_instance_dao.go`, `daos/session_instance_dao.go`, `daos/session_exercise_instance_dao.go`: `Create` simple (sin `Update`/`SoftDelete` — son inmutables), `Delete` físico, `FindByID`.
- [ ] Método para chequear si una `SessionInstance`/`ExerciseInstance` tiene `workout_feedback` asociado (join contra `workout_feedback.assigned_session_id`/`assigned_exercise_id`) — ver design.md D3.

### 3. `calendar_service.go` — instanciación en vez de referencia

- [ ] Extraer un helper `instantiateSession(ctx, sessionID) (*dbs.SessionInstance, error)` que resuelve `Session`+`SessionExercise`+`Exercise` del catálogo y crea la instancia completa (1 `SessionInstance` + N `SessionExerciseInstance` + N `ExerciseInstance`), reusable desde `UpsertDay`/`Bulk`/`Stamp`.
- [ ] `UpsertDay`: si `kind=training`, llamar al helper en vez de guardar `req.session_id` tal cual; el orden dentro de la transacción es: crear instancia nueva → repuntear `GroupCalendarDay.session_instance_id` → chequear feedback contra la instancia vieja → si no hay, borrar vieja (`SessionExerciseInstance` → `ExerciseInstance` → `SessionInstance`, D10); si la fecha está cerrada, `422` antes de tocar nada (D8).
- [ ] Mover `isCalendarDayClosed` de `session_service.go` a `calendar_service.go`, reusarla como guard (no como trigger de clonado) en `UpsertDay`/`DeleteDay`/`Bulk`/`BulkClear`/`Stamp`/`Shift` (D8 — los 5 endpoints de escritura, no solo 3).
- [ ] `Bulk`: mismo guard por cada fecha del lote, todo o nada (422 con lista si alguna está cerrada).
- [ ] `BulkClear`: mismo guard (D8) — rechazar el lote completo si alguna fecha está cerrada.
- [ ] `Shift`: mismo guard sobre las filas afectadas (`date >= from_date`) — rechazar el corrimiento completo si alguna fila afectada está cerrada.
- [ ] `Stamp`: mismo guard sobre las fechas resultantes del plan, instanciar por cada `PlanDay` con `kind=training`.
- [ ] `DeleteDay`: bloquear sobre día cerrado; si el día tenía instancia, borrarla (mismo chequeo de feedback, D10).
- [ ] `GetRange`/`NextSession`: devolver el detalle completo de la instancia embebido (D9, ya decidido — `session_instance: {id, name, description, exercises: [...]}`, no un ID bare). Actualizar `CalendarDayResponse`/`NextSessionResponse` en `domains/calendar/`.
- [ ] Eliminar `GET /sessions/{id}/assigned-groups`: `CalendarServiceInterface.AssignedGroups`, `GroupCalendarDaoInterface.FindDistinctGroupsBySession`, el handler en `session_controller.go`, la ruta en `url_mappings.go` (D7).

### 4. Remover el mecanismo viejo

- [ ] `session_service.go`: sacar `ExcludeGroupIDs`/`CloneName`/`CloneDescription` de `SessionRequest`, la rama de `Update` que los procesa, `cloneSessionInternal`'s `deepCloneExercises`/param de `exerciseDao` (volver a la firma sin eso), `isCalendarDayClosed` (se mudó, no se duplica).
- [ ] `exercise_service.go`: sacar el chequeo de días cerrados agregado por `congelar-ejercicio-en-clon` — `Update` vuelve a ser edición simple, constructor vuelve a solo `exerciseDao`.
- [ ] `GroupCalendarDaoInterface`: sacar `FindBySessionID`, `FindByExerciseID`, `RepointSessionForGroups`, `RepointDaysByID` (sin usuarios).
- [ ] Borrar los tests correspondientes en `session_service_test.go`, `exercise_service_test.go`, `group_calendar_day_dao_test.go`, `calendar_service_test.go` (mocks de los métodos removidos).
- [ ] `app/app.go`: ajustar wiring de `NewExerciseService`/`NewSessionService` a las firmas simplificadas.

### 5. Tests nuevos

- [ ] DAO: creación/borrado de instancias, chequeo de feedback-antes-de-borrar.
- [ ] Service: instanciar al asignar (mock + real-Postgres), reasignar día futuro borra instancia vieja (sin feedback) y la conserva (con feedback), guard de día cerrado en los 4 endpoints de escritura, `stamp`/`bulk` rechazan el lote completo si alguna fecha está cerrada.

### 6. Documentación

- [ ] Reescribir §8 de `docs/CATALOGO_Y_CALENDARIO.md` (clon por divergencia → instanciación).
- [ ] Nota cruzada en `calendario-asignacion-grupos/design.md` y `congelar-ejercicio-en-clon/design.md` apuntando a este change como reemplazo.
- [ ] Avisar a la sesión de frontend (cambios de contrato confirmados, no "puede cambiar"): `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` §5 (clonado por divergencia) queda obsoleto; `PUT /sessions/{id}` pierde `exclude_group_ids`/`clone_name`/`clone_description`; `GET /sessions/{id}/assigned-groups` se elimina sin reemplazo; `CalendarDayResponse`/`NextSessionResponse` cambian `session_id` por `session_instance: {id, name, description, exercises: [...]}` (D9).

### 7. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` verde.
- [ ] `make coverage-with-db` — gate de 80%.
