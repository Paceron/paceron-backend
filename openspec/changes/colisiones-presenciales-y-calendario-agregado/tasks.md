# Tasks: colisiones-presenciales-y-calendario-agregado

Ejecución: subagent-driven-development, 3 etapas (Gap 9 → 10 → 11) en una sola PR/rama. Reglas transversales: Postgres real para tests DAO/service (`docker start paceron-test-db`, :5433, env TEST_DB_*), coverage gate 80 (no bajar umbral; `go clean -cache` si el número sale raro — bug conocido de cache con doble instrumentación), Swagger regenerado con `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal`, stageear solo rutas explícitas (nunca `git add -A`), no push/merge (lo hace el usuario).

## Etapa 1 — Gap 9: colisión presencial en escrituras

### Task 1: Base de detección (helper owner + query + comparador + tipos)

- [x] 1.1 `group_dao` (o servicio): helper `FindByOwnerID(ownerID)` — grupos activos de todos los equipos cuyo `teams.owner_id = ownerID` (composición `team_dao.GetAllByOwnerID` + query por team_ids, o query única con join; preferir única si es simple).
- [x] 1.2 `group_calendar_day_dao`: `FindPresencialForGroupsInRange(groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error)` — `kind='training' AND is_presencial=true AND date IN dates AND group_id IN groupIDs`, con `Order("date, presencial_time_from")`.
- [x] 1.3 Tipos en `domains/calendar`: `PresencialConflict{GroupID, GroupName, TeamID, TeamName int64/string, Date string "YYYY-MM-DD", PresencialTimeFrom, PresencialTimeTo string "HH:MM"}`; error `ErrCalendarPresencialCollision` + `calendarPresencialCollisionError{conflicts []PresencialConflict}` (patrón stampConflict) con `Error()` que lista fecha+grupo.
- [x] 1.4 En `calendar_service.go`: `findPresencialCollisions(ctx, db, ownerID, excludeGroupID *int64, excludeDayIDs []int64, dates, candidates)` que devuelve (cross []PresencialConflict, same []PresencialConflict) usando D1/D3 del design (overlap medio-abierto, solo training+presencial, exclusión de self).
- [x] 1.5 `mapCalendarError`: case `ErrCalendarPresencialCollision` → `409` con body `{"message": "colisión presencial con otro equipo", "conflicts": [...]}` (estructura de respuesta JSON dedicada en el controller, no string).
- [x] 1.6 `go build` + `go vet` + tests DAO del helper (grupos de varios equipos, grupos soft-deleted fuera).

### Task 2: Wiring en PUT, stamp, bulk, shift + same_team_warnings

- [x] 2.1 `UpsertDay`: después de guards, si la fila resultante queda `is_presencial=true` → detección (excluyendo self); cross → 409 rollback tx; same → guardar. `CalendarDayResponse` + `same_team_warnings []PresencialConflict json:"same_team_warnings,omitempty"` (respuesta del endpoint individual).
- [x] 2.2 `Stamp`: después del 409 de conflictos y del parse de exclude_dates, evaluar los días objetivo que quedan presenciales (con `calendarRequestFromPlanDay`); cross → 409 all-or-nothing; same → warnings. Respuesta pasa de array crudo a wrapper `CalendarMutationResponse{Days []CalendarDayResponse json:"days", SameTeamWarnings []PresencialConflict json:"same_team_warnings,omitempty"}`.
- [x] 2.3 `Bulk`: en el loop de validación previa (junto con cerrado), all-or-nothing: cualquier cross → rechazar lote completo 409 listando todas las fechas; same → warnings agregados. Respuesta → wrapper igual que stamp.
- [x] 2.4 `Shift`: tras guard cerrado, evaluar fechas nuevas de filas presenciales movidas (excluir filas movidas por ID); cross → 409 rollback; same → warnings. Respuesta → wrapper.
- [x] 2.5 Swagger: anotaciones de 409 nuevo y `same_team_warnings` en los 4 endpoints + regenerar artefactos.
- [x] 2.6 `go build` + `go vet` + suite existente verde (los tests viejos de stamp/bulk/shift que asuman array crudo se actualizan al wrapper).

### Task 3: Tests de colisión (etapa 1)

- [x] 3.1 Test helper: fixture 1 owner + 2 equipos (A: grupos G1/G2; B: grupo G3).
- [x] 3.2 PUT cross-team → 409 sin escribir (fila no existe); PUT same-team → 200 + same_team_warnings poblado; PUT sin superposición → 200 sin warnings.
- [x] 3.3 Bordes que se tocan (09:00/09:00) → NO colisión (200).
- [x] 3.4 Cancelado como colisionante NO bloquea: día cancelled presencial superpuesto → 200.
- [x] 3.5 Stamp: cross → 409 con fechas; mismo-team → wrapper con days + warnings; exclude_dates + colisión en fecha no excluida → 409; colisión solo en fecha excluida → 201.
- [x] 3.6 Bulk cross en 2ª fecha → 409 all-or-nothing (1ª fecha no escrita); bulk same → warnings.
- [x] 3.7 Shift: mover presencial a fecha ocupada cross → 409 rollback (fila mantiene fecha vieja); same → warnings.
- [x] 3.8 Regresión: escrituras no presenciales siguen sin pasar por detección (día async en horario ocupado → 200).
- [x] 3.9 Suite completa + coverage ≥ 80.

## Etapa 2 — Gap 10: banners del home

### Task 4: next-session shape nuevo

- [x] 4.1 DAO: query próxima por kind con filtro "hoy cuenta" (date > hoy OR (date == hoy AND (NOT presencial OR time_from > now))) — reutilizar `isCalendarDayClosed` como criterio de filtro donde aplique.
- [x] 4.2 Service: reescribir `NextSession` → `{next_cancelled, next_training}` (DTOs nuevos `NextSessionBannerItem{GroupID, GroupName, Date, SessionName}` + `NextTrainingBannerItem` con campos presenciales); `group_name` resuelto (batch por IDs); `session_name` de la instancia (null si falta); siempre devuelve respuesta (nunca nil/204).
- [x] 4.3 Controller: dejar de responder 204; swagger actualizado; romper shape documentado como breaking en la anotación.
- [x] 4.4 Tests: ambos próximos; solo uno; ninguno (200 con ambos null); hoy presencial ya arrancado no cuenta; hoy presencial por arrancar cuenta; hoy async cuenta; membresía inactiva (date_end/deleted) fuera.

### Task 5: next-presencial-session del entrenador

- [x] 5.1 Service: grupos administrados del caller (helper Task 1) + query próxima presencial → respuesta con team_id/team_name (batch de teams).
- [x] 5.2 Ruta `GET /api/v1/users/:id/next-presencial-session` + guard `id == callerID` (403) + controller (200 o 204) + swagger.
- [x] 5.3 Tests: próxima entre varios equipos; ninguna → 204; solo cuenta training presencial (no async, no cancelled); hoy por arrancar cuenta / ya arrancada no; 403 por id ajeno.

## Etapa 3 — Gap 11: calendario agregado

### Task 6: member-calendar

- [x] 6.1 DTO `AggregateCalendarDayResponse` (campos de CalendarDayResponse + group_id/group_name/team_id/team_name).
- [x] 6.2 Service: memberships activas → días por rango → merge ordenado por fecha → nombres batch (1 query groups, 1 query teams).
- [x] 6.3 Ruta `GET /api/v1/users/:id/member-calendar` + guard id==caller + from/to obligatorios (400) y from<=to (400) + swagger.
- [x] 6.4 Tests: 2 grupos mismo rango; rango sin días → 200 []; 403 id ajeno; 400 sin from/to.

### Task 7: administered-calendar con presencial_collision

- [x] 7.1 Service: grupos del owner → días → para cada día presencial, detección (reutiliza Task 1.4, excluyendo la fila misma) → `presencial_collision {type, conflicts}` (cross gana sobre same; conflicts lista todos); null/omitido si no colisiona.
- [x] 7.2 Ruta + guard + validación from/to + swagger.
- [x] 7.3 Tests: colisión same marcada en ambos días; colisión cross marcada; colisión vieja (insertada por DAO directo) detectada; día aislado sin collision; día cancelled presencial NO genera collision.

### Task 8: Docs + verificación final

- [ ] 8.1 `docs/CATALOGO_Y_CALENDARIO.md`: sección de colisión (reglas, 409 shape, warnings, wrappers), endpoints de banner y agregados.
- [ ] 8.2 `docs/FRONTEND_IMPACTO_INSTANCIACION.md`: sección nueva — next-session breaking (shape nuevo), wrapper en stamp/bulk/shift, 409 de colisión y same_team_warnings, endpoints nuevos, cómo detectar colisiones viejas.
- [ ] 8.3 `openspec validate colisiones-presenciales-y-calendario-agregado --strict` + gofmt (archivos tocados) + `go build` + `go vet` + `go test ./...` (Postgres real) + `make coverage-with-db` (gate 80, `go clean -cache` si sale raro).
- [ ] 8.4 Tildar tasks.md completo; commits con rutas explícitas por etapa.
