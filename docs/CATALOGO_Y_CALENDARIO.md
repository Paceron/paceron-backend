# Catálogo de entrenamiento y calendario de grupos

Documento de referencia completo del dominio **catálogo** (`Exercise`, `Session`, `TrainingPlan`) y **calendario** (`GroupCalendarDay`). Cubre modelo de datos, reglas de validación, guards de autorización, endpoints y la lógica especial de edición/clonado. Fuente de verdad: el código en `cmd/api/domains/dbs`, `cmd/api/services`, `cmd/api/controllers` — este doc lo resume y explica, no lo reemplaza.

Origen de las specs: `openspec/changes/catalogo-planes-entrenamiento/` y `openspec/changes/calendario-asignacion-grupos/` (`design.md`/`specs/*/spec.md` tienen el detalle de decisión con Given/When/Then; acá va la síntesis operativa).

## 1. Relación entre entidades

```
Exercise (catálogo, reusable) ──┐
                                 ├─< SessionExercise >── Session (catálogo, reusable)
                                 │        (rol: warmup/main/cooldown)
                                 │
Session ──< PlanDay >── TrainingPlan (template reusable, días 1..N)
                                 │
                                 │  POST /groups/{id}/calendar/stamp
                                 ▼
                        GroupCalendarDay (calendario REAL de un grupo,
                        una fila por (group_id, date) con contenido)
```

Puntos clave de esta jerarquía:

- **`Exercise` y `Session` son catálogo reusable de un entrenador (`owner_id`)**, no atado a ningún plan ni calendario. Un mismo `Exercise` puede estar en N `Session`; una misma `Session` puede estar en N `PlanDay` de N planes distintos, y estampada en N días de calendario de N grupos distintos.
- **`TrainingPlan` es un template** — sus `PlanDay` dicen "día 3 = training con `Session` X, presencial a las 18:00" pero no tiene fecha real. Estampar (`stamp`) es lo que traduce ese template a fechas concretas en `GroupCalendarDay`.
- **`GroupCalendarDay` es la única entidad con fecha real.** Es dispersa: si no hay fila para `(group_id, date)`, ese día está vacío. No repite datos por referencia únicamente — copia físicamente `kind`, `session_id`, `is_presencial`, `presencial_time_from`, `presencial_time_to`, `presencial_location` al momento del stamp (ver §7).

## 2. Dónde "viven" los ejercicios de una sesión (aclaración habitual)

Pregunta recurrente: *cuando guardo una sesión, ¿los ejercicios quedan embebidos ahí o siguen viviendo en el catálogo?*

- `Session` **no tiene un array embebido de ejercicios.** Es una tabla propia (`sessions`) sin columna de ejercicios.
- La relación vive en una tabla intermedia dedicada: **`session_exercises`** (`SessionExercise`), con `session_id`, `exercise_id`, `role`, `repeat_count`, `rest_minutes`. Es decir, cada combinación "este ejercicio va en esta sesión con este rol" es una fila propia — el `Exercise` original en `exercises` no se copia ni se mueve, solo se referencia por `exercise_id`.
- El **`PUT /sessions/{id}` reemplaza el set de `SessionExercise` completo**, no hace un patch fila por fila: borra todas las filas de esa sesión y crea exactamente las del array recibido, en el orden dado (`SessionExerciseDao.ReplaceForSession`, transaccional). Si mandás una lista distinta, las filas viejas desaparecen y aparecen las nuevas — pero el `Exercise` catalogado (`exercises`) nunca se toca por este PUT.
- El borrado de un `Exercise` es **lógico** (`deleted_at`), justamente para que una `Session` que ya lo referencia (vía `SessionExercise.exercise_id`) siga resolviendo el detalle completo en `GET /sessions/{id}` aunque ese ejercicio ya no aparezca en `GET /exercises?owner_id=...` (dejó de listarse para catálogo nuevo, pero no rompe lo ya armado).
- Un `Exercise` **no exige combinación de campos por `kind`** (`kind`, `intensity`, `muscle_group`, `minutes`, `distance_m`, `speed_kph` son todos independientes entre sí, todos opcionales salvo `kind`) — sí se valida que cada valor (si viene) pertenezca a su enum.
- Al **clonar una sesión** (`POST /sessions/{id}/clone`) se hace copia profunda de sus `SessionExercise` (mismos `exercise_id`, mismo `role`/`repeat_count`/`rest_minutes`) apuntando al `Session` nuevo — los `Exercise` referenciados siguen siendo los mismos, no se clonan.

En síntesis: **el ejercicio vive una sola vez en `exercises`; cada sesión que lo usa agrega una fila en `session_exercises` que apunta a él por ID.** Editar la sesión nunca edita el ejercicio catalogado, y borrar/editar el ejercicio catalogado no rompe sesiones que ya lo referencian (salvo que sea un borrado físico, que no existe para `Exercise`).

**Editar un `Exercise` que ya está en un día de calendario cerrado no pisa el historial** — mismo congelamiento por divergencia que tiene editar una `Session` (§8), extendido un nivel más profundo (§8.5). Antes de esa extensión, este era un gap real: editar el ejercicio (ej. cambiar `distance_m` de 100 a 50) pegaba en vivo incluso sobre días ya pasados, invalidando cualquier feedback ya cargado sobre esa actividad.

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
| `other_name`, `session_id`, `cancelled_reason` | según `kind` |
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
| `NextSession`/`CalendarSummary` (`/users/{id}/...`) | `userID == callerID` estricto — nadie puede consultar el calendario de otro usuario, ni el propio entrenador (`403 no podés consultar los datos de otro usuario`) |
| `AssignedGroups` (`/sessions/{id}/assigned-groups`) | sin guard — cualquier autenticado puede pedirlo (uso interno pensado para que el frontend muestre "esta sesión está asignada en N grupos" antes de confirmar una edición) |

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
| GET | `/api/v1/sessions/{id}/assigned-groups` | — | `200` array `{group_id,group_name}` | `400` |

`SessionRequest`: `owner_id*`, `name*`, `description`, `exercises*` (array de `{exercise_id*, role*, repeat_count?, rest_minutes?}`, mínimo 1 de cada rol). En `PUT` además (opcionales, ver §8): `exclude_group_ids`, `clone_name`, `clone_description`.

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
| PUT | `/api/v1/groups/{id}/calendar/{date}` | `CalendarDayRequest` | `200` (upsert) | `400`, `403`, `422` |
| DELETE | `/api/v1/groups/{id}/calendar/{date}` | — | `204` | `400`, `403` |
| POST | `/api/v1/groups/{id}/calendar/stamp` | `StampRequest{plan_id*, start_date*, force?}` | `201` array | `400`, `403`, `404` (plan), `409` (conflicto) |
| POST | `/api/v1/groups/{id}/calendar/bulk` | `BulkRequest{dates*, kind*, session_id?, other_name?, is_presencial?, presencial_time_from?, presencial_time_to?, presencial_location?}` | `200` array | `400`, `403`, `422` |
| POST | `/api/v1/groups/{id}/calendar/bulk-clear` | `BulkClearRequest{dates*}` | `204` | `400`, `403` |
| POST | `/api/v1/groups/{id}/calendar/shift` | `ShiftRequest{from_date*, days*}` | `200` array | `400`, `403`, `409` (colisión) |
| GET | `/api/v1/users/{id}/next-session` | — | `200` / `204` sin próxima | `400`, `403` |
| GET | `/api/v1/users/{id}/calendar-summary` | — | `200` array `{group_id,group_name}` | `400`, `403` |

`Stamp`: copia cada `PlanDay` del plan a `start_date + (sequence_no - 1)` días, marcando `source_plan_id`. Sin `force=true`, si alguna fecha destino ya tiene contenido → `409 hay fechas con contenido existente` y no escribe nada.

`Shift`: mueve todas las filas desde `from_date` en adelante, `days` posiciones (entero positivo). Antes de escribir valida que ninguna fecha destino choque con una fila **anterior a `from_date`** que quede fuera del rango desplazado (`409 el corrimiento haría chocar dos fechas`).

`NextSession`: busca, entre los grupos donde el usuario es miembro, la próxima fila con `kind IN (training, cancelled)` y `date >= hoy` (hoy = medianoche local, no UTC — ver nota de timezone en §9). `204` si no hay ninguna.

## 7. Stamp — copiado físico, no por referencia

Al estampar (`Stamp`) un plan, cada `GroupCalendarDay` generado copia **por valor** `kind`, `other_name`, `session_id`, `is_presencial`, `presencial_time_from`, `presencial_time_to`, `presencial_location` desde el `PlanDay` correspondiente — no queda ligado a "vivir" del `TrainingPlan`. Por eso:

- Borrar el `TrainingPlan` origen (`DELETE /training-plans/{id}`) **no** rompe ni borra los días de calendario ya estampados — solo limpia `source_plan_id` a `null` en esas filas (`ClearSourcePlan`, corrido antes del delete físico del plan). El contenido copiado queda intacto.
- Editar el `TrainingPlan` (sus `PlanDay`) después de estampar **no** propaga nada a calendarios ya estampados — el stamp es una foto en un momento dado, no una referencia viva.
- Lo que sí sigue siendo una referencia viva (no copia) es `GroupCalendarDay.session_id` → apunta al `Session` real, y por eso editar esa `Session` sí puede impactar en calendario ya estampado (ver §8).

## 8. Edición de una `Session` ya asignada en calendario — clon por divergencia

Esta es la pieza más elaborada del dominio (D8 + D13 en `calendario-asignacion-grupos/design.md`). Resuelve: *si edito una `Session` que ya está en el calendario de varios grupos, ¿a quién le pega la edición?*

**Regla base:** editar una `Session` (`PUT /sessions/{id}`) propaga en vivo a **todo** lo que la referencia por `session_id` — todos los `GroupCalendarDay.session_id` iguales a esa sesión ven el cambio inmediatamente (porque `GroupCalendarDay` no copia el contenido de la sesión, solo el ID).

Dos mecanismos permiten que ciertos días **no** reciban esa edición y en cambio queden apuntando a un clon congelado con el contenido *anterior*:

### 8.1 Exclusión manual por grupo (`exclude_group_ids`)

El body de `PUT /sessions/{id}` acepta `exclude_group_ids: number[]` opcional. Cualquier grupo listado ahí **no** recibe la edición: todas sus filas de calendario que apuntaban a la sesión original se repuntean (`session_id`) a un clon nuevo con el contenido *previo a la edición*. `clone_name`/`clone_description` opcionales para nombrar ese clon (si no, `"<nombre original> (copia)"`).

### 8.2 Congelamiento automático de días ya cerrados (D13, sin cron)

Independiente de `exclude_group_ids`, **cada día individual** (no grupo entero) que ya está "cerrado" al momento del `PUT` se auto-excluye de la edición, sin que el entrenador tenga que pedirlo. Un día se considera cerrado según esta regla, calculada al vuelo contra `time.Now()` en cada request (sin cron, sin columna persistida):

| Condición de la fecha del día | ¿Cerrado? |
|---|---|
| `date < hoy` | Sí, siempre |
| `date > hoy` | No, nunca |
| `date == hoy` y `is_presencial = true` | Sí **a partir de** `presencial_time_from` de ese día (antes, sigue abierto — el entrenador puede seguir ajustando la sesión de hoy antes de que empiece) |
| `date == hoy` y `is_presencial = false` (async) | Sí, siempre — los corredores pueden hacerla en distintos momentos del mismo día, no hay forma de saber si ya la arrancaron |

**Por qué existe:** preservar fidelidad histórica de lo que realmente estuvo programado/hecho en cada día, de cara a una futura funcionalidad de "registrar qué se hizo realmente" (por el entrenador a posteriori, o por el corredor en tiempo real) — **esa funcionalidad de logging no está implementada todavía**, esto solo prepara el terreno para que editar una sesión no pise el historial.

### 8.3 Cómo interactúan ambos mecanismos

- Ambos casos (exclusión manual por grupo + auto-cierre por día) usan **el mismo clon único** por operación de `PUT` — no se crea un clon por grupo y otro por día cerrado, todo el contenido "que no debe cambiar" en esta edición va al mismo clon.
- El repunteo es de **granularidad distinta** a propósito:
  - Exclusión manual → `RepointSessionForGroups(groupIDs, oldSessionID, newSessionID)`: repuntea **todas** las filas de esos grupos que usaban la sesión (group-wide).
  - Auto-cierre → `RepointDaysByID(dayIDs, newSessionID)`: repuntea **filas puntuales por ID**, no por grupo — porque un mismo grupo puede tener la misma sesión en varias fechas con distinto estado (una ya pasó, otra es futura), y un repunteo group-wide movería incorrectamente también la fecha futura que debía seguir recibiendo la edición en vivo.
- Si ni `exclude_group_ids` trae nada ni hay ningún día cerrado → no se crea ningún clon, el `Update` es directo (camino rápido, sin transacción).
- Si hay algo de cualquiera de los dos tipos → todo corre dentro de una transacción (`s.db.Transaction`) que: crea el clon, repuntea lo que corresponda de cada tipo, y recién ahí aplica la edición a la sesión original.

### 8.4 Ejemplo concreto

Sesión "Fartlek 5K" asignada en 3 grupos: A (hoy, presencial 18:00, son las 15:00), B (mañana), C (ayer). El entrenador edita la sesión sin marcar `exclude_group_ids`:

- Grupo A: `date == hoy`, presencial, `now < presencial_time_from` → **no** cerrado → recibe la edición en vivo.
- Grupo B: `date > hoy` → **no** cerrado → recibe la edición en vivo.
- Grupo C: `date < hoy` → **cerrado** → se auto-clona, esa fila queda apuntando al clon con el contenido viejo.

Si en cambio el entrenador edita a las 19:00 (después de las 18:00 de hoy), el Grupo A también queda cerrado y se clona junto con C.

### 8.5 Congelamiento profundo: también protege contra editar el `Exercise` (`congelar-ejercicio-en-clon`)

Gap detectado post-D13 (spec propia: `openspec/changes/congelar-ejercicio-en-clon/`): clonar la `Session` no alcanza si el clon sigue apuntando a los mismos `Exercise` originales por `exercise_id` — editar el `Exercise` directamente (`PUT /exercises/{id}`) pegaba igual sobre días ya congelados, porque el contenido real (`distance_m`, `minutes`, etc.) nunca vivía copiado en ningún lado, solo la referencia.

Fix: el mismo clon de sesión ahora también **clona cada `Exercise`** que la sesión referencia (todos, no solo el editado — si se clonara solo uno, los demás seguirían vivos y reabrirían el mismo bug), y apunta el `SessionExercise` clonado al `Exercise` clonado en vez del original. Esto corre en dos disparadores:

- **`SessionService.Update`** (D8 manual + D13 automático): sin cambios de comportamiento visible, solo el clon ahora es más profundo.
- **`ExerciseService.Update`** (nuevo): mismo chequeo de días cerrados que D13, pero yendo un salto más — `exercise_id` → `session_exercises` (qué sesiones lo usan) → `group_calendar_days` (qué días usan esas sesiones) → filtrar cerrados → agrupar por sesión → clonar cada sesión afectada (con todos sus ejercicios) → repuntear solo esos día IDs. Sesiones del mismo ejercicio sin ningún día cerrado no se tocan.

El clonado manual explícito (`POST /sessions/{id}/clone` y `POST /exercises/{id}/clone`, "duplicar para editar") **no** cambia — sigue compartiendo `exercise_id`/catálogo con el original a propósito, no es congelamiento histórico.

Sin cambio de schema ni backfill: no había datos reales dependiendo de la protección faltante (gap detectado antes de que la feature de feedback tuviera uso real).

## 9. Detalles de implementación relevantes

- **Timezone:** todo cálculo de "hoy"/"ahora" en este dominio usa `time.Date(now.Year(), now.Month(), now.Day(), 0,0,0,0, now.Location())` para obtener medianoche **local**, nunca `time.Now().Truncate(24*time.Hour)` (eso trunca a medianoche UTC, incorrecto en `America/Argentina/Cordoba`, UTC-3). Si se agrega lógica nueva de fechas en este dominio, replicar ese patrón.
- **`default_time_from`/`default_time_to`/`presencial_time_from`/`presencial_time_to` viajan como string `"HH:MM"`** en JSON, se parsean con `time.Parse("15:04", ...)` al guardar (queda tagueado UTC, fecha `0000-01-01`) — la columna Postgres real es `timestamptz`, no `time` (GORM no aplica `type:time` para este dialecto, se mantiene timestamptz a propósito, ver el punto siguiente). `to` debe ser posterior a `from` (`422` si no).
- **Bug real encontrado y corregido (`fix/presencial-time-from-to`, 2026-09-19): siempre leer estos campos con `.UTC()` antes de `.Hour()/.Minute()/.Format(...)`, nunca confiar en `.Location()` del valor recién leído de la DB.** Postgres devuelve `timestamptz` convertido al `TimeZone` de la sesión, y el driver lo escanea en `time.Local` — como estos valores siempre se escriben en UTC (por el default de `time.Parse`), leerlos sin normalizar corrompe la hora por el offset de `time.Local` en cada round-trip (en `America/Argentina/Cordoba` esto rompía silenciosamente el corte de D13 dependiendo de la hora del día en que corriera el proceso). Los 4 puntos de lectura (`toCalendarDayResponse`, `toPlanDayResponse`, `isCalendarDayClosed`, `NextSession`) ya aplican `.UTC()` — replicar el patrón en cualquier lectura nueva de estos campos.
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
| presencial_time_from/presencial_time_to formato inválido | 422 |
| presencial_time_to no posterior a presencial_time_from | 422 |
| conflicto de fechas en stamp | 409 |
| colisión de fechas en shift | 409 |
| no podés consultar datos de otro usuario | 403 (chequeado en el controller, no en el service) |
