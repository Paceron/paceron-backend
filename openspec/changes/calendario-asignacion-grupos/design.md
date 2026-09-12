## D1 — Modelo de datos: `group_calendar_days`

| Campo | Tipo Go/GORM | Notas |
|---|---|---|
| `ID` | `int64` PK | |
| `GroupID` | `int64` not null | `UNIQUE(group_id, date)` — índice compuesto |
| `Date` | `time.Time` not null | `gorm:"type:date"`, fecha real (no `sequence_no`) |
| `Kind` | `string` not null | enum app-level `rest`/`other`/`training`/`cancelled` (D2) |
| `OtherName` | `*string` | solo si `kind=other` |
| `SessionID` | `*int64` | solo si `kind IN (training, cancelled)` — se mantiene en `cancelled` como contexto de qué sesión era |
| `CancelledReason` | `*string` | solo si `kind=cancelled` |
| `IsPresencial` | `bool` not null default false | solo tiene sentido si `kind IN (training, cancelled)` |
| `PresencialTime` | `*time.Time` | `gorm:"type:time"`, mismo criterio que `PlanDay.DefaultTime` del change anterior (D3 de ese design) |
| `PresencialLocation` | `*string` | `gorm:"type:jsonb"`, reusa el struct `trainingplan.Location` del change 1 (D4 de ese design) |
| `SourcePlanID` | `*int64` | informativo, `ON DELETE SET NULL` — no hay FK física en este repo (ver `team_dao.go`), se limpia a mano en el service cuando se borra un `TrainingPlan` (extiende `TrainingPlanService.Delete` de la task correspondiente en el change 1 — o se agrega acá si ese change ya está mergeado a la rama al llegar a esta tarea) |
| `CreatedAt`/`UpdatedAt` | `time.Time` | |

## D2 — Enum `GroupCalendarDayKind`

`cmd/api/domains/constants/group_calendar_day_kind.go`: `rest`, `other`, `training`, `cancelled` — 4 valores, un superset de `PlanDayKind` (que no tiene `cancelled`, no tiene sentido en un template). Mismo patrón `TeamUserRole`.

## D3 — Validación al guardar (`PUT` día individual, `stamp`, `bulk`)

Server-side siempre, spec frontend §3.1 exacta:
- `kind=other` ⇒ `other_name` no nulo.
- `kind=training` ⇒ `session_id` no nulo, `other_name`/`cancelled_reason` nulos.
- `kind=cancelled` ⇒ `cancelled_reason` no nulo. **Transición solo permitida desde `kind=training`** — cancelar un día `rest`/vacío es `422`. Esto implica que el service debe leer el estado actual del día (si existe) antes de aceptar un `PUT` a `cancelled`.
- `kind=rest` ⇒ `other_name`/`session_id`/`cancelled_reason` nulos.
- `is_presencial=true` ⇒ `presencial_time`/`presencial_location` no nulos. `is_presencial=false` ⇒ limpiar ambos a `nil` aunque vengan en el body (mismo criterio D5 del change 1 para `default_presencial`).

## D4 — Permisos por endpoint (no la convención general del resto del backend)

Tabla exacta de la spec frontend §4 — el controller valida esto **antes** de llamar al service (guard de autorización, no de negocio):

| Endpoint | Quién |
|---|---|
| `GET /groups/{id}/calendar` | entrenador dueño **o** cualquier corredor miembro (lectura) |
| `PUT`/`DELETE`/`stamp`/`bulk`/`bulk-clear`/`shift` | **solo** entrenador dueño — `403` para corredores, incluso miembros |
| `stamp` (chequeo extra) | el `plan_id` debe ser `owner_id` = el mismo entrenador que administra el grupo — `403` si intenta estampar un plan ajeno |
| `GET /users/{id}/next-session`, `GET /users/{id}/calendar-summary` | `{id}` debe ser el propio usuario del token — `403` si no coincide |

Implementación: reusar `teamUserDao.FindByTeamAndUser`/`groupDao.FindByID` (grupo → team → owner) para el chequeo de "dueño del equipo", igual patrón que `isEntrenadorOfTeam` en `team_service.go`. Para "miembro del grupo" (lectura), `groupUserDao` con `deleted_at IS NULL`.

## D5 — `stamp`: copiar un plan al calendario

`POST /groups/{id}/calendar/stamp` `{plan_id, start_date, force?}`:

1. Validar permisos (D4: dueño del grupo + plan del mismo `owner_id`).
2. Leer `TrainingPlan` + sus `PlanDay` ordenados por `sequence_no` (404 si no existe).
3. Calcular la fecha destino de cada día: `start_date + (sequence_no - 1)` días.
4. Si `force` no es `true`: chequear con una sola query si alguna de esas fechas ya tiene fila en `group_calendar_days` para ese `group_id` — si hay alguna, responder `409` con la lista de fechas en conflicto, **sin escribir nada**.
5. Si `force=true` o no había conflictos: upsert (crear o reemplazar) una fila de `GroupCalendarDay` por cada `PlanDay`, copiando `kind`/`other_name`/`session_id`/`default_presencial→is_presencial`/`default_time→presencial_time`/`default_location→presencial_location`, y `source_plan_id = plan.id`. `cancelled_reason` siempre `nil` (un stamp nunca crea un día `cancelled`, ese `kind` no existe en el template).
6. Devolver el array de filas creadas/reemplazadas.

Todo el paso 5 en una transacción (`db.Transaction`, mismo patrón que `ApplyTeamMembershipGate`).

## D6 — `bulk` / `bulk-clear` / `shift`

- **`bulk`** `{dates:[...], kind, session_id?, other_name?, is_presencial?, presencial_time?, presencial_location?}`: mismo contenido a todas las fechas listadas, valida D3 una vez (el body es uno solo, no por fecha) y aplica upsert a cada fecha en una transacción. `200` con el array resultante.
- **`bulk-clear`** `{dates:[...]}`: `DELETE` físico de las filas de esas fechas (las que existan; fechas sin fila no son error). `204`.
- **`shift`** `{from_date, days}` (`days` entero positivo): todas las filas con `date >= from_date` pasan a `date + days`. Antes de aplicar, chequear que ninguna fecha destino choque con una fila que **no** se está corriendo (es decir, una fecha `< from_date` que coincida con `date_original + days` de alguna fila corrida) — si choca, `409` sin aplicar nada. Implementación sugerida: leer todas las filas afectadas, calcular destinos en memoria, chequear colisión contra las filas NO afectadas (`date < from_date`), recién ahí un `UPDATE` masivo o fila por fila en una transacción.

## D7 — Vistas del corredor

- **`GET /users/{id}/next-session`**: entre TODOS los grupos de los que `{id}` es miembro activo (`teamUserDao`/`groupUserDao`), buscar la `GroupCalendarDay` con `date >= hoy` y `kind IN (training, cancelled)` de fecha más próxima (una sola query con `ORDER BY date ASC LIMIT 1` sobre un `IN` de group_ids del usuario). `200` con `{group_id,date,session_id,is_presencial,presencial_time?,presencial_location?}` o `204` si no hay ninguna.
- **`GET /users/{id}/calendar-summary`**: `200` array `{group_id,group_name}`, un ítem por grupo del que es miembro — no trae el calendario en sí, cada ítem dispara `GET /groups/{id}/calendar` aparte desde el frontend.

## D8 — Clonado por divergencia (extiende `PUT /sessions/{id}` del change 1)

Nuevos campos opcionales en el body ya existente de `PUT /sessions/{id}`: `{..., exclude_group_ids?, clone_name?, clone_description?}`.

```
si exclude_group_ids viene vacío/omitido:
    PUT normal (comportamiento ya implementado en el change 1), clone_name/clone_description se ignoran
si no:
    TRANSACCIÓN:
        1. clon = SessionService.cloneInternal(original, name=clone_name ?? original.Name+" (copia)", description=clone_description ?? original.Description)
        2. UPDATE group_calendar_days SET session_id = clon.ID WHERE group_id IN (exclude_group_ids) AND session_id = {id}
        3. aplicar el resto del PUT (campos normales) a la sesión original
    si cualquier paso falla → rollback completo, ningún cambio parcial
```

`cloneInternal` es la misma lógica interna que ya expone `POST /sessions/{id}/clone` (change 1, D8 de ese design) — se refactoriza a una función privada reusable por ambos call sites al llegar a esta tarea, no se duplica el código de copia profunda de `session_exercises`.

**Un solo clon compartido**: los N grupos de `exclude_group_ids` apuntan todos al mismo `clon.ID`, una sola vez.

## D9 — `GET /sessions/{id}/assigned-groups`

Endpoint auxiliar que dispara el frontend antes de mostrar el form de edición (spec frontend de calendario §5, paso 1): `200` array `{group_id,group_name}` distinct, de cualquier `GroupCalendarDay.session_id = {id}` sin importar la fecha. Vacío ⇒ el frontend no ofrece el checklist de divergencia.

## D10 — Borrado de plan: limpiar `source_plan_id`

Al borrar un `TrainingPlan` (`DELETE /training-plans/{id}`, ya implementado en el change 1), este change agrega un `UPDATE group_calendar_days SET source_plan_id = NULL WHERE source_plan_id = {id}` antes/junto al borrado físico del plan — sin tocar `kind`/`session_id`/nada más de esas filas (ya copiaron los datos físicamente, D5). Se implementa como una extensión de `TrainingPlanService.Delete` que ahora también recibe `groupCalendarDayDao` — o, si el change 1 ya cerró esa interfaz sin este dao, se resuelve con una query directa desde `CalendarService` invocada por el mismo endpoint (decisión de implementación, no de producto: cualquiera de las dos formas cumple el mismo contrato observable).

## D11 — Endpoints (tabla completa)

Mismo formato de error `{"message":"..."}` que el change 1 (D6 de ese design) — sigue siendo el mismo dominio nuevo, misma convención de frontend.

| Método | Path | Body | Respuesta |
|---|---|---|---|
| `GET` | `/groups/{id}/calendar?from={date}&to={date}` | — | `200` array, ambos límites inclusive y obligatorios |
| `PUT` | `/groups/{id}/calendar/{date}` | D3 | `200` — upsert |
| `DELETE` | `/groups/{id}/calendar/{date}` | — | `204` — borra la fila |
| `POST` | `/groups/{id}/calendar/stamp` | `{plan_id,start_date,force?}` | `201` \| `409` (D5) |
| `POST` | `/groups/{id}/calendar/bulk` | D6 | `200` |
| `POST` | `/groups/{id}/calendar/bulk-clear` | `{dates:[...]}` | `204` |
| `POST` | `/groups/{id}/calendar/shift` | `{from_date,days}` | `200` \| `409` |
| `GET` | `/users/{id}/next-session` | — | `200` \| `204` |
| `GET` | `/users/{id}/calendar-summary` | — | `200` array |
| `GET` | `/sessions/{id}/assigned-groups` | — | `200` array (D9) |
| `PUT` | `/sessions/{id}` (extiende change 1) | `{...,exclude_group_ids?,clone_name?,clone_description?}` | `200` (D8) |

## D12 — Capas y archivos (sin delegate)

Mismo criterio que el change 1 — `CalendarService` toca DAOs de varios dominios directo (`groupCalendarDayDao`, `trainingPlanDao`, `planDayDao`, `teamDao`, `groupDao`, `teamUserDao`, `groupUserDao`), sin pasar por `TeamService`/`TrainingPlanService`. La única extensión a un service ya existente es `SessionService.Update` (D8), en el mismo archivo del change 1.

- `cmd/api/domains/dbs/group_calendar_day.go`
- `cmd/api/domains/constants/group_calendar_day_kind.go`
- `cmd/api/domains/calendar/{calendar_day_request.go,calendar_day_response.go,next_session_response.go,calendar_summary_response.go}`
- `cmd/api/daos/group_calendar_day_dao.go`
- `cmd/api/services/calendar_service.go`
- `cmd/api/services/session_service.go` (extendido, D8)
- `cmd/api/controllers/calendar_controller.go`
- `cmd/api/controllers/session_controller.go` (extendido — `assigned-groups` handler + el `PUT` existente acepta los 3 campos nuevos)
- `cmd/api/app/url_mappings.go` / `cmd/api/app/app.go` (wiring)
- `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate)
