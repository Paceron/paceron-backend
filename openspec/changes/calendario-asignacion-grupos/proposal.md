## Why

Segundo y último change para cerrar Gap 4 (`paceron-frontend/docs/BACKEND_API_GAPS.md`). Con el catálogo (`catalogo-planes-entrenamiento`) ya implementado en esta misma rama, falta la mitad que le da sentido de uso real: cómo un plan-template se convierte en el calendario de un grupo. Sin esto, `TrainingPlan` es un catálogo sin forma de aplicarse a nadie.

La spec completa (`paceron-frontend/docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md`) reemplaza un diseño anterior (`RunnerPlanAssignment`, asignación directa a un corredor, `CurrentPlanMark`, `Group.training_plan_id`) que **nunca se implementó** — no hay migración de datos, se descarta sin más.

## What Changes

- **Decisión central: no hay entidad "Assignment"** — la asignación de un plan a un grupo es, directamente, su calendario (`GroupCalendarDay`, una fila por fecha con contenido). Un corredor no "recibe" una asignación al sumarse a un grupo: ve el calendario que ya existe. "Estampar" un plan copia sus `PlanDay` día por día a filas de `GroupCalendarDay` reales — es un atajo de carga masiva, no una relación persistente con el plan.
- **`GroupCalendarDay`**: tabla dispersa (sin fila = día vacío), `UNIQUE(group_id, date)`, `kind` `rest`/`other`/`training`/`cancelled` (el 4to estado, `cancelled`, no existe en el catálogo — es propio del calendario real). `source_plan_id` es puramente informativo (`ON DELETE SET NULL`), nunca bloquea ediciones.
- **Endpoints de escritura restringidos al entrenador dueño del grupo** — lectura abierta a cualquier miembro. Permisos explícitos por endpoint (ver design.md D2), no la convención general del resto del backend.
- **`stamp`**: copia un `TrainingPlan` completo al calendario a partir de una fecha, con detección de conflicto (`409` + lista de fechas, salvo `force=true`).
- **`bulk`/`bulk-clear`/`shift`**: operaciones de calendario en lote, propias de este dominio (no existe un equivalente de batch en el catálogo, ver `catalogo-planes-entrenamiento` Non-Goals).
- **Vistas del corredor**: `next-session` (próximo entrenamiento entre todos sus grupos) y `calendar-summary` (lista de grupos para "Mis asignaciones") — ambas de "mis propios datos", `403` si el `{id}` de la URL no coincide con el usuario autenticado.
- **Clonado por divergencia al editar una `Session` con asignaciones activas**: si el entrenador destilda algunos grupos al editar una sesión desde el catálogo, esos grupos pasan a apuntar a un clon nuevo (transaccional, un solo clon compartido entre todos los destildados) en vez de recibir el cambio — extiende el `PUT /sessions/{id}` del change anterior con 3 campos opcionales nuevos en el body.

## Capabilities

### New Capabilities

- `group-calendar`: CRUD de días de calendario por grupo (`GET` rango, `PUT`/`DELETE` día individual, `stamp`, `bulk`, `bulk-clear`, `shift`), con permisos entrenador-dueño-only para escritura.
- `runner-calendar-views`: `next-session` y `calendar-summary`, endpoints de "mis datos" para el home/sección de asignaciones del corredor.
- `session-divergence-clone`: extensión de `PUT /sessions/{id}` (del change de catálogo) para clonar por divergencia cuando hay grupos que no deben recibir la edición.

## Non-Goals

Todos explícitos en la spec de frontend (`BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` §7), documentados acá para que no se den por sentado en la implementación:
- Tracking de completado/cumplido de una sesión o día.
- Monitoreo en tiempo real de ubicación durante una sesión presencial.
- Reuso de `LocationPicker`/columnas de ubicación en `Team`/`User`.
- Multi-sesión por día (`GroupCalendarDay` es 1:1 con la fecha).
- Notificaciones al estampar/editar/cancelar.
- Concurrencia — dos entrenadores editando el mismo calendario al mismo tiempo: last-write-wins, sin lock, aceptable al tamaño de equipo actual.

## Impact

- **Schema/DB**: tabla nueva `group_calendar_days`. Aditivo. AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- **Dominios/DTOs**: `cmd/api/domains/dbs/group_calendar_day.go`; `cmd/api/domains/constants/group_calendar_day_kind.go`; `cmd/api/domains/calendar/` (request/response, reusa `trainingplan.Location` del change anterior).
- **DAOs**: `cmd/api/daos/group_calendar_day_dao.go`.
- **Servicios**: `cmd/api/services/calendar_service.go` (toca `groupCalendarDayDao`, `trainingPlanDao`, `planDayDao`, `teamDao`/`groupDao`/`teamUserDao`/`groupUserDao` directo — sin delegate, mismo criterio que el change anterior); extiende `services/session_service.go` (clonado por divergencia, ya existe del change 1).
- **Controllers/rutas**: `cmd/api/controllers/calendar_controller.go`, 8 rutas nuevas en `cmd/api/app/url_mappings.go`. Extiende `controllers/session_controller.go` (nuevos campos opcionales en el body de `PUT`, mismo handler).
- **Swagger**: regenerar `cmd/api/docs`.
- **Tests**: DAO con Postgres real, services/controllers con mocks; caso completo de clonado por divergencia (transacción con rollback si el `PUT` final falla).
