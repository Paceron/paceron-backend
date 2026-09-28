# Gestión de asistencia desde el panel del entrenador

## Why

Hoy la asistencia solo se puede registrar desde el propio endpoint de alta
(`POST /attendance/team/:team_id/session/:training_session_id`), y **ningún cliente
lo consume**: el frontend no tiene una sola línea de código de asistencia
(no hay `services/attendance.js`, ni hook, ni pantalla). En la práctica el
entrenador no tiene forma de ver quién asistió a una sesión presencial, ni de
cargar o corregir la asistencia, y tampoco puede emitir el QR de una sesión desde
la app — el endpoint de QR existe pero es alcanzable por cualquier usuario
autenticado (`README.md:349`).

El caso de uso es el del entrenador que termina de una sesión presencial y
necesita, en el momento, cerrar la asistencia del grupo: ver la grilla de
corredores con su estado, marcar los que asistieron en una sola operación,
corregir un error borrando una fila, y tener el QR de esa sesión a mano para
compartir.

## Objetivo

Habilitar la gestión completa de asistencia de una sesión presencial desde el
panel del entrenador, con los cuatro datos que hoy no existen: listado de
sesiones presenciales ya ocurridas de un grupo, grilla de corredores con su
estado de asistencia, carga/edición masiva y borrado individual, y un QR
restringido al entrenador del equipo.

## Alcance

- **Listado de sesiones presenciales ocurridas de un grupo** — endpoint nuevo que
  devuelve los `session_instance` de días con `kind=training` **AND**
  `is_presencial=true` **AND** `date <= hoy`, cada uno con sus conteos de
  asistencia. Es lo que alimenta el selector de sesión del front; hoy no existe
  ninguna forma de obtenerlo (`GET /groups/:id/calendar` es range-scoped y
  mensual, no sirve para "todas las presenciales pasadas").
- **Grilla de asistencia por sesión** — endpoint nuevo que resuelve en **un solo
  round-trip** el roster del grupo cruzado con el estado de asistencia de cada
  corredor, más los agregados (asistentes / no confirmados / % ). Evita que el
  front haga N+1 con roster + búsqueda.
- **Carga masiva de asistencia** — endpoint nuevo de upsert por lotes, para que
  el entrenador marque varias filas y las guarde con un solo clic.
- **Borrado individual de asistencia** — endpoint nuevo, con verificación de rol.
- **Provenance de la asistencia** — columnas aditivas `source` (`qr` | `manual`)
  y `registered_by_user_id`, para poder mostrar/auditar cómo y quién confirmó cada
  fila.
- **Endurecimiento de autorización** (seguridad, ver más abajo):
  - `GET /attendance/qr` pasa a exigir ser entrenador del `team_id` consultado.
    Hoy cualquiera autenticado puede pedir el QR de cualquier equipo.
  - `POST /attendance/team/:team_id/session/:training_session_id` pasa a exigir
    que el usuario autenticado sea **miembro del grupo** al que pertenece la
    sesión. Hoy cualquiera autenticado puede cargar asistencia ajena.
  - Todos los endpoints nuevos exigen entrenador del equipo.
- **FK real en `attendances.training_session_id`** → `session_instances(id)`. La
  columna quedó documentada como FK opaca porque `training_sessions` no existe;
  la tabla contra la que sí se puede apuntar (`session_instances`) ya existe.

## No alcance

- **Escaneo del QR por parte del corredor** y su historial de asistencias: es
  otra sesión, según lo acordado. Este change solo construye el lado entrenador;
  el `POST` de alta del corredor existe y se mantiene ( endurecido, no rediseñado).
- **Marcado de asistencia a distancia / en vivo** desde la pantalla de la sesión.
- **Reportes, exportación a Excel/CSV, gráficos históricos** y cualquier
  agregación que no sea el resumen por sesión.
- **Reestructurar `attendances` para apuntar a `group_calendar_days`**
  (`group_id` + `date` + `is_presencial` desde la fuente). Se evaluó y se
  descarta: la clave semántica acordada es `session_instance_id`, y el JOIN de
  validación cubre el caso de uso sin una migración de datos. Queda anotado
  para cuando aparezca el histórico.
- **Tabla `training_sessions`** y su constante cambio en el calendario.
- Notificaciones push al registrar/borrar asistencia.

## Métrica de éxito

- `go test ./...` en verde con coverage global ≥ 80 (gate de CI, no se baja).
- Los 4 endpoints nuevos y los 2 endurecidos tienen test de matriz de autorización: `403` para no
  entrenador, `404` para equipo/grupo/inexistente, `422` para sesión no
  presencial o futura.
- El upsert masivo es **idempotente**: guardar dos veces el mismo lote no crea
  filas duplicadas ni revierte las ya existentes.
- Un `DELETE` seguido de un nuevo marcado del mismo corredor funciona: el borrado
  es físico, así que libera la fila del índice `UNIQUE` y el re-marcado no choca
  (justificación completa en D4 de `design.md`).

## What Changes

- **4 endpoints nuevos** bajo `/api/v1/attendance` (más el endurecimiento de 2
  existentes):
  - `GET /api/v1/groups/:group_id/attendance-sessions?team_id=` — sesiones
    presenciales ocurridas del grupo, con conteos por sesión.
  - `GET /api/v1/attendance/session/:session_instance_id?team_id=&group_id=` —
    grilla (roster + estado) + agregados de la sesión.
  - `POST /api/v1/attendance/bulk` — upsert por lotes (crear/actualizar).
  - `DELETE /api/v1/attendance/:id?team_id=` — borrado individual.
  - (y endurecimiento de los 2 existentes: `GET /qr`, `POST /team/.../session/...`).
- **2 columnas aditivas** en `attendances`: `source` y `registered_by_user_id`
  (nullable, backfill desde las filas existentes).
- **1 FK real**: `attendances.training_session_id → session_instances(id)`.
  Requiere limpiar las filas de prueba manuales que apuntan a
  `training_session_id` inexistentes antes de agregar la constraint.
- **Nueva capability** `attendance-management` con el requirement set completo
  (ADDED, ver nota abajo). La capability `attendance-qr` **nunca fue archivada**
  (`openspec/specs/` solo tiene `user-bank-alias`, `workout-feedback`,
  `workout-feedback-gps-points` y `session-registration-review`), así que sus
  requirements endurecidos se declaran como ADDED acá en vez de MODIFIED,
  siguiendo el precedente de `asignacion-por-instanciacion`
  (`openspec/changes/asignacion-por-instanciacion/specs/group-calendar/spec.md`).
- Documentación Swagger regenerada (`swag init`) + filas nuevas en el
  `README.md`.

## Capabilities

### New Capabilities

- `attendance-management`: gestión de asistencia de una sesión presencial desde
  el panel del entrenador — listado de sesiones presenciales ocurridas con
  conteos, grilla roster + estado con agregados, upsert masivo idempotente,
  borrado individual, provenance (`source`/`registered_by`), y endurecimiento de
  la autorización de los endpoints de QR y de alta del corredor.

### Modified Capabilities

- (ninguna — `attendance-qr` nunca se archivó, por lo que sus requirements
  modificados se redeclaran completos bajo ADDED dentro de la capability nueva)

## Impact

**Código nuevo:**

- `cmd/api/domains/attendance/` — DTOs: `SessionAttendanceResponse`, `AttendanceRow`,
  `AttendanceSummary`, `BulkAttendanceRequest`/`Response`, `SessionAttendanceRow`
  (la sesión del selector con sus conteos).
- `cmd/api/daos/attendance_dao.go` — métodos nuevos: `BulkUpsert`, `DeleteByID`,
  `GetGrid` (roster × asistencia con LEFT JOIN), `FindPastPresencialSessionsForGroup`.
- `cmd/api/daos/team_membership_dao.go` — helper `IsGroupMember` (el `POST` de
  alta del corredor hoy no valida pertenencia al grupo).
- `cmd/api/services/attendance_service.go` — lógica de los 4 endpoints nuevos +
  el guard de autorización de los 2 endurecidos.
- `cmd/api/controllers/attendance_controller.go` — handlers + anotaciones Swagger.
- `cmd/api/app/url_mappings.go` + `app.go` — wiring de rutas y dependencias.
- `cmd/api/daos/attendance_migration_test.go` (o equivalente) — cobertura de la
  FK y de las columnas nuevas.

**Código modificado:**

- `cmd/api/domains/dbs/attendance.go` — `Source`, `RegisteredByUserID`; comentario
  de `TrainingSessionID` actualizado (ya no es opaca).
- `cmd/api/services/attendance_service.go` + `attendance_dao.go` — guards de
  autorización sobre `GenerateQR` y `Register`.
- `cmd/api/daos/attendance_dao_test.go`, `attendance_service_test.go`,
  `attendance_controller_test.go` — tests de la matriz nueva.
- `cmd/api/docs/` — regenerado, no editado a mano.
- `README.md` — tabla de endpoints de asistencia.

**Dependencias:** ninguna nueva. Reusa `github.com/skip2/go-qrcode` (ya en
`go.mod`) y no necesita librería de métricas: los agregados se calculan en el
service.

**Otros repos:**

- El frontend (`paceron-frontend`) tiene su propio change gemelo,
  `gestion-asistencia-entrenador`, que consume estos endpoints. **Este change es
  el que define el contrato**; el front no puede avanzar sin él.
- La única acción destructiva es el **truncado del set de pruebas manual** de
  `attendances` (el equipo confirmó que se puede descartar) antes de agregar la
  FK. Ninguna fila de producción se toca: la migración solo agrega columnas
  nullable y la constraint, y la limpieza es reversible desde el backup.
