# Design: colisiones-presenciales-y-calendario-agregado

## Context

El módulo de calendario vive en `calendar_service.go` (write paths `UpsertDay`/`Stamp`/`Bulk`/`Shift`, lecturas `GetRange`/`NextSession`/`CalendarSummary`). Hoy no existe la noción de "todos los grupos que administra un owner" ni comparación horaria entre días de grupos distintos. `GroupUser.DateEnd`/`DeletedAt` ya filtran membresías activas vía `FindByUserID`. Los horarios presenciales se persisten como `*time.Time` (UTC) y viajan "HH:MM" en las respuestas.

## Goals / Non-Goals

- Goals: detección de colisión presencial en las 4 escrituras con semántica cross/same team; endpoints de banner (corredor y entrenador); calendario agregado por rol con marcado de colisiones.
- Non-Goals: no se arreglan colisiones viejas retroactivamente (solo se detectan vía `administered-calendar`); no se agrega `force` a la colisión cross-team; no se toca el guard de día cerrado existente; no se implementa ninguna pieza de frontend.

## Decisions

### D1 — Comparador de overlap y alcance de la detección

Overlap de dos rangos presenciales el mismo día: `from < other.to && other.from < to` (intervalos medio-abiertos) — terminar 09:00 y arrancar 09:00 **no** colisiona (el frontend puede sumar su propio warning sobre el borde; fuera de alcance del backend). Solo participan días con `kind='training'` e `is_presencial=true`: los `cancelled` quedan fuera **en ambos lados** (ni como escritura a validar ni como colisionante) — un cancelado no es un compromiso físico.

### D2 — Helper de alcance del owner

`ownedGroupsForOwner(ownerID) []dbs.Group`: `teams.GetAllByOwnerID(ownerID)` → `groups.GetByTeamID(teamID)` por equipo (2 niveles, sin `deleted_at` en grupos activos). Devuelve grupos con `ID/Name/TeamID`; nombres de equipo se resuelven con `team_dao.FindByID` batch (map). No se cachea: volumen acotado por equipo.

### D3 — Detección de colisión (servicio)

`findPresencialCollisions(tx, ownerID, excludeGroupID *int64, excludeDayIDs []int64, dates []time.Time, candidateByDate map[date]rango)` → lista de conflictos `{group_id, group_name, team_id, team_name, date, presencial_time_from, presencial_time_to}`. Query: días presenciales de los grupos del owner en las fechas pedidas, excluyendo el grupo escrito (para `PUT`/`bulk`) y las filas movidas por su ID viejo (para `shift`). Cada conflicto se clasifica por `team_id` del grupo colisionante vs el del grupo escrito: distinto → bloqueante, igual → warning. Dentro de la transacción cuando la escritura es transaccional (evita TOCTOU básico; residual igual al TOCTOU ya parked del guard de día cerrado).

### D4 — Tipos de error y respuesta

- Error tipado `calendarPresencialCollisionError{conflicts []PresencialConflict}` con sentinel `ErrCalendarPresencialCollision`; `Error()` formatea fechas+grupos; `mapCalendarError` → `409` con body `{"message": "colisión presencial con otro equipo", "conflicts": [...]}` (patrón `calendarStampConflictError`).
- `PresencialConflict{GroupID, GroupName, TeamID, TeamName, Date "YYYY-MM-DD", PresencialTimeFrom "HH:MM", PresencialTimeTo "HH:MM"}`.
- `same_team_warnings []PresencialConflict` (omitir en JSON si vacío, `omitempty`) se agrega a las respuestas exitosas de `PUT` (CalendarDayResponse) y `stamp`/`bulk`/`shift` (wrapper nuevo `CalendarMutationResponse{days, same_team_warnings}`? — **decisión**: no: para no romper shapes existentes, `stamp`/`bulk`/`shift` devuelven un wrapper nuevo `{days: [...], same_team_warnings: [...]}` — breaking solo para consumidores que asuman array crudo; el frontend aún no maneja warnings, así que se coordina). ⚠️ Ver Open Questions.

### D5 — Wiring por endpoint

- **`PUT`**: después de guards de cerrado/validación, antes de escribir: si la fila resultante queda presencial, correr D3 sobre esa fecha. Cross → 409 rollback; same → guardar y devolver warnings.
- **`stamp`**: después del `409` de conflictos existente y del parse de `exclude_dates`, sobre los días objetivo que quedan presenciales (`calendarRequestFromPlanDay`), all-or-nothing igual que el resto del stamp. Cross → 409 con lista completa; same → warnings en respuesta.
- **`bulk`**: en el mismo loop de validación previa a escritura (junto con cerrado), all-or-nothing: cualquier cross → rechazar lote completo 409 listando todas las fechas conflictivas; same → warnings agregados.
- **`shift`**: tras validar cerrado, evaluar las fechas **nuevas** de las filas presencial movidas (excluyendo de la detección las filas movidas mismas, por ID). Cross → 409 rollback; same → warnings.
- Días cuyo resultado **no** queda presencial (cambiar presencial a async, borrar, cancelar) nunca disparan la detección.

### D6 — `next-session` (shape nuevo, in-place)

Reutiliza `groupUserDao.FindByUserID` + query nueva `FindNextForGroupsByKinds(groupIDs, fromDate, kinds)` (o 2 queries, una por kind). Filtro "hoy cuenta": `date > hoy` OR (`date == hoy` AND (no presencial OR `presencial_time_from > now`)) — mismo criterio que `isCalendarDayClosed` (hoy presencial ya arrancado = cerrado = no cuenta). Respuesta siempre `200`:
`{next_cancelled: {group_id, group_name, date, session_name} | null, next_training: {..., is_presencial?, presencial_time_from?, presencial_time_to?, presencial_location?} | null}` — `session_name` sale de `SessionInstance.Name` (si la instancia falta, `session_name: null`, no rompe). El controller deja de devolver `204`. Breaking confirmado.

### D7 — `next-presencial-session` (nuevo)

Grupos administrados del caller (D2) → query próxima `training`+`is_presencial` con el mismo filtro de "hoy" de D6 → `200` `{group_id, group_name, team_id, team_name, date, session_name, presencial_time_from, presencial_time_to, presencial_location}` o `204`. Guard `id == callerID` como los otros.

### D8 — Calendario agregado

- `member-calendar`: memberships activas → `FindByGroupAndRange` por grupo (N queries acotadas por cantidad de grupos; aceptable — batch real exigiría query custom; si la cantidad de grupos crece, optimizar después) → merge ordenado por fecha. Item = struct nueva `AggregateCalendarDayResponse` con los campos de `CalendarDayResponse` + `group_id/group_name/team_id/team_name` (embebidos planos, no nested).
- `administered-calendar`: grupos del owner (D2) → mismo merge → para cada día presencial del rango, correr detección D3 contra los demás grupos administrados (excluyendo la fila misma); si hay conflictos → `presencial_collision: {type: "cross_team"|"same_team", conflicts}` (`cross_team` gana si hay de ambos; `conflicts` lista todos). Colisiones viejas (pre-guard) aparecen naturalmente porque la detección es sobre datos actuales.
- Batch de nombres: una query de `groups` por IDs + una de `teams` por IDs → maps. `from`/`to` obligatorios (`400` si faltan), validación `from <= to`.

### D9 — Migración

Ninguna: no se agregan columnas ni tablas. Todo es lógica sobre datos existentes.

## Risks / Trade-offs

- La detección en `shift` por ID excluido depende de que las filas viejas sigan identificables en la tx — mitigado corriendo el chequeo antes del `UpdateDatesForShift`.
- `stamp` con colisión cross-team después del 409 de conflictos: dos pasos de validación en cadena, sin escritura parcial en ninguno (mismo all-or-nothing).
- `administered-calendar` hace detección por día presencial (coste O(días presenciales × grupos)); para tamaños actuales (docenas) es despreciable.
- Wrapper nuevo en `stamp`/`bulk`/`shift` es un cambio de shape (array crudo → objeto) — a coordinar con frontend; alternativa rechazada: header/custom field imposible en JSON array.

## Open Questions

- ~~Wrapper vs array~~ — resuelto: wrapper `{days, same_team_warnings}` para `stamp`/`bulk`/`shift`; `PUT` individual conserva `CalendarDayResponse` plano + `same_team_warnings` como campo extra.
