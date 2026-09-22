# Proposal: colisiones-presenciales-y-calendario-agregado

## Why

Tres gaps del frontend (`BACKEND_API_GAPS.md` del repo frontend, actualización 2026-09-22) sobre el módulo de calendario, todos detectados al encarar la vista agregada de calendario y los banners del home de corredor/entrenador:

1. **Gap 9** — Nada impide que un entrenador que administra varios equipos/grupos termine con dos días presenciales superpuestos en horario en grupos distintos. Si son de equipos distintos, son dos compromisos físicos simultáneos que no puede cumplir. Se pide validación en los 4 endpoints de escritura (`PUT`/`stamp`/`bulk`/`shift`): cross-equipo → `409` sin `force` posible; mismo equipo → guarda + `same_team_warnings` no bloqueante.
2. **Gap 10** — Los banners del home (corredor y entrenador) necesitan endpoints livianos de "próximo compromiso". `next-session` existe pero con otro shape (un solo día, `204` si no hay); se pide el shape `{next_cancelled, next_training}` con `200` siempre, y el endpoint nuevo `next-presencial-session` para el entrenador.
3. **Gap 11** — La vista mensual agregada (corredor y entrenador) implicaría 1 `GET /groups/{id}/calendar` por grupo (N+1) con merge client-side. Se piden `member-calendar` y `administered-calendar` agregados por rol, con `presencial_collision` en el lado del entrenador (incluye colisiones same-team permitidas por el guard y viejas guardadas antes del guard).

## What Changes

### Etapa 1 — Gap 9: validación de colisión presencial (capability `group-calendar`)

- Helper "todos los grupos administrados por un `owner_id`" (composición `teams.GetAllByOwnerID` + `groups.GetByTeamID`).
- Query de días presenciales (`kind='training'`, `is_presencial=true`) de esos grupos en las fechas afectadas, con `group`/`team` para nombres.
- Comparador de overlap: `(from < other.to) && (other.from < to)` — **bordes que se tocan NO colisionan**; días `cancelled` quedan **fuera** de la detección (ni como escritura ni como colisionante).
- Wiring en `PUT`/`stamp`/`bulk`/`shift`:
  - Cross-team → `409` tipado nuevo `ErrCalendarPresencialCollision` con lista de `conflicts` (patrón `calendarStampConflictError`), sin forma de forzar.
  - Same-team → la escritura sigue, y las respuestas exitosas de los 4 endpoints suman `same_team_warnings` (opcional, aditivo, misma forma que `conflicts`).
  - All-or-nothing en `bulk`/`shift` (misma semántica que el guard de día cerrado).
  - Un día se excluye a sí mismo como colisionante; en `shift` se evalúan las fechas nuevas excluyendo las filas movidas.
  - En `stamp` el chequeo corre **después** del `409` de conflictos existente, sobre los días que van a quedar presenciales.
- Respuestas `CalendarDayResponse`/`StampResponse`/`BulkResponse` + `same_team_warnings` opcional.

### Etapa 2 — Gap 10: banners del home (capability `user-calendar`)

- `GET /users/:id/next-session` cambia de shape **in-place** (breaking, confirmado): `200 {next_cancelled, next_training}` siempre (nunca `204`), independientes y nullable, cada uno `{group_id, group_name, date, session_name}` + presencial_* solo en `next_training`. `date >= hoy`, hoy presencial cuenta solo si `presencial_time_from` no pasó. Ya no embebe `session_instance`.
- `GET /users/:id/next-presencial-session` nuevo (entrenador): próxima `training`+`is_presencial` entre grupos que administra, `200` con `team_id`/`team_name` o `204`.

### Etapa 3 — Gap 11: calendario agregado (capability `user-calendar`)

- `GET /users/:id/member-calendar?from&to` — días de todos los grupos donde es miembro, item = `CalendarDayResponse` + `group_id/group_name/team_id/team_name`.
- `GET /users/:id/administered-calendar?from&to` — días de todos los grupos que administra + `presencial_collision` (`{type: same_team|cross_team, conflicts}`) en días presenciales, incluyendo same-team, cross-team viejas y entre cualquier par de grupos administrados. Reutiliza el comparador de la Etapa 1.
- `from`/`to` obligatorios; batch de nombres (grupos y equipos en 2 queries, sin N+1).

## Capabilities

### New

- `user-calendar` — vistas agregadas y banners del calendario por usuario (next-session nuevo shape, next-presencial-session, member-calendar, administered-calendar).

### Modified

- `group-calendar` — validación de colisión presencial en escrituras (`PUT`/`stamp`/`bulk`/`shift`) y `same_team_warnings` en respuestas.

## Impact

- **Código**: `calendar_service.go` (helpers de colisión + wiring en 4 write paths + NextSession reescrito), `calendar_controller.go` (mapeo 409 + 2 endpoints nuevos + shape), DAOs `group_calendar_day_dao`/`group_dao`/`team_dao` (queries nuevas), DTOs de respuesta (`same_team_warnings`, items agregados), `app/url_mappings.go` (2 rutas), Swagger regenerado, docs (`CATALOGO_Y_CALENDARIO.md`, `FRONTEND_IMPACTO_INSTANCIACION.md`).
- **Contrato**: `next-session` cambia de shape in-place (breaking para quien lo consuma con el shape viejo — confirmado que el frontend aún no lo consume); el resto es aditivo.
- **Frontend** (coordinar al final, como siempre): banners del home y vista agregada quedan desbloqueados; el guard de colisión requiere manejo del `409` nuevo y de `same_team_warnings` en las 4 pantallas de escritura de calendario.
- **Testing**: tests de colisión (PUT/stamp/bulk/shift, cross/same team, borde, all-or-nothing), de los 3 endpoints nuevos/reescritos y de la vista agregada, contra Postgres real; coverage gate 80 intacto.
