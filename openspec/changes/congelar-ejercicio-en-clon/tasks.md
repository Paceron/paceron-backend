# Tasks

## Tasks

### 1. DAO: `FindByExerciseID`

- [x] `GroupCalendarDaoInterface.FindByExerciseID(ctx, exerciseID) ([]dbs.GroupCalendarDay, error)` en `cmd/api/daos/group_calendar_day_dao.go` — join `group_calendar_days` × `session_exercises` por `session_id`, filtrado por `exercise_id`.
- [x] Test real-Postgres en `cmd/api/daos/group_calendar_day_dao_test.go`.
- [x] Agregar el método al mock `mockGroupCalendarDao` (`cmd/api/services/calendar_service_test.go`).

### 2. `cloneSessionInternal`: clon profundo opcional

- [x] Agregar parámetros `exerciseDao daos.ExerciseDaoInterface` y `deepCloneExercises bool` a `cloneSessionInternal` (`cmd/api/services/session_service.go`).
- [x] Si `deepCloneExercises`, clonar cada `Exercise` distinto referenciado (cachear por `exercise_id` original para no duplicar si la sesión repite el mismo ejercicio en varios roles) y usar el ID clonado en `SessionExercise.ExerciseID`.
- [x] Actualizar los 2 call sites existentes: `Clone()` → `false`; rama de divergencia de `Update()` → `true` (agregar `txExerciseDao := daos.NewExerciseDao(tx)`).

### 3. `ExerciseService.Update`: mismo chequeo de días cerrados que D13

- [x] Extender struct/constructor de `exerciseService` con `sessionDao`, `sessionExerciseDao`, `groupCalendarDayDao`, `db *gorm.DB`.
- [x] En `Update`: buscar `groupCalendarDayDao.FindByExerciseID(id)`, filtrar cerrados con `isCalendarDayClosed` (reusar tal cual, mismo paquete), agrupar IDs de día por `session_id`.
- [x] Si no hay ningún día cerrado: camino simple existente, sin cambios.
- [x] Si hay: transacción — por cada sesión afectada, `cloneSessionInternal(..., deepCloneExercises=true)` + `RepointDaysByID` con los IDs de esa sesión; aplicar la edición del ejercicio al final, con `txExerciseDao`.
- [x] Actualizar wiring en `cmd/api/app/app.go` (`NewExerciseService` gana 4 args nuevos, todas las DAOs ya existen en ese scope).

### 4. Tests de servicio

- [x] Actualizar los 11 call sites de `NewExerciseService(...)` en `cmd/api/services/exercise_service_test.go` con los mocks nuevos (default: sin días referenciados).
- [x] Nuevo test mock: ejercicio sin días referenciados → camino simple, sin transacción.
- [x] Nuevo test mock: solo días futuros → no dispara clon.
- [x] Nuevo test real-Postgres: día pasado → clona sesión + ejercicio, repuntea; verificar que el `Exercise` original queda con el valor editado y el clon con el valor viejo.
- [x] Nuevo test real-Postgres: mismo ejercicio en 2 sesiones (una con día cerrado, otra sin) → solo la primera se clona.
- [x] Nuevo test real-Postgres: misma sesión con un día cerrado en un grupo y uno abierto en otro → solo el cerrado se repuntea.

### 5. Documentación

- [x] Actualizar `docs/CATALOGO_Y_CALENDARIO.md` §8 (clon por divergencia) para reflejar el congelamiento profundo y el nuevo disparador desde `Exercise.Update`.
- [x] Actualizar `openspec/changes/calendario-asignacion-grupos/design.md` con una nota de que D13 fue extendido por este change (referencia cruzada, sin reescribir el original).

### 6. Verificación final

- [x] `go build ./...`, `go vet ./...`, `go test ./...` verde.
- [x] `make coverage-with-db` — confirmar que no rompe el gate de 80%.
- [x] Confirmar (por inspección de código, no por query a DB real) que `workout_feedback` no referencia ningún archivo tocado por este change.
