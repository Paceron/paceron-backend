# Tasks

## Tasks

### 1. Modelo, constants y migración

- [ ] `cmd/api/domains/dbs/group_calendar_day.go` (D1).
- [ ] `cmd/api/domains/constants/group_calendar_day_kind.go` (D2).
- [ ] Índice único `(group_id, date)` — vía GORM tag `uniqueIndex` compuesto.
- [ ] AutoMigrate en `postgres.go`.
- [ ] `go build ./...` verde.

### 2. DTOs de dominio

- [ ] `cmd/api/domains/calendar/{calendar_day_request.go,calendar_day_response.go,next_session_response.go,calendar_summary_response.go}` (reusa `trainingplan.Location`).

### 3. DAO

- [ ] `daos/group_calendar_day_dao.go` (+ `_test.go` Postgres real): `Upsert(day)` (por `group_id`+`date`), `FindByGroupAndRange(groupID, from, to)`, `FindByGroupAndDate`, `Delete(groupID, date)`, `DeleteByDates(groupID, dates)`, `FindNextSessionForGroups(groupIDs, fromDate)`, `FindDistinctGroupsBySession(sessionID)`, `ClearSourcePlan(planID)`, `ShiftDates(groupID, fromDate, days)` (o resolver shift leyendo+escribiendo desde el service, ver D6).

### 4. Servicio de calendario

- [ ] `services/calendar_service.go`: `GetRange`, `UpsertDay` (D3), `DeleteDay`, `Stamp` (D5, transaccional), `Bulk`, `BulkClear`, `Shift` (D6, con detección de colisión), `NextSession` (D7), `CalendarSummary` (D7), `AssignedGroups` (D9).
- [ ] Chequeos de permisos (D4) como funciones del service (`isGroupOwner`, `isGroupMember`) reusadas por el controller antes de cada operación de escritura.
- [ ] Tests unitarios: cada regla de D3, `stamp` con y sin conflicto (+`force`), `shift` con y sin colisión, `next-session` cruzando 2+ grupos.

### 5. Clonado por divergencia (extiende change 1)

- [ ] Refactor `session_service.go`: extraer `cloneInternal` reusada por `Clone` (ya existente) y por el nuevo flujo de `Update`.
- [ ] `session_service.go`: `Update` acepta `exclude_group_ids?/clone_name?/clone_description?`, implementa D8 en una transacción.
- [ ] Extiende `training_plan_service.go`: `Delete` limpia `source_plan_id` en `group_calendar_days` antes de borrar el plan (D10) — o resuelto desde `calendar_service` si el archivo de change 1 ya cerró su interfaz sin ese dao (decisión de implementación libre, incluida en design.md D10).
- [ ] Tests: `PUT` con `exclude_group_ids` (transacción completa, rollback si falla el paso final), `PUT` sin el campo (comportamiento sin cambios respecto al change 1).

### 6. Controllers y rutas

- [ ] `controllers/calendar_controller.go`: 8 handlers de D11 + `assigned-groups`, guards de permisos D4 antes de llamar al service.
- [ ] Extiende `controllers/session_controller.go`: nuevo handler `AssignedGroups`, el `PUT` existente parsea los 3 campos nuevos.
- [ ] 9 rutas nuevas en `url_mappings.go` (8 de calendario + `assigned-groups`), detrás de `AuthMiddleware()`.
- [ ] Wiring en `app.go`.
- [ ] Tests de controller: cada regla de permisos D4 (403 en cada combinación) + camino feliz.

### 7. Swagger y documentación

- [ ] Regenerar `cmd/api/docs`.
- [ ] Actualizar tabla de endpoints en `README.md`.

### 8. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` verde (repo completo, incluye change 1).
- [ ] `make coverage-with-db` — confirmar gate de 80%.
- [ ] Prueba manual end-to-end contra testing: crear plan con 2 días, estampar en un grupo, editar la sesión de uno de los días con `exclude_group_ids`, confirmar que el grupo excluido quedó en el clon y el calendario del otro grupo (si lo hubiera) no cambió.
