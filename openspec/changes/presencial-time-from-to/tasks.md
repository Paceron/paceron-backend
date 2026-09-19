# Tasks

## Tasks

### 1. Modelo y migración

- [x] `dbs.GroupCalendarDay.PresencialTime` → `PresencialTimeFrom`/`PresencialTimeTo`.
- [x] `dbs.PlanDay.DefaultTime` → `DefaultTimeFrom`/`DefaultTimeTo`.
- [x] Migración raw SQL en `postgres.go`: drop de `presencial_time`/`default_time`.
- [x] Documentar en el modelo la convención UTC-en-lectura (ver Task 4).

### 2. DTOs

- [x] `trainingplan.PlanDayRequest`/`PlanDayResponse`.
- [x] `calendar.CalendarDayRequest`/`CalendarDayResponse`.
- [x] `calendar.BulkRequest`.
- [x] `calendar.NextSessionResponse`.

### 3. Servicios y controllers

- [x] `training_plan_service.go`: `validateAndBuildDays` (parseo + `to > from`), `toPlanDayResponse`. Sentinel `ErrPlanInvalidTimeRange`.
- [x] `calendar_service.go`: `validateDayFields`, `buildRow`, `toCalendarDayResponse`, `Stamp`, `Bulk`, `NextSession`. Sentinels `ErrCalendarInvalidTimeFormat`/`ErrCalendarInvalidTimeRange`.
- [x] `session_service.go`: `isCalendarDayClosed` usa `PresencialTimeFrom`.
- [x] Controllers: nuevos sentinels mapeados a `422` en `training_plan_controller.go`/`calendar_controller.go`.

### 4. Fix de bug preexistente (D3 de design.md)

- [x] `.UTC()` antes de `.Hour()/.Minute()/.Format(...)` en los 4 puntos de lectura: `toCalendarDayResponse`, `toPlanDayResponse`, `isCalendarDayClosed`, `NextSession`.
- [x] Confirmado con test aislado (round-trip UTC 10:15 → Local 07:15 sin el fix; 10:15 → 10:15 con el fix).
- [x] Confirmado que el bug es preexistente en `develop` HEAD (reproducido sin ningún cambio de este change, contra Postgres real fresco).

### 5. Tests

- [x] Actualizar tests existentes que referenciaban `PresencialTime`/`DefaultTime` (mocks y reales).
- [x] Nuevo test: `time_to <= time_from` → `422` (calendar y training plan).
- [x] Confirmar estabilidad de los 3 tests de D13 que antes eran intermitentes (corridos x3 sin fallos).

### 6. Documentación

- [x] `docs/CATALOGO_Y_CALENDARIO.md`: todas las menciones a `presencial_time`/`default_time` actualizadas a `*_from`/`*_to`, más nota del bug de timezone en §9.
- [x] Swagger regenerado (`swag init`).

### 7. Verificación final

- [x] `go build ./...`, `go vet ./...`, `go test ./...` verde (con y sin `TEST_DB_HOST`).
- [x] `make coverage-with-db` con `-count=1` (evitar cache stale) → 81.1%, gate del 80% no se rompe.
