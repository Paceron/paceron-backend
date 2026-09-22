# Catálogo de entrenamiento y calendario de grupos

Documento de referencia completo del dominio **catálogo** (`Exercise`, `Session`, `TrainingPlan`) y **calendario** (`GroupCalendarDay`). Cubre modelo de datos, reglas de validación, guards de autorización, endpoints y el mecanismo de instanciación al asignar contenido. Fuente de verdad: el código en `cmd/api/domains/dbs`, `cmd/api/services`, `cmd/api/controllers` — este doc lo resume y explica, no lo reemplaza.

Origen de las specs: `openspec/changes/catalogo-planes-entrenamiento/`, `openspec/changes/calendario-asignacion-grupos/`, (mecanismo vigente de instanciación) `openspec/changes/asignacion-por-instanciacion/` y (colisión presencial + banners + calendario agregado) `openspec/changes/colisiones-presenciales-y-calendario-agregado/` (`design.md`/`specs/*/spec.md` tienen el detalle de decisión con Given/When/Then; acá va la síntesis operativa).

## 1. Relación entre entidades

```
Exercise (catálogo, reusable) ──┐
                                 ├─< SessionExercise >── Session (catálogo, reusable)
                                 │        (rol: warmup/main/cooldown)
                                 │
Session ──< PlanDay >── TrainingPlan (template reusable, días 1..N)
                                  │
                                  │  POST /groups/{id}/calendar/stamp
                                  │  (o PUT/bulk de día individual)
                                  ▼
                         GroupCalendarDay ──> SessionInstance ──< SessionExerciseInstance >── ExerciseInstance
                         (calendario REAL de un grupo,      (copias congeladas, inmutables,
                          una fila por (group_id, date)      separadas del catálogo:
                          con contenido)                     `session_instances`/
                                                             `session_exercise_instances`/
                                                             `exercise_instances`)
```

Puntos clave de esta jerarquía:

- **`Exercise` y `Session` son catálogo reusable de un entrenador (`owner_id`)**, no atado a ningún plan ni calendario. Un mismo `Exercise` puede estar en N `Session`; una misma `Session` puede estar en N `PlanDay` de N planes distintos, y estampada en N días de calendario de N grupos distintos.
- **`TrainingPlan` es un template** — sus `PlanDay` dicen "día 3 = training con `Session` X, presencial a las 18:00" pero no tiene fecha real. Estampar (`stamp`) es lo que traduce ese template a fechas concretas en `GroupCalendarDay`.
- **`GroupCalendarDay` es la única entidad con fecha real.** Es dispersa: si no hay fila para `(group_id, date)`, ese día está vacío. Copia físicamente `kind`, `is_presencial`, `presencial_time_from`, `presencial_time_to`, `presencial_location` al momento del stamp (ver §8); el contenido de entrenamiento no lo referencia **ni lo copia en columnas propias** — apunta a `session_instance_id`, una copia congelada creada en el momento de la asignación (ver §8).

## 2. Dónde "viven" los ejercicios de una sesión (aclaración habitual)

Pregunta recurrente: *cuando guardo una sesión, ¿los ejercicios quedan embebidos ahí o siguen viviendo en el catálogo?*

- `Session` **no tiene un array embebido de ejercicios.** Es una tabla propia (`sessions`) sin columna de ejercicios.
- La relación vive en una tabla intermedia dedicada: **`session_exercises`** (`SessionExercise`), con `session_id`, `exercise_id`, `role`, `repeat_count`, `rest_minutes`. Es decir, cada combinación "este ejercicio va en esta sesión con este rol" es una fila propia — el `Exercise` original en `exercises` no se copia ni se mueve, solo se referencia por `exercise_id`.
- El **`PUT /sessions/{id}` reemplaza el set de `SessionExercise` completo**, no hace un patch fila por fila: borra todas las filas de esa sesión y crea exactamente las del array recibido, en el orden dado (`SessionExerciseDao.ReplaceForSession`, transaccional). Si mandás una lista distinta, las filas viejas desaparecen y aparecen las nuevas — pero el `Exercise` catalogado (`exercises`) nunca se toca por este PUT.
- El borrado de un `Exercise` es **lógico** (`deleted_at`), justamente para que una `Session` que ya lo referencia (vía `SessionExercise.exercise_id`) siga resolviendo el detalle completo en `GET /sessions/{id}` aunque ese ejercicio ya no aparezca en `GET /exercises?owner_id=...` (dejó de listarse para catálogo nuevo, pero no rompe lo ya armado).
- Un `Exercise` **no exige combinación de campos por `kind`** (`kind`, `intensity`, `muscle_group`, `minutes`, `distance_m`, `speed_kph` son todos independientes entre sí, todos opcionales salvo `kind`) — sí se valida que cada valor (si viene) pertenezca a su enum.
- Al **clonar una sesión** (`POST /sessions/{id}/clone`) se hace copia profunda de sus `SessionExercise` (mismos `exercise_id`, mismo `role`/`repeat_count`/`rest_minutes`) apuntando al `Session` nuevo — los `Exercise` referenciados siguen siendo los mismos, no se clonan.

En síntesis: **el ejercicio vive una sola vez en `exercises`; cada sesión que lo usa agrega una fila en `session_exercises` que apunta a él por ID.** Editar la sesión nunca edita el ejercicio catalogado, y borrar/editar el ejercicio catalogado no rompe sesiones que ya lo referencian (salvo que sea un borrado físico, que no existe para `Exercise`).

**Editar un `Exercise` que ya está asignado a un día de calendario no pisa el historial** — bajo el modelo de instanciación (§8), un día asignado usa copias congeladas (`ExerciseInstance`), nunca referencias vivas al catálogo. Editar o borrar el `Exercise` original no cambia nada de lo ya asignado. (Antes de la instanciación, esto era un gap real: editar el ejercicio pegaba en vivo incluso sobre días ya pasados.)

## 3. Modelo de datos

### 3.1 `exercises` (`Exercise`)

| Columna | Tipo | Notas |
|---|---|---|
| `id` | PK | |
| `owner_id` | not null | entrenador dueño |
| `name` | not null | |
| `description` | nullable | |
| `kind` | not null | enum `ExerciseKind` |
| `intensity` | nullable | enum `ExerciseIntensity` |
| `minutes` | nullable | int |
| `distance_m` | nullable | int |
| `speed_kph` | nullable | `numeric(4,1)` |
| `muscle_group` | nullable | enum `MuscleGroup` |
| `video_url` | nullable | sin endpoint de escritura propio hoy (queda para foto/video futuro) |
| `deleted_at` | nullable | **soft delete** |
| `created_at`/`updated_at` | | |

Sin reglas cruzadas entre campos — cualquier combinación de opcionales es válida mientras cada uno (si viene) sea un valor válido de su enum.

### 3.2 `sessions` (`Session`) + `session_exercises` (`SessionExercise`)

`sessions`: `id`, `owner_id`, `name`, `description`, `deleted_at` (**soft delete**), timestamps. Sin columna de ejercicios (ver §2).

`session_exercises`: `id`, `session_id` (not null), `exercise_id` (not null), `role` (enum `SessionExerciseRole`: `warmup`/`main`/`cooldown`), `repeat_count` (default 1), `rest_minutes` (default 0). Sin columna de orden explícito — el `id` autoincremental desempata el orden de inserción, que es el orden recibido en el request.

### 3.3 `training_plans` (`TrainingPlan`) + `plan_days` (`PlanDay`)

`training_plans`: `id`, `owner_id`, `name`, `description`, timestamps. **Sin `deleted_at` — borrado físico** (única entidad del dominio catálogo con delete físico; decisión explícita, ver D1 en `catalogo-planes-entrenamiento/design.md`).

`plan_days`: `id`, `plan_id`, `sequence_no` (1..N sin huecos/repetidos), `kind` (enum `PlanDayKind`: `rest`/`other`/`training`), `other_name`, `session_id`, `default_presencial` (bool, default false), `default_time_from`/`default_time_to` (`time`, nullable, `to > from` si presencial), `default_location` (`jsonb`, nullable — shape `Location{lat,lng,label?}`).

`default_*` son **informativos para el momento del stamp** — no afectan nada del template en sí, solo se copian a `GroupCalendarDay` cuando se estampa (§7).

### 3.4 `group_calendar_days` (`GroupCalendarDay`)

Tabla dispersa: una fila por `(group_id, date)` **con contenido**. `UNIQUE(group_id, date)` vía índice compuesto (`idx_group_calendar_day_group_date`).

| Columna | Notas |
|---|---|
| `group_id`, `date` | únicos juntos |
| `kind` | enum `GroupCalendarDayKind`: `rest`/`other`/`training`/`cancelled` (4to valor, `cancelled`, solo existe acá — no tiene sentido en un template) |
| `other_name`, `session_instance_id`, `cancelled_reason` | según `kind`. `session_instance_id` apunta a `session_instances` (copia congelada, nunca al catálogo) — ver §8. El `training` sigue llegando con `session_id` de catálogo en el **request**, pero el service lo instancia y guarda el ID de la instancia, no el del catálogo |
| `is_presencial`, `presencial_time_from`, `presencial_time_to`, `presencial_location` | igual shape que `default_*` de `PlanDay`, `to > from` obligatorio si presencial |
| `source_plan_id` | nullable — de qué plan vino este día si fue estampado; se limpia a `null` (no se borra la fila) si el plan origen se borra |
| `created_at`/`updated_at` | |

Sin `deleted_at` — el `DELETE /groups/{id}/calendar/{date}` borra la fila físicamente (vaciar un día = que no exista fila).

## 4. Constants / enums

| Enum | Valores | Uso |
|---|---|---|
| `ExerciseKind` | `walking`, `jogging`, `elongation`, `cruising`, `running` | `Exercise.kind` |
| `ExerciseIntensity` | `light`, `moderate`, `vigorous` | `Exercise.intensity` |
| `MuscleGroup` | `cuadriceps`, `isquiotibiales`, `gemelos`, `gluteos`, `aductores`, `psoas`, `lumbares`, `core` | `Exercise.muscle_group` |
| `SessionExerciseRole` | `warmup`, `main`, `cooldown` | `SessionExercise.role` |
| `PlanDayKind` | `rest`, `other`, `training` | `PlanDay.kind` |
| `GroupCalendarDayKind` | `rest`, `other`, `training`, `cancelled` | `GroupCalendarDay.kind` (superset de `PlanDayKind` + `cancelled`) |

## 5. Guards de autorización

Todas las rutas de este dominio están detrás de `AuthMiddleware()` (JWT). Sobre eso, cada service aplica su propio check ABAC (no hay middleware de autorización genérico — decisión ya tomada para todo el backend, ver memoria de sesiones previas):

| Entidad / acción | Guard |
|---|---|
| `Exercise`/`Session`/`TrainingPlan` — Create | `req.owner_id == callerID` (el JWT no puede crear a nombre de otro) → si no, `403 no autorizado` |
| `Exercise`/`Session`/`TrainingPlan` — Update/Delete/Clone | `existing.OwnerID == callerID` → si no, `403` |
| `Exercise`/`Session`/`TrainingPlan` — Get/List | **sin guard de ownership** — cualquier usuario autenticado puede leer por ID o listar por `owner_id` arbitrario. No hay noción de "catálogo privado" hoy. |
| Calendario — lectura (`GetRange`) | `isGroupOwnerOrMember`: dueño del equipo del grupo (`Team.owner_id == callerID`) **o** miembro activo del grupo (`GroupUser` existe) |
| Calendario — escritura (`PutDay`, `DeleteDay`, `Stamp`, `Bulk`, `BulkClear`, `Shift`) | `isGroupOwner`: solo el entrenador dueño del equipo del grupo (`Team.owner_id == callerID`) — un miembro/runner no puede escribir calendario |
| `Stamp` — plan usado | además de ser dueño del grupo, `plan.OwnerID == callerID` (`403 el plan no pertenece al entrenador dueño del grupo` si no) |
| `NextSession`/`CalendarSummary`/`NextPresencialSession`/`MemberCalendar`/`AdministeredCalendar` (`/users/{id}/...`) | `userID == callerID` estricto — nadie puede consultar los datos de otro usuario, ni el propio entrenador (`403 no podés consultar los datos de otro usuario`) |

## 6. Endpoints

Convención de error de todo este dominio: **`{"message": "..."}`** vía `respondCatalogError` — deliberadamente distinto del `apierror.APIError` SCREAMING_SNAKE del resto del backend (pedido explícito del frontend en la spec original).

### 6.1 Catálogo de ejercicios

| Método | Ruta | Body | Éxito | Errores |
|---|---|---|---|---|
| POST | `/api/v1/exercises` | `ExerciseRequest` | `201` | `400` payload, `403` |
| GET | `/api/v1/exercises?owner_id=` | — | `200` array | `400` |
| GET | `/api/v1/exercises/{id}` | — | `200` | `404` |
| PUT | `/api/v1/exercises/{id}` | `ExerciseRequest` (reemplazo total) | `200` | `400`, `403`, `404` |
| DELETE | `/api/v1/exercises/{id}` | — | `204` (soft delete) | `403`, `404` |
| POST | `/api/v1/exercises/{id}/clone` | — | `201` | `403`, `404` |

`ExerciseRequest`: `owner_id*`, `name*`, `description`, `kind*`, `intensity`, `minutes`, `distance_m`, `speed_kph`, `muscle_group` (`*` = requerido).

### 6.2 Catálogo de sesiones

| Método | Ruta | Body | Éxito | Errores |
|---|---|---|---|---|
| POST | `/api/v1/sessions` | `SessionRequest` | `201` | `400`, `403`, `422` |
| GET | `/api/v1/sessions?owner_id=` | — | `200` array | `400` |
| GET | `/api/v1/sessions/{id}` | — | `200` | `404` |
| PUT | `/api/v1/sessions/{id}` | `SessionRequest` (reemplaza `exercises` entero) | `200` | `400`, `403`, `404`, `422` |
| DELETE | `/api/v1/sessions/{id}` | — | `204` (soft delete) | `403`, `404` |
| POST | `/api/v1/sessions/{id}/clone` | — | `201` | `403`, `404` |

`SessionRequest`: `owner_id*`, `name*`, `description`, `exercises*` (array de `{exercise_id*, role*, repeat_count?, rest_minutes?}`, mínimo 1 de cada rol). Sin campos extra: el PUT reemplaza `exercises` entero y nunca deja tocar asignaciones de calendario — esas usan copias congeladas (ver §8).

Errores `422` propios: rol faltante (`la sesión debe tener al menos un ejercicio de cada rol`), `role` inválido, `exercise_id` inexistente/borrado.

### 6.3 Catálogo de planes de entrenamiento

| Método | Ruta | Body | Éxito | Errores |
|---|---|---|---|---|
| POST | `/api/v1/training-plans` | `TrainingPlanRequest` | `201` | `400`, `403`, `422` |
| GET | `/api/v1/training-plans?owner_id=` | — | `200` array | `400` |
| GET | `/api/v1/training-plans/{id}` | — | `200` | `404` |
| PUT | `/api/v1/training-plans/{id}` | `TrainingPlanUpdateRequest` (parcial; `days` si viene reemplaza el set entero) | `200` | `400`, `403`, `404`, `422` |
| DELETE | `/api/v1/training-plans/{id}` | — | `204` (**físico**) | `403`, `404` |
| POST | `/api/v1/training-plans/{id}/clone` | — | `201` | `403`, `404` |

`TrainingPlanRequest`: `owner_id*`, `name*`, `description`, `days*` (2..31 items). Cada `PlanDayRequest`: `sequence_no*`, `kind*`, `other_name`, `session_id`, `default_presencial`, `default_time_from`/`default_time_to` (`"HH:MM"`, `to > from`), `default_location` (`{lat,lng,label?}`).

Errores `422` propios: cantidad de días fuera de 2..31, `sequence_no` con hueco/repetido, `kind` inválido, combinación de campos inválida para el `kind` (ver §6.4), `session_id` inexistente, `default_time_from`/`default_time_to` con formato inválido o `default_time_to` no posterior a `default_time_from`.

### 6.4 Combinación de campos por `kind` (`PlanDay` y `GroupCalendarDay`)

| `kind` | Requiere | Prohíbe |
|---|---|---|
| `training` | `session_id` | `other_name` |
| `other` | `other_name` | `session_id` |
| `rest` | — | `other_name`, `session_id` |
| `cancelled` (solo `GroupCalendarDay`) | `cancelled_reason` | — |

Además, en cualquier `kind`: si `is_presencial`/`default_presencial = true` → `presencial_time_from`/`presencial_time_to` (o `default_time_from`/`default_time_to`) y `presencial_location`/`default_location` son obligatorios, con `time_to > time_from` (si no, `422`); si es `false`, todos se limpian a `null`.

`cancelled` solo se puede aplicar sobre un día que **actualmente** está en `training` (`ErrCalendarInvalidCancelTransition` si no) — es una transición, no un estado inicial.

### 6.5 Calendario de grupo

| Método | Ruta | Body | Éxito | Errores |
|---|---|---|---|---|
| GET | `/api/v1/groups/{id}/calendar?from=&to=` | — | `200` array | `400`, `403` |
| PUT | `/api/v1/groups/{id}/calendar/{date}` | `CalendarDayRequest` | `200` (upsert) `CalendarDayResponse` + `same_team_warnings` opcional | `400`, `403`, `409` (colisión presencial), `422` (incl. día cerrado) |
| DELETE | `/api/v1/groups/{id}/calendar/{date}` | — | `204` | `400`, `403`, `422` (día cerrado) |
| POST | `/api/v1/groups/{id}/calendar/stamp` | `StampRequest{plan_id*, start_date*, force?, exclude_dates?}` | `201` wrapper `{days, same_team_warnings?}` | `400`, `403`, `404` (plan), `409` (conflicto de fechas o colisión presencial), `422` (día cerrado o `exclude_dates` con formato inválido) |
| POST | `/api/v1/groups/{id}/calendar/bulk` | `BulkRequest{dates*, kind*, session_id?, other_name?, is_presencial?, presencial_time_from?, presencial_time_to?, presencial_location?}` | `200` wrapper `{days, same_team_warnings?}` | `400`, `403`, `409` (colisión presencial), `422` (incl. día cerrado con lista de fechas) |
| POST | `/api/v1/groups/{id}/calendar/bulk-clear` | `BulkClearRequest{dates*}` | `204` | `400`, `403`, `422` (día cerrado, lista de fechas) |
| POST | `/api/v1/groups/{id}/calendar/shift` | `ShiftRequest{from_date*, days*}` | `200` wrapper `{days, same_team_warnings?}` | `400`, `403`, `409` (colisión de fechas o colisión presencial), `422` (día cerrado, lista de fechas) |
| GET | `/api/v1/users/{id}/next-session` | — | `200` siempre `{next_cancelled, next_training}` | `400`, `403` |
| GET | `/api/v1/users/{id}/next-presencial-session` | — | `200` / `204` sin próxima | `400`, `403` |
| GET | `/api/v1/users/{id}/member-calendar?from=&to=` | — | `200` array | `400`, `403` |
| GET | `/api/v1/users/{id}/administered-calendar?from=&to=` | — | `200` array | `400`, `403` |
| GET | `/api/v1/users/{id}/calendar-summary` | — | `200` array `{group_id,group_name}` | `400`, `403` |

`Stamp`: copia cada `PlanDay` del plan a `start_date + (sequence_no - 1)` días, marcando `source_plan_id`. Sin `force=true`, si alguna fecha destino ya tiene contenido → `409 hay fechas con contenido existente` y no escribe nada.

Con `exclude_dates` (opcional, array de fechas `YYYY-MM-DD`; change `stamp-exclude-dates`): cada fecha del set que caiga dentro del rango objetivo se salta **por completo** — su fila e instancia quedan intactas, no cuenta para el `409` de conflictos ni para el `422` de día cerrado, y no aparece en la respuesta. `force` sigue aplicando igual sobre las fechas NO excluidas. Una fecha excluida fuera del rango se ignora; formato inválido → `422 ErrCalendarInvalidDate` sin escribir nada; rango totalmente excluido → `201` con `[]`. Omitir el campo o enviar `[]` = comportamiento idéntico al previo.

`Shift`: mueve todas las filas desde `from_date` en adelante, `days` posiciones (entero positivo). Antes de escribir valida que ninguna fecha destino choque con una fila **anterior a `from_date`** que quede fuera del rango desplazado (`409 el corrimiento haría chocar dos fechas`).

`NextSession`, `NextPresencialSession`, `MemberCalendar` y `AdministeredCalendar` se documentan en §8.7 (banners) y §8.8 (calendario agregado).

## 7. Stamp — copiado físico, no por referencia

Al estampar (`Stamp`) un plan, cada `GroupCalendarDay` generado copia **por valor** `kind`, `other_name`, `is_presencial`, `presencial_time_from`, `presencial_time_to`, `presencial_location` desde el `PlanDay` correspondiente, y marca `source_plan_id` — no queda ligado a "vivir" del `TrainingPlan`. Por eso:

- Borrar el `TrainingPlan` origen (`DELETE /training-plans/{id}`) **no** rompe ni borra los días de calendario ya estampados — solo limpia `source_plan_id` a `null` en esas filas (`ClearSourcePlan`, corrido antes del delete físico del plan). El contenido copiado queda intacto.
- Editar el `TrainingPlan` (sus `PlanDay`) después de estampar **no** propaga nada a calendarios ya estampados — el stamp es una foto en un momento dado, no una referencia viva.
- Cada día `training` del stamp instancia la `Session` del `PlanDay` (igual que un `PUT` individual, §8) — `session_instance_id` apunta a la copia congelada, nunca al catálogo. Editar la `Session` de catálogo después del stamp no afecta ningún día ya estampado.

## 8. Instanciación por asignación — copias congeladas, sin clonado reactivo

Mecanismo central del dominio calendario (spec `asignacion-por-instanciacion`). Resuelve: *si edito una `Session` o `Exercise` del catálogo que ya está en el calendario de varios grupos, ¿a quién le pega la edición?* — **a nadie asignado ya**. `Exercise`/`Session`/`TrainingPlan` son 100% template; asignar contenido a un día crea copias inmutables propias, separadas del catálogo.

### 8.1 Tablas de instancia

| Tabla | Copia de | Columnas |
|---|---|---|
| `session_instances` | `Session` | `id`, `name`, `description`, `created_at` |
| `exercise_instances` | `Exercise` | `id`, `name`, `description`, `kind`, `intensity`, `minutes`, `distance_m`, `speed_kph`, `muscle_group`, `video_url`, `created_at` |
| `session_exercise_instances` | `SessionExercise` | `id`, `session_instance_id`, `exercise_instance_id`, `role` (`warmup`/`main`/`cooldown`), `repeat_count` (default 1), `rest_minutes` (default 0) |

Sin `owner_id` (no son catálogo de nadie), sin `deleted_at` ni `updated_at`: una instancia nunca se edita después de creada. El borrado es **físico**, no lógico (§8.4). Están separadas de las tablas de catálogo (no `is_instance` en las mismas tablas): una instancia no puede aparecer por error en `GET /sessions?owner_id=...`. No hay endpoint público que resuelva una instancia por ID — solo se llega a ellas vía calendario.

### 8.2 Cuándo se crea la instancia (en el `save`, no después)

Al escribir `kind=training` sobre un día (vía `PUT` individual, `bulk`, o `stamp` de un plan), el service (`instantiateSession` en `calendar_service.go`) copia en el momento, dentro de la transacción:

1. `SessionInstance` (copia `name`/`description` de la `Session` de catálogo).
2. Por cada `SessionExercise` de la sesión: una `ExerciseInstance` (copia completa del `Exercise`) + una `SessionExerciseInstance` que vincula ambos y guarda `role`/`repeat_count`/`rest_minutes`.
3. `GroupCalendarDay.session_instance_id` apunta a la instancia creada.

Sin deduplicación: asignar la misma `Session` a 10 días crea 10 instancias independientes (y sus ejercicios, también sin compartir). Edición posterior de la `Session`/`Exercise` de catálogo (`PUT /sessions/{id}`, `PUT /exercises/{id}`) **no toca** ninguna instancia ya creada — no hay divergencia, porque nunca hubo referencia viva.

El request sigue usando `session_id` de catálogo (y sigue validando que exista, `422` si no); lo que persiste en el día es `session_instance_id`.

**`session_id` es opcional en `PUT`/`Bulk` `kind=training` sobre un día que ya tiene instancia** (change `instancia-referencia-catalogo`): sin `session_id` y con instancia previa, el día **conserva su instancia tal cual** — no se reinstancia ni se borra nada (mismo mecanismo que `cancelled`, §8.3). Sin `session_id` y sin instancia previa, `422` (`ErrCalendarFieldMismatch` individual; en lote `ErrCalendarTrainingWithoutInstance` listando las fechas sin instancia, all-or-nothing). `Stamp` no admite omisión: cada `PlanDay` de entrenamiento referencia el catálogo por definición.

Cada instancia guarda además su **origen de catálogo**: `session_instances.source_session_id` y `exercise_instances.source_exercise_id` — referencias opacas app-managed (sin FK, mismo patrón que `source_plan_id`), puramente informativas: se pueblan al instanciar y **no se limpian** si la `Session`/`Exercise` de catálogo se soft-borra después (la fila de catálogo nunca desaparece físicamente, solo deja de listarse). Instancias anteriores al change tienen `NULL` y las respuestas D9 exponen ambos campos (`session_id`/`exercise_id`) como `null` — ver §8.5.

### 8.3 Guard de día cerrado — restricción de escritura, no de edición de catálogo

La regla de cuándo un día está "cerrado" (calculada al vuelo contra `time.Now()`, sin cron ni columna persistida) es la misma que la del change original:

| Condición de la fecha del día | ¿Cerrado? |
|---|---|
| `date < hoy` | Sí, siempre |
| `date > hoy` | No, nunca |
| `date == hoy` y `is_presencial = true` | Sí **a partir de** `presencial_time_from` de ese día |
| `date == hoy` y `is_presencial = false` (async) | Sí, siempre — no hay forma de saber si ya la arrancaron |

Pero su rol cambió: de "trigger de clonado" (mecanismo anterior, eliminado) pasó a **guard de escritura**. Si el día ya existe y está cerrado, no se puede asignar, reasignar ni borrar su contenido — ni siquiera reasignarlo con el mismo contenido. Aplica a los 5 endpoints de escritura:

- `PUT`/`Bulk`/`Stamp`: rechaza crear o reemplazar contenido sobre un día cerrado (`422 ErrCalendarDayClosed`, `{"message": "el día de calendario está cerrado: <fecha>"}`).
- `DeleteDay`/`BulkClear`: mismo guard, por simetría — no se puede "borrar la historia" tampoco.
- `Shift`: si alguna fila afectada por el corrimiento (`date >= from_date`) ya está cerrada, se bloquea el corrimiento completo.

En operaciones batch (`Stamp`/`Bulk`/`BulkClear`/`Shift`) el guard es **todo-o-nada**: se validan todas las fechas afectadas antes de escribir cualquiera y si alguna está cerrada se rechaza el lote entero con la lista de fechas en conflicto (`el día de calendario está cerrado: 2026-09-19, 2026-09-20`), sin cambios parciales.

**Única excepción: `cancelled`.** La transición a `kind=cancelled` (desde `training`) sigue permitida sobre un día cerrado — cancelar no reinstancia nada: marca `cancelled_reason` y conserva el `session_instance_id` existente como contexto. `is_presencial` participa solo en decidir si el día está cerrado; una vez cerrado, la única operación permitida es `cancelled`, sin excepción.

### 8.4 Reasignación y borrado de instancia superada

Reasignar un día **futuro** que ya tenía instancia sí está permitido (nunca puede ser cerrado, por §8.3). Dentro de la misma transacción (D10 del design):

1. Se crea la instancia nueva completa.
2. Se repuntea `session_instance_id` al nuevo.
3. Se chequea si algún `workout_feedback` (`assigned_session_id`/`assigned_exercise_id`) referencia la instancia vieja.
4. Sin feedback: se borra físicamente — `SessionExerciseInstance` primero, después sus `ExerciseInstance`, por último la `SessionInstance` vieja.
5. Con feedback: no se borra nada de la vieja — queda huérfana (ningún día activo la apunta, pero sigue resolviendo por ID desde el feedback histórico). No es un error, es el resultado esperado (mismo criterio que el soft-delete de catálogo: dejar de listarse, seguir resolviendo).

`DeleteDay`/`BulkClear` sobre un día futuro con instancia aplican el mismo borrado con feedback-check.

### 8.5 Respuesta de calendario — `session_instance` embebido

`CalendarDayResponse`/`NextSessionResponse` embeben el detalle completo de la instancia (D9), no un `session_id` bare:

```json
{
  "id": 1,
  "group_id": 2,
  "date": "2026-09-21",
  "kind": "training",
  "other_name": null,
  "session_instance": {
    "id": 123,
    "session_id": 77,
    "name": "Fartlek 5K",
    "description": null,
    "created_at": "2026-09-20T10:00:00Z",
    "exercises": [
      {"id": 456, "exercise_id": 501, "name": "Trote", "kind": "jogging", "description": null, "intensity": null,
       "minutes": 10, "distance_m": null, "speed_kph": null, "muscle_group": null, "video_url": null,
       "role": "warmup", "repeat_count": 1, "rest_minutes": 0}
    ]
  },
  "cancelled_reason": null,
  "is_presencial": false,
  "presencial_time_from": null, "presencial_time_to": null, "presencial_location": null,
  "source_plan_id": null, "created_at": "...", "updated_at": "..."
}
```

`session_instance` es `null` cuando el día no tiene instancia: `kind=rest`/`other` (o día `cancelled` sin sesión previa, caso que no debería darse dado que `cancelled` solo se alcanza desde `training`). Con `kind=cancelled` la instancia **sigue embebida** (`calendar_service.go` conserva el `session_instance_id` existente al cancelar): el alumno ve qué sesión era la que se canceló. Es el único detalle posible: no existe endpoint que resuelva una instancia por ID, y resolverla contra `GET /sessions/{id}` devolvería el catálogo en vivo, no lo congelado para ese día.

### 8.6 Lo que ya no existe (mecanismo anterior)

El mecanismo previo de **clonado por divergencia** (`calendario-asignacion-grupos` D8/D13 + `congelar-ejercicio-en-clon`) fue eliminado por completo, no convive con la instanciación. Ya no existen:

- `exclude_group_ids`/`clone_name`/`clone_description` en `PUT /sessions/{id}` (se ignoran silenciosamente si el frontend aún los manda) — la edición de catálogo jamás repuntea días.
- El auto-clonado de días ya cerrados al editar (`isCalendarDayClosed` como trigger). La función pasó a `calendar_service.go` con el mismo cálculo de fecha/horario pero como guard de escritura (§8.3).
- El congelamiento profundo en `ExerciseService.Update` (clonar sesiones/ejercicios en cascada) — sin objeto histórico que congelar, `ExerciseService.Update` volvió a ser una edición directa de catálogo.
- `GET /sessions/{id}/assigned-groups` — no cumplía ninguna función bajo el modelo nuevo (editar la sesión ya no afecta asignaciones). Eliminado sin reemplazo.
- Los DAOs `FindBySessionID`/`FindByExerciseID`/`RepointSessionForGroups`/`RepointDaysByID`.

Detalle del reemplazo y sus decisiones: `openspec/changes/asignacion-por-instanciacion/design.md`.

### 8.7 Colisión presencial en escrituras y banners del home

Change `colisiones-presenciales-y-calendario-agregado`. Resuelve: *si dos grupos del mismo entrenador tienen entrenamiento presencial superpuesto, ¿quién avisa?* La detección corre en el `save` de las 4 escrituras de calendario y alimenta banners de próxima sesión por rol.

#### Reglas de detección (D1/D3)

- **Solo participa `kind=training` con `is_presencial=true`, en ambos lados**: como escritura a validar y como colisionante. Un día `cancelled` (ni async ni `rest`/`other`) nunca bloquea ni genera warning — un cancelado no es un compromiso físico. Cambiar presencial a async, borrar o cancelar nunca dispara la detección.
- **Overlap medio-abierto, mismo día**: colisionan sii `fromA < toB && fromB < toA` — terminar 09:00 y arrancar 09:00 **no** colisiona.
- **Alcance**: todos los grupos activos de los equipos cuyo owner es el entrenador que escribe (`teams.owner_id`), cruzados contra lo que se va a escribir. Excluye el grupo escrito (PUT) y las filas movidas por su ID viejo (shift). Dentro de la transacción cuando la escritura es transaccional.
- **Clasificación por equipo del colisionante vs el del grupo escrito**: equipo distinto → **bloqueante** (409); mismo equipo → **warning** no bloqueante.

#### 409 de colisión y `same_team_warnings` (D4)

Cross-team rechaza la escritura (all-or-nothing en stamp/bulk/shift, con rollback completo). Body del 409 — estructura JSON dedicada, no el string plano del resto de los errores de calendario:

```json
HTTP 409
{
  "message": "colisión presencial con otro equipo",
  "conflicts": [
    {"group_id": 3, "group_name": "Maratón B", "team_id": 2, "team_name": "Equipo B",
     "date": "2026-10-01", "presencial_time_from": "09:00", "presencial_time_to": "10:00"}
  ]
}
```

`conflicts` lista **todos** los colisionantes (dedup por grupo/fecha/horario/clasificación). `force=true` del stamp **no** la bypasea.

Wiring por endpoint (D5):

| Escritura | Cuándo evalúa | Cross | Same |
|---|---|---|---|
| `PUT` individual | tras guards, si la fila resultante queda presencial | `409` sin escribir | guarda; `same_team_warnings` como campo extra del `CalendarDayResponse` (`omitempty`) |
| `stamp` | después del 409 de conflictos de fechas y del parse de `exclude_dates` | `409` all-or-nothing | warnings en el wrapper |
| `bulk` | en el loop de validación previa (junto con cerrado) | `409` lote completo rechazado | warnings agregados por cada fecha que superpone |
| `shift` | fechas nuevas de filas presenciales movidas (excluyendo las movidas por ID) | `409` rollback (filas mantienen fecha vieja) | warnings |

`same_team_warnings` solo se completa en la escritura individual (PUT). En lecturas y en el wrapper de stamp/bulk/shift queda vacío (no viaja en el JSON por `omitempty`).

#### Wrapper `CalendarMutationResponse` en stamp/bulk/shift (D4)

Reemplaza al array crudo de `CalendarDayResponse` (breaking coordinado con frontend):

```json
{ "days": [ { "id": 1, "group_id": 2, "…": "…" } ], "same_team_warnings": [ { "…": "…" } ] }
```

`days` siempre viaja; `same_team_warnings` solo cuando hay warnings. Código de éxito sin cambios: stamp `201`, bulk/shift `200`.

#### Banners del home (D6/D7)

**`GET /api/v1/users/{id}/next-session` — shape nuevo, in-place (BREAKING).** Antes: una sola sesión con `session_instance` embebida, `204` si no había. Ahora **siempre `200`**, con ambos próximos independientes y nullable:

```json
{
  "next_cancelled": {"group_id": 1, "group_name": "Maratón A", "date": "2026-09-25", "session_name": "Trote suave"},
  "next_training": {
    "group_id": 2, "group_name": "Fondo B", "date": "2026-09-26", "session_name": "Fartlek 5K",
    "is_presencial": true,
    "presencial_time_from": "18:00", "presencial_time_to": "19:00",
    "presencial_location": {"lat": -31.4, "lng": -64.2, "label": "Parque Sarmiento"}
  }
}
```

- `next_cancelled` es `NextSessionBannerItem` plano `{group_id, group_name, date, session_name}`; `next_training` agrega los campos presenciales (`is_presencial`, `presencial_time_from`/`to`, `presencial_location`) — un training async los deja `null`/`false`.
- Cada campo es el más próximo de su kind (`training`/`cancelled`) entre los grupos con membresía activa del usuario; sin próxima de un kind → `null` (nunca `204`).
- Filtro "hoy cuenta": `date > hoy` OR (`date == hoy` AND (no presencial OR `presencial_time_from > ahora`)) — hoy presencial ya arrancado no cuenta; hoy async sí. `session_name` sale de la instancia; si falta, `null` sin romper.

**`GET /api/v1/users/{id}/next-presencial-session` — nuevo, banner del entrenador (D7).** La próxima sesión `training`+`presencial` entre **todos** los grupos que administra el usuario (owner de sus equipos), la primera cronológicamente sin importar el equipo, con el mismo filtro de "hoy cuenta". `200` con `{group_id, group_name, team_id, team_name, date, session_name, presencial_time_from, presencial_time_to, presencial_location}` o `204` si no hay ninguna. Sin grupos administrados → `204` directo (no consulta el calendario).

### 8.8 Calendario agregado por rol (D8)

Dos lecturas nuevas con el mismo item `AggregateCalendarDayResponse`: los campos de `CalendarDayResponse` embebidos + `group_id`/`group_name`/`team_id`/`team_name` embebidos planos (resueltos server-side, batch de 1 query de groups + 1 de teams). El `group_id` propio sombra al del struct embebido en el JSON (regla de profundidad de `encoding/json` — los dos son el mismo valor).

- **`GET /api/v1/users/{id}/member-calendar?from=&to=`** — días de calendario de **todos los grupos con membresía activa** del usuario (membresías expiradas o soft-deleted fuera), merge ordenado por fecha. `200` con array (vacío si no hay). **Nunca trae `presencial_collision`.**
- **`GET /api/v1/users/{id}/administered-calendar?from=&to=`** — días de **todos los grupos administrados** por el usuario (owner de los equipos), mismo shape y orden. Cada día presencial que superpone con otro día presencial de otro grupo administrado trae:

```json
"presencial_collision": {
  "type": "cross_team",
  "conflicts": [
    {"group_id": 3, "group_name": "Maratón B", "team_id": 2, "team_name": "Equipo B",
     "date": "2026-10-01", "presencial_time_from": "09:00", "presencial_time_to": "10:00"}
  ]
}
```

  - `type`: `"cross_team"` si algún colisionante es de otro equipo (gana sobre `"same_team"` si hay de ambos), `"same_team"` si todos lo son. `conflicts` lista **todos** los colisionantes. Ausente (`omitempty`) si el día no colisiona.
  - Aplica las mismas reglas de §8.7 (solo training+presencial, overlap medio-abierto, cancelled fuera) y **detecta colisiones viejas**: días guardados antes de que existiera el guard aparecen marcados, porque la detección corre sobre los datos actuales. No las arregla — solo las hace visibles (arreglo manual: reprogramar o cancelar uno de los dos).
  - El día colisionante también aparece marcado en el item del otro grupo (la detección corre por día presencial, excluyendo la fila misma).

## 9. Detalles de implementación relevantes

- **Timezone:** todo cálculo de "hoy"/"ahora" en este dominio usa `time.Date(now.Year(), now.Month(), now.Day(), 0,0,0,0, now.Location())` para obtener medianoche **local**, nunca `time.Now().Truncate(24*time.Hour)` (eso trunca a medianoche UTC, incorrecto en `America/Argentina/Cordoba`, UTC-3). Si se agrega lógica nueva de fechas en este dominio, replicar ese patrón.
- **`default_time_from`/`default_time_to`/`presencial_time_from`/`presencial_time_to` viajan como string `"HH:MM"`** en JSON, se parsean con `time.Parse("15:04", ...)` al guardar (queda tagueado UTC, fecha `0000-01-01`) — la columna Postgres real es `timestamptz`, no `time` (GORM no aplica `type:time` para este dialecto, se mantiene timestamptz a propósito, ver el punto siguiente). `to` debe ser posterior a `from` (`422` si no).
- **Bug real encontrado y corregido (`fix/presencial-time-from-to`, 2026-09-19): siempre leer estos campos con `.UTC()` antes de `.Hour()/.Minute()/.Format(...)`, nunca confiar en `.Location()` del valor recién leído de la DB.** Postgres devuelve `timestamptz` convertido al `TimeZone` de la sesión, y el driver lo escanea en `time.Local` — como estos valores siempre se escriben en UTC (por el default de `time.Parse`), leerlos sin normalizar corrompe la hora por el offset de `time.Local` en cada round-trip (en `America/Argentina/Cordoba` esto rompía silenciosamente el corte del guard de cierre dependiendo de la hora del día en que corriera el proceso). Los 4 puntos de lectura (`toCalendarDayResponse`, `toPlanDayResponse`, `isCalendarDayClosed`, `NextSession`) ya aplican `.UTC()` — replicar el patrón en cualquier lectura nueva de estos campos.
- **`Location{lat,lng,label?}`** (paquete `trainingplan`) es compartida entre `PlanDay.default_location` y `GroupCalendarDay.presencial_location` — se persiste como `jsonb` vía marshal/unmarshal manual en el service, no hay tipo custom de GORM.
- **Transacciones:** siempre se instancian DAOs *nuevos* atados al `*gorm.DB` de la transacción (`daos.NewXDao(tx)`) — nunca se pasa `tx` a una instancia de DAO ya creada, este patrón de DAO no lo soporta.
- **`Stamp`/`Bulk`/`Shift`** escriben múltiples filas dentro de una única transacción — si falla la fila N, ninguna de las N-1 anteriores queda aplicada.

## 10. Tabla de errores por dominio

### Exercise (`mapExerciseError`)
| Error | HTTP |
|---|---|
| no encontrado | 404 |
| kind inválido | 400 |
| intensity inválido | 400 |
| muscle_group inválido | 400 |
| no autorizado | 403 |

### Session (`mapSessionError`)
| Error | HTTP |
|---|---|
| sesión no encontrada | 404 |
| falta ejercicio de algún rol | 422 |
| role inválido | 422 |
| ejercicio referenciado no encontrado | 422 |
| no autorizado | 403 |

### TrainingPlan (`mapTrainingPlanError`)
| Error | HTTP |
|---|---|
| plan no encontrado | 404 |
| cantidad de días fuera de 2..31 | 422 |
| sequence_no inválido | 422 |
| kind de día inválido | 422 |
| combinación de campos inválida | 422 |
| session_id referenciado no encontrado | 422 |
| default_time_from/default_time_to formato inválido | 422 |
| default_time_to no posterior a default_time_from | 422 |
| no autorizado | 403 |

### Calendar (`mapCalendarError`)
| Error | HTTP |
|---|---|
| grupo no encontrado | 404 |
| plan no encontrado | 404 |
| no autorizado | 403 |
| plan no pertenece al dueño del grupo | 403 |
| kind inválido | 422 |
| combinación de campos inválida | 422 |
| transición de cancelación inválida | 422 |
| sesión referenciada no encontrada | 422 |
| día cerrado (asignar/reasignar/borrar/correr contenido) | 422 — en lote, el message lista las fechas en conflicto |
| training sin `session_id` y sin instancia previa que conservar | 422 (individual: combinación de campos; lote: el message lista las fechas sin instancia, all-or-nothing) |
| presencial_time_from/presencial_time_to formato inválido | 422 |
| presencial_time_to no posterior a presencial_time_from | 422 |
| conflicto de fechas en stamp | 409 |
| colisión de fechas en shift | 409 |
| colisión presencial con otro equipo (cross-team) | 409 — body `{message, conflicts}` (§8.7), no el string plano; en stamp `force` no la bypasea |
| no podés consultar datos de otro usuario | 403 (chequeado en el controller, no en el service) |
