# Proposal: optimizaciones-calendario-y-indices

## Why

El hot path de la app (vistas de calendario) hace queries lineales en datos. `toCalendarDayResponse`
resuelve el detalle de la sesión instancia con 3 queries **por día** (instancia + links + ejercicios),
y ese builder corre en loop en GetRange, Stamp, Bulk, Shift, BulkClear, MemberCalendar y
AdministeredCalendar: un calendario de 30 días ≈ 92 queries por request. La búsqueda de equipos suma
2 queries por card de resultado. CalendarSummary (home del corredor) resuelve nombres de grupo en loop.

En paralelo, 4 tablas calientes no tienen índices más allá de la PK (`group_users`, `team_users`,
`session_exercise_instances`, `workout_feedback_point`): toda query por group_id/team_id/user_id/
feedback_id hoy es seq scan, y esas columnas alimentan rosters, guards, banners y el builder batch
propuesto.

## What Changes

- Builder batch de respuestas de calendario: 3 queries totales para N días (instancias IN + links IN +
  ejercicios IN) + map por `session_instance_id`. Los loops de GetRange/Stamp/Bulk/Shift/BulkClear/
  MemberCalendar/AdministeredCalendar pasan por el batch; las respuestas de un solo día (UpsertDay,
  detalle) conservan su camino actual.
- Búsqueda de equipos: owners en batch (`FindByIDs`) + `CountActiveByTeams` (COUNT GROUP BY) — la
  página completa cuesta 3-4 queries en vez de 2N+1.
- CalendarSummary: nombres de grupo en un solo batch (`FindByIDs`).
- Índices nuevos en `group_users` (group_id, user_id), `team_users` (team_id, user_id),
  `session_exercise_instances` (session_instance_id, exercise_instance_id) y `workout_feedback_point`
  (feedback_id), vía tags gorm + AutoMigrate.

**Constraint duro del usuario: no cambian endpoints ni contratos de respuesta.** Los shapes y el
comportamiento (incl. errores de instancia huérfana) quedan byte-idénticos; es optimización interna.

## Impact

- **Código:** `calendar_service.go` (refactor de builders), `session_instance_dao.go` (FindByIDs),
  `session_exercise_instance_dao.go` (FindBySessionInstances), `team_user_dao.go` (CountActiveByTeams),
  `team_service.go`, `group_summary` (CalendarSummary), 4 modelos de `dbs/` con tags de índice, mocks
  de tests.
- **Sin cambios de API:** ningún handler, DTO, shape ni código HTTP cambia.
- **Base de datos:** 7 índices nuevas creados por AutoMigrate; sin backfill ni columnas.
- **Specs:** sin deltas de comportamiento observable (RF igual); change declarado `skip_specs`.
