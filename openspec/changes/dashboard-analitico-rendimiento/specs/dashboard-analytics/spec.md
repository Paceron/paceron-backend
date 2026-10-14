# Spec Delta

## Purpose

Analítica de rendimiento de entrenamiento: convierte las series registradas en `workout_feedback` en métricas agregadas por corredor, grupo y tipo de ejercicio, con autorización estricta por scope y sin exponer datos fuera del scope del solicitante.

## ADDED Requirements

### Requirement: Los filtros del dashboard se devuelven recortados por el scope del solicitante

El sistema SHALL exponer `GET /api/v1/dashboard/filters` que devuelve, **ya filtrados por el scope del solicitante**: sus equipos, sus grupos, sus corredores, el catálogo de tipos de ejercicio (`liviano`, `elongacion`, `ritmo_continuo`, `velocidad_tiempo`, `velocidad_distancia`), sus ejercicios con su objetivo vigente, y el rango de fechas **real** con datos (`MIN`/`MAX` de `session_date` dentro del scope).

El sistema SHALL entregar `date_range` derivado de los datos existentes, no como un rango fijo ni como un rango por defecto: ofrecer una ventana sin datos fuerza al usuario a descubrir el vacío por sí mismo.

#### Scenario: El entrenador recibe solo su scope

- **WHEN** un entrenador dueño de los equipos 29 y 30 pide `GET /dashboard/filters`
- **THEN** la respuesta contiene únicamente esos dos equipos y sus grupos, y ningún corredor ajeno

#### Scenario: El corredor no ve otros corredores en los filtros

- **WHEN** un corredor miembro de un grupo pide `GET /dashboard/filters`
- **THEN** la lista de corredores contiene solo su propia entrada, aunque comparta grupo con otros corredores

#### Scenario: Sin datos, el rango de fechas refleja la ausencia

- **WHEN** el scope no tiene ninguna serie registrada
- **THEN** `date_range` viene `null` y la lista de ejercicios y corredores viene vacía

### Requirement: El scope entra por el equipo del que se llega, y el rol decide qué hay

El `team_id` SHALL ser **obligatorio** en todos los endpoints del dashboard, y se toma del equipo desde el que el usuario entró. No es un filtro opcional ni se deduce del scope: el acceso es por pestaña del detalle de equipo, así que el equipo siempre se conoce.

Si el `team_id` no pertenece al scope del solicitante, el sistema SHALL responder `403`.

Las tres endpoints de comparación SHALL rechazarse con `403` cuando el llamador **no sea dueño del equipo**:

| Endpoint | Entrenador (dueño) | Corredor (miembro) |
|---|---|---|
| `/leaderboard` | `200` | **`403`** |
| `/comparisons` | `200` | **`403`** |
| `/comparisons/periods` | `200` | **`403`** |
| el resto | `200` | `200` |

La regla va **en el backend y no solo en la UI** a propósito: si el corredor no debe ver las métricas de sus compañeros, dejarlo的 en un `if` del front es una decisión que se Erosiona con el tiempo — el próximo que agrega un botón lo rompe sin darse cuenta, y para entonces ya es un default. En el backend, agregar "ver el ranking del equipo" exige **revisar la decisión explícitamente**.

No es una frontera de privacidad: el roster del equipo **ya** es visible para los corredores (la pestaña `corredores` de `team-detail-screen.jsx` no está filtrada por rol). Es una decisión de producto —"el corredor no entra a la comparación con otros"— y por eso se implementa como una, sin disfrazarla de seguridad.

#### Scenario: Corredor pide el leaderboard

- **WHEN** un corredor miembro del equipo pide `/dashboard/leaderboard`
- **THEN** el sistema responde `403` y no ejecuta la consulta

#### Scenario: Entrenador pide su propio equipo

- **WHEN** el dueño del equipo pide `/dashboard/leaderboard`
- **THEN** el sistema responde `200` con los corredores de su scope

#### Scenario: Equipo ajeno

- **WHEN** cualquier llamador pide métricas con un `team_id` fuera de su scope
- **THEN** el sistema responde `403`, incluido el dueño de otro equipo

### Requirement: Los endpoints de métricas declaran sus filtros

Sin esto, un cliente no puede llamar a los endpoints: no hay forma de saber **qué ejercicio** se está midiendo, y sin eso `axis`, `has_target` y `best_time_ms` no significan nada. Un "ritmo promedio" sobre 100 m y 21 km en la misma fila es basura.

El filtro es **por ejercicio**, y hay exactamente tres formas de elegirlo:

| Forma | Parámetro | Cuándo |
|---|---|---|
| **Un ejercicio** | `exercise_id` (catalog) | es lo más común, y lo que emite el selector |
| **Varios ejercicios** | `exercise_ids[]` | comparar dos ejercicios entre sí |
| **Un tipo entero** | `kinds[]` | el tablist de tipos del front: el catálogo tiene 5 `ritmo_continuo` y 4 `velocidad_distancia`, así que el tipo no cabe en un `exercise_id` |

Exactamente uno de los tres SHALL venir, y el sistema SHALL rechazar con `400` los casos inválidos: ninguno, más de uno, un `exercise_id` inexistente, o un `kind` fuera de los cinco vigentes.

**`kinds[]` agrega los ejercicios de esos tipos ya resueltos en el backend.** El front tiene prohibido agregar (non-goal de no recalcular KPIs), así que un `ritmo_continuo` de 4 km y uno de 21 km llegan en la misma fila con sus KPIs ya agregados, y el backend declara cómo: **ponderación por volumen**, es decir `achieved_speed_kph` es la media de las velocidades **ponderada por distancia**, y `best_time_ms` es el mínimo entre ejercicios.

Ponderar por distancia y no hacer media simple no es cosmético: un 21 km a 5 km/h y un 4 km a 15 km/h promedian 10 km/h con media simple, cuando el volumen real está casi entero en el 21 km. El resultado que importa es el del volumen.

#### Scenario: Filtro por tipo

- **WHEN** se pide el resumen con `kinds=ritmo_continuo`
- **THEN** devuelve los KPIs agregados de los 5 `ritmo_continuo` del grupo, ponderados por distancia

#### Scenario: Filtro por un ejercicio

- **WHEN** se pide el resumen con `exercise_id=7`
- **THEN** devuelve los KPIs de ese ejercicio, sin agregar con ningún otro

#### Scenario: Filtro inexistente

- **WHEN** se pide el resumen con `exercise_id=999999`
- **THEN** el sistema responde `404`

#### Scenario: Dos filtros a la vez

- **WHEN** se pide el resumen con `exercise_id=7` y `kinds=ritmo_continuo`
- **THEN** el sistema responde `400`, porque son excluyentes

### Requirement: Cada endpoint declara su request completo

El sistema SHALL aceptar **exactamente** los parámetros listados en esta tabla: cualquier otro SHALL ser rechazado con `400`.

Los parámetros que no se listan aquí no se aceptan.

| Endpoint | Método | Parámetros |
|---|---|---|
| `/dashboard/filters` | GET | `team_id` **obligatorio** (el resto del scope sale del token) |
| `/dashboard/metrics/summary` | GET | `team_id` **obligatorio**, `exercise_id` o `exercise_ids[]` o `kinds[]`, `from` **obligatorio**, `to`, `tolerance` |
| `/dashboard/metrics/timeseries` | GET | los de summary, más `granularity` (default `session`) |
| `/dashboard/leaderboard` | GET | los de summary, más `page`, `page_size`, `sort` (default `best_time_ms`), `order` (default `asc`) |
| `/dashboard/exercises/{id}/series` | GET | `{id}` de catálogo, más `from`, `to`, `tolerance`, `page`, `page_size` |
| `/dashboard/comparisons` | POST | body `{ group_id, exercise_id o exercise_ids[] o kinds[], from, to, tolerance, athlete_user_ids[] }` |
| `/dashboard/comparisons/periods` | POST | body `{ group_id, exercise_id o exercise_ids[] o kinds[], current: { from, to }, tolerance, previous: { from, to } }` |

Tres defaults que antes no existían y que cambian lo que se ve: `granularity = session`, `sort = best_time_ms`, `order = asc`. El default de `sort` tiene que ser `best_time_ms` con orden ascendente porque con `mean_speed_kph` el ascendente pondría **primero al más lento**, que es lo contrario de un ranking.

`previous` es **opcional** en `comparisons/periods`: si no viene, el backend lo deriva desplazando `current` hacia atrás su misma cantidad de días.

**Todos los filtros de scope —`group_id`, `athlete_user_id`— son opcionales** y se resuelven del scope del solicitante cuando faltan. `group_id` explícito **no** amplía el scope: si no está en el scope del solicitante, responde `403`, no `404`.

#### Scenario: El previous se deriva

- **WHEN** se manda `current = {2026-09-01 → 2026-09-30}` sin `previous`
- **THEN** el backend deriva `previous = {2026-08-02 → 2026-08-31}`

#### Scenario: El default del leaderboard

- **WHEN** se pide el leaderboard sin `sort` ni `order`
- **THEN** ordena por `best_time_ms` ascendente, que pone primero al más rápido

### Requirement: El filtro declara su forma exacta

Los tipos de los ids SHALL declararse, porque el repo ya tiene un antecedente de que adivinarlos rompe en silencio: `utils/id-match.js` documenta el caso de un selector que muestra el placeholder **teniendo el valor elegido** por comparar `number` con `string` (`utils/id-match.js#isSameId`). Cualquier id que viaje de un select a un request SHALL pasar por esa comparación, y el tipo declarado evita el mismatch desde el origen.

`GET /dashboard/filters` SHALL devolver:

```jsonc
{
  "teams":   [ { "id": 29, "name": "Paceron Running" } ],
  "groups":  [ { "id": 39, "team_id": 29, "name": "Grupo A", "role": "owner" } ],
  "athletes":[ { "id": 118, "name": "Ana", "surname": "Gómez" } ],
  "exercise_types": ["liviano", "elongacion", "ritmo_continuo",
                     "velocidad_tiempo", "velocidad_distancia"],
  "exercises": [ { "id": 7, "name": "Distancia 4 km", "kind": "ritmo_continuo",
                   "current_target_minutes": 30,
                   "current_target_distance_m": 4000 } ],
  "date_range": { "min": "2026-09-01", "max": "2026-10-10" }
}
```

- `id` de equipo, grupo, corredor y ejercicio es **número entero** en todas las plataformas. Un id que sale de acá y entra en un body de request SHALL convertirse con `Number(...)`: el backend rechaza bodies con ids no numéricos, y ese rechazo es invisible contra mocks.
- `date_range` SHALL venir `null` cuando el scope no tiene ninguna sesión con `workout_feedback`, no `{min: null, max: null}`: el front usa ese `null` para distinguir "nunca hubo datos" de "hay datos acotados", y decidir entre el estado vacío y el primer uso.
- `exercises[]` SHALL traer **el objetivo vigente del catálogo** (`current_target_*`), explícitamente rotulado así. No es el objetivo con el que se evalúa una sesión histórica: ese va congelado en la `ExerciseInstance` y solo aparece en `exercises/{id}/series`.

#### Scenario: Scope sin datos

- **WHEN** el corredor todavía no tiene ninguna sesión
- **THEN** `exercises` viene `[]` y `date_range` viene `null`

### Requirement: El scope se aplica en la consulta, no sobre el resultado

El sistema SHALL resolver el scope del solicitante una vez por request y aplicarlo **dentro de la cláusula `WHERE`** de cada consulta de métricas. El sistema SHALL **NOT** traer filas y descartarlas después en memoria.

Cuando un `group_id`, `athlete_user_id` o `exercise_id` recibido no pertenece al scope del solicitante, el sistema SHALL responder `403`. Reservar `404` para recursos que no existen.

`POST /dashboard/comparisons` SHALL validar **cada** `athlete_user_id` individualmente y, si alguno está fuera de scope, responder `403` identificando cuál falló, sin devolver datos de los que sí están en scope.

#### Scenario: Un entrenador pide métricas de un grupo ajeno

- **WHEN** el entrenador del equipo 29 pide `GET /dashboard/metrics/summary?group_id=<grupo del equipo 30>`
- **THEN** el sistema responde `403` y no ejecuta la consulta de agregados

#### Scenario: Comparación con un corredor fuera de scope

- **WHEN** se envía `POST /dashboard/comparisons` con tres `athlete_user_id`, uno de ellos fuera del scope
- **THEN** el sistema responde `403` nombrando ese `athlete_user_id`, y no devuelve los datos de los otros dos

#### Scenario: Recurso inexistente versus recurso ajeno

- **WHEN** se pide un `group_id` que no existe en el sistema
- **THEN** el sistema responde `404`; si el grupo existe pero pertenece a otro scope, responde `403`

### Requirement: El eje del objetivo lo declara el tipo de ejercicio

El sistema SHALL determinar el eje del objetivo a partir del `kind` del ejercicio, sin campos adicionales:

| `kind` | eje del objetivo | velocidad |
|---|---|---|
| `liviano` | distancia **o** tiempo, el que esté cargado | derivada si hay ambos |
| `elongacion` | tiempo | no aplica (no hay distancia) |
| `ritmo_continuo` | distancia **y** tiempo | **derivada** |
| `velocidad_tiempo` | tiempo | derivada si hay distancia |
| `velocidad_distancia` | distancia | derivada si hay tiempo |

El objetivo de una serie SHALL ser el valor congelado en la `ExerciseInstance` de esa sesión, nunca el valor vigente del catálogo: cada día se juzga contra lo que el corredor intentaba ese día, no contra un objetivo redefinido después.

El campo `speed_kph` SHALL **NOT** usarse como eje del objetivo en ningún tipo: se deriva siempre de distancia y tiempo.

#### Scenario: Ejercicio de ritmo continuo con ambos datos

- **WHEN** se consulta una `ExerciseInstance` con `kind=ritmo_continuo`, `distance_m=1000` y `minutes=4`
- **THEN** el eje es distancia y tiempo, y la velocidad esperada es 15 km/h

#### Scenario: El objetivo se congela al estampar

- **WHEN** un entrenador cambia el `minutes` de un ejercicio del catálogo **después** de que una sesión con ese ejercicio ya fue estampada
- **THEN** el cumplimiento de esa sesión ya estampada se sigue midiendo contra el `minutes` original de su `ExerciseInstance`

#### Scenario: Ejercicio de velocidad en tiempo sin distancia

- **WHEN** se consulta una `ExerciseInstance` con `kind=velocidad_tiempo`, `minutes=2` y `distance_m` vacío
- **THEN** el eje es tiempo y no hay velocidad esperada

### Requirement: El tiempo logrado prioriza el tiempo activo

El sistema SHALL calcular el tiempo de una serie como `active_duration_ms` cuando esté presente, y usar `duration_ms` como sustituto cuando no lo esté, marcando en ese caso la serie con calidad de tiempo `degraded`.

El sistema SHALL exponer el conteo de series degradadas dentro del indicador de calidad de datos.

#### Scenario: Serie con tiempo activo

- **WHEN** la serie tiene `active_duration_ms = 230000` y `duration_ms = 250000`
- **THEN** el tiempo logrado es 230000 ms y la calidad es `alta`

#### Scenario: Serie sin tiempo activo

- **WHEN** la serie tiene `active_duration_ms` en `null` y `duration_ms = 250000`
- **THEN** el tiempo logrado es 250000 ms y la calidad de tiempo es `degraded`

### Requirement: El cumplimiento del objetivo usa una tolerancia configurable

El sistema SHALL evaluar el cumplimiento de una serie contra el objetivo con una tolerancia por defecto de **±5%**, ajustable por parámetro de la consulta dentro del rango 0.01–0.20:

```
tiempo:    cumpliste  ⟺  logrado ≤ objetivo × (1 + tolerancia)
distancia: cumpliste  ⟺  lograda ≥ objetivo × (1 − tolerancia)
velocidad: cumpliste  ⟺  lograda ≥ esperada × (1 − tolerancia)
```

Superar el objetivo (más rápido, más lejos) SHALL contar como cumplimiento, con el delta marcado como positivo. Un valor de tolerancia fuera del rango SHALL ser rechazado con `400`.

#### Scenario: Dentro de la tolerancia

- **WHEN** el tiempo objetivo es 240 s, la tolerancia 0.05 y el tiempo logrado es 250 s
- **THEN** la serie cuenta como cumplida

#### Scenario: Superar el objetivo cuenta como cumplimiento

- **WHEN** el tiempo objetivo es 240 s y el logrado es 230 s
- **THEN** la serie cuenta como cumplida y el delta se reporta positivo

#### Scenario: Tolerancia inválida

- **WHEN** se solicita el resumen con `tolerance=0.5`
- **THEN** el sistema responde `400`

### Requirement: Las series no calculables se excluyen de los KPI y se contabilizan

Una serie SHALL considerarse válida para KPI cuando cumple **las dos** condiciones:

1. **Tiene tiempo logrado**: `COALESCE(active_duration_ms, duration_ms) > 0`. Se evalúa el tiempo *logrado*, no `duration_ms` en exclusiva — una serie con `active_duration_ms` válido y `duration_ms` en `null` es perfectamente medible y no debe descartarse.
2. **Tiene cargado el campo que su `kind` exige para el eje**: `liviano` tiempo **o** distancia; `elongacion` tiempo; `ritmo_continuo` tiempo **y** distancia; `velocidad_tiempo` tiempo; `velocidad_distancia` distancia. El valor debe ser mayor a cero.

El incumplimiento de cualquiera de las dos cuenta en `data_quality.invalid_series`.

#### Scenario: Serie con tiempo activo y duración nula

- **WHEN** la serie tiene `active_duration_ms = 230000` y `duration_ms` en `null`
- **THEN** la serie es **válida** y su tiempo logrado es 230000 ms

#### Scenario: Serie sin ningún tiempo

- **WHEN** `active_duration_ms` y `duration_ms` están ambos en `null`
- **THEN** la serie es inválida y se cuenta en `invalid_series`

#### Scenario: Serie con tiempo pero sin el campo de su eje

- **WHEN** la serie tiene `active_duration_ms = 230000` pero su `kind=velocidad_distancia` no tiene `distance_meters`
- **THEN** la serie es inválida: tiene tiempo, pero no el campo que su eje exige

Las series no válidas SHALL quedar fuera de promedios, porcentajes de cumplimiento y rankings, y SHALL aparecer contadas en `data_quality.invalid_series`.

#### Scenario: Serie de elongación sin tiempo

- **WHEN** una serie tiene `kind=elongacion`, `duration_ms` en `null` y `distance_meters` en `null`
- **THEN** no entra en ningún promedio y se cuenta en `invalid_series`

#### Scenario: Una sola serie inválida no invalida el resto

- **WHEN** un grupo tiene 10 series de las cuales 1 no tiene distancia en un `ritmo_continuo`
- **THEN** el ritmo promedio se calcula sobre las 9 válidas y `invalid_series` vale 1

### Requirement: Solo las sesiones terminadas alimentan los indicadores

El filtro SHALL ser un **LEFT JOIN** a `runner_session` por `(session_instance_id, athlete_user_id)`, no un INNER JOIN.

El motivo es concreto: `workout_feedback` se puede crear **sin** fila `runner_session` — el servicio de feedback no la exige ni la crea. Todo el feedback cargado antes del feature de registro de sesión pertenece a ese caso. Con INNER JOIN, ese histórico **desaparecería del dashboard sin que ningún contador lo señale**, que es la peor forma de perder datos: silenciosa y sin señal.

Por eso la regla es:

| `runner_session.status` | Entra al KPI | Cuenta en |
|---|---|---|
| `finished` / `interrupted` | sí | — |
| `wip` | no | `data_quality.open_sessions` |
| **sin fila** (`NULL`) | **sí** | `data_quality.legacy_sessions` |

El caso `NULL` se incluye porque esos datos son válidos: describen sesiones que sí ocurrieron, y excluirlos vaciaría el dashboard de todo el histórico previo al registro de sesión.

Por eso `data_quality` **no es un conjunto cerrado de cinco campos**: suma `legacy_sessions`, el contador de series que entraron al KPI por una vía que no es la normal. Reúne los dos casos en que eso pasa:

1. La serie **no tiene fila `runner_session`** — datos anteriores al feature de registro de sesión.
2. La serie pertenece a una `ExerciseInstance` **sin `source_exercise_id`** que el backfill no pudo resolver, y que el fallback por nombre normalizado resuelve contra un ejercicio del mismo dueño.

El caso 2 debería ser **cero** después del backfill: la columna se agregó con `AutoMigrate` el 2026-09-21 y no hubo backfill de filas previas, así que la migración del ítem 11 las rellena por `(owner_id, nombre normalizado)`. Queda en NULL solo lo que no matchea —mismo nombre con más de un ejercicio del mismo dueño, o un ejercicio que ya no está en el catálogo— y esas filas **no se descartan**: entran al KPI y se cuentan acá.

Agrupar los dos casos en un campo es lo correcto porque el usuario los ve igual: en los dos, la serie se contou pero sin respaldo completo. El nombre dice "sessions" por el de la tabla, pero cuenta series como el resto del bloque; `open_sessions` sí cuenta sesiones.

Un conjunto cerrado no puede expresar "esto no es un error, es histórico", y sin ese campo el front no puede explicarle al usuario por qué ve menos sesiones de las que recuerda.

#### Scenario: Sesión a medias no degrada el cumplimiento

- **WHEN** un corredor tiene una sesión `wip` con 2 de 5 series reportadas y 4 sesiones `finished` completas
- **THEN** el cumplimiento se calcula solo sobre las 4 sesiones terminadas, y `open_sessions` vale 1

#### Scenario: Feedback sin runner_session

- **WHEN** una serie pertenece a una sesión que nunca tuvo fila en `runner_session`
- **THEN** la serie **entra** al KPI y `legacy_sessions` suma 1

#### Scenario: Instancia que el backfill no pudo resolver

- **WHEN** una `ExerciseInstance` sigue con `source_exercise_id` en NULL porque su nombre normalizado matchea más de un ejercicio del mismo dueño
- **THEN** la serie **entra** al KPI resuelta por el fallback, y `legacy_sessions` suma 1

#### Scenario: El filtro se aplica en la consulta

- **WHEN** se inspecciona la consulta de los indicadores
- **THEN** el filtro por `status` está en el `WHERE` (vía el LEFT JOIN), no en un paso posterior sobre el resultado en memoria

### Requirement: Los aggregates se agrupan por fecha lógica, no por instante

Cada granularidad tiene un anclaje **de calendario**, no relativo al `from`:

| `granularity` | Anclaje | Bucket |
|---|---|---|
| `session` | la fecha de la sesión | un bucket por día con actividad |
| `week` | **lunes** (ISO 8601) | lunes de la semana que contiene la fecha |
| `month` | día 1 del mes | mes calendario |
| `quarter` | día 1 del mes, cada 3 meses (**enero, abril, julio, octubre**) | trimestre calendario |

Un bucket se incluye si **alguna** de sus fechas cae dentro del rango pedido, aunque su inicio quede antes del `from` o su fin después del `to`. El recorte nunca parte una semana ni un trimestre por la mitad para acomodar los bordes: el bucket se devuelve completo.

El sistema SHALL agrupar las series en buckets de tiempo usando `session_date`, que es una fecha lógica. El sistema SHALL **NOT** agrupar por `started_at` ni por ningún campo `timestamptz`, porque el desplazamiento de zona horaria movería las sesiones de la tarde-noche al día equivocado.

Los buckets SHALL admitir granularidad `session`, `week`, `month` y `quarter`.

#### Scenario: Sesión cerca de la medianoche

- **WHEN** una sesión tiene `session_date = 2026-06-10` y `started_at` en la zona horaria del servidor a las 23:30
- **THEN** la serie cuenta en el bucket del 2026-06-10

#### Scenario: Granularidad trimestral

- **WHEN** se pide el timeseries con `granularity=quarter` entre 2026-01-01 y 2026-12-31
- **THEN** los buckets son `2026-01-01`, `2026-04-01`, `2026-07-01` y `2026-10-01`

#### Scenario: Granularidad semanal anclada en lunes

- **WHEN** se pide el timeseries con `granularity=week` entre 2026-06-01 y 2026-06-30
- **THEN** los buckets caen todos en lunes, y el primero es el lunes de la semana que contiene el `from` aunque caiga antes

### Requirement: Solo los ejercicios con rol main alimentan los indicadores

El sistema SHALL contar únicamente las series cuyo ejercicio tiene `role = 'main'` en `session_exercise_instances`. Los roles `warmup` y `cooldown` quedan fuera de todos los indicadores: un calentamiento de 10 minutos no debe alterar el ritmo ni la cumplimiento de una sesión de 40.

El rol **no** está en `workout_feedback` ni en `exercise_instances`: vive en el link `session_exercise_instances`. La consulta SHALL unir por `(session_instance_id, exercise_instance_id)` para obtenerlo, y el filtro SHALL aplicarse en el `WHERE`, no sobre el resultado.

#### Scenario: Calentamiento excluido de los indicadores

- **WHEN** una sesión tiene un `liviano` con rol `warmup` y un `ritmo_continuo` con rol `main`, y el corredor reportó series de ambos
- **THEN** solo las series del `ritmo_continuo` entran en los indicadores y el calentamiento no aparece

### Requirement: El estado de cumplimiento declarado no descarta series

El campo `completion_status` SHALL tratarse como texto no controlado. El sistema SHALL reconocer `completa`, `partial` y `skipped`; cualquier otro valor, o `null`, SHALL contar como `unknown`, SHALL quedar registrado en `data_quality.unknown_status` y **no** SHALL excluir la serie de los indicadores.

#### Scenario: Estado desconocido no pierde la serie

- **WHEN** una serie válida tiene `completion_status = "saltada"` (valor no reconocido)
- **THEN** la serie sigue entrando en los cálculos y `unknown_status` aumenta en 1

### Requirement: El resumen no inventa valores donde no hay dato

Cuando no exista un objetivo válido o no haya series suficientes, el sistema SHALL devolver el campo como `null`. El sistema SHALL **NOT** devolver `0` como sustituto de un valor no calculable, porque `0` es un resultado legítimamente distinto de "no hay dato".

#### Scenario: Ejercicio de elongación sin series

- **WHEN** se pide el resumen de un `elongacion` sin ninguna serie en el período
- **THEN** `achieved_speed_kph`, `delta_pct` y `compliance_rate` vienen `null`, no `0`

### Requirement: El ordenamiento del leaderboard usa una lista explícita de columnas

`GET /dashboard/leaderboard` SHALL aceptar `sort` únicamente entre `best_time_ms`, `mean_speed_kph`, `total_distance_m` y `compliance_rate`, y `order` únicamente entre `asc` y `desc`. Cualquier otro valor SHALL ser rechazado con `400`. El campo de ordenamiento SHALL aparecer en la respuesta para que el cliente lo refleje.

El leaderboard SHALL ordenar los empates de forma determinista: menor tiempo, luego mayor distancia, luego orden alfabético por nombre.

#### Scenario: Columna fuera de la lista

- **WHEN** se pide el leaderboard con `sort=duration_ms`
- **THEN** el sistema responde `400` y no ejecuta la consulta

#### Scenario: Empate de tiempos

- **WHEN** dos corredores tienen el mismo mejor tiempo y distancias distintas
- **THEN** el de mayor distancia queda primero

### Requirement: Las consultas analíticas están acotadas

`from` SHALL ser obligatorio en los endpoints que aceptan rango de fechas. El rango SHALL poder cubrir como máximo **2 años** y SHALL ser rechazado con `400` si lo excede. `page_size` SHALL tener un máximo de `100`.

Estas cotas sustituyen a un rate limiter: acotan el peor caso de cada request individual sin introducir dependencia ni infraestructura nueva.

#### Scenario: Rango excesivo

- **WHEN** se pide el resumen con `from=2015-01-01&to=2026-10-10`
- **THEN** el sistema responde `400`

#### Scenario: Rango dentro del límite

- **WHEN** se pide el resumen con `from=2025-01-01&to=2026-10-10`
- **THEN** el sistema responde `200`

### Requirement: Las tendencias solo se calculan con volumen suficiente

La variación de tendencia SHALL comparar el tercil de series más reciente contra el más antiguo del período.

`trend_pct` SHALL medirse **sobre `achieved_speed_kph`**, la velocidad lograda, y no sobre el mejor tiempo. Se elige la velocidad a propósito: con `best_time_ms` el signo se invierte (bajar de tiempo es mejorar), lo que haría que un "-8 %" significara "va mejor". Con la velocidad, **más alto siempre es mejor**, y el umbral de ±3 % se lee sin ambigüedad.

`trend_pct` = (media del tercil más reciente − media del tercil más antiguo) / media del tercil más antiguo × 100.

`verdict` SHALL ser `improving` si `trend_pct > 3`, `regressing` si `trend_pct < −3`, y `stable` en cualquier otro caso, **inclusive cuando `trend_pct` viene `null`**.

Si la media del tercil más antiguo es `0`, `trend_pct` SHALL venir `null` y `verdict` SHALL ser `stable`: dividir por cero devolvería `Infinity`, que el front no tiene forma de pintar. Con **menos de 3** series en el período, el sistema SHALL devolver la tendencia como `null` y marcar `data_quality.insufficient_data` en `true`.

El sistema SHALL clasificar la señal: mejora por encima de +3 %, estable entre −3 % y +3 %, retroceso por debajo de −3 %.

#### Scenario: Dos series no generan tendencia

- **WHEN** el período contiene 2 series
- **THEN** `trend_pct` viene `null` y `insufficient_data` vale `true`

#### Scenario: Cinco series sí generan tendencia

- **WHEN** el período contiene 5 series
- **THEN** `trend_pct` viene con valor y `verdict` es `improving`, `stable` o `regressing`

### Requirement: El dashboard expone el estado de los datos, no solo los números

Cada respuesta de métricas SHALL incluir un bloque `data_quality` con, como mínimo: `invalid_series`, `degraded_time`, `unknown_status`, `open_sessions` e `insufficient_data`. El bloque SHALL estar presente aunque todos los contadores sean cero, para que el cliente pueda distinguir "no hay datos" de "todos los datos están limpios".

#### Scenario: Todo limpio

- **WHEN** todas las series son válidas, tienen tiempo activo y un estado reconocido
- **THEN** `data_quality` viene presente con todos los contadores en cero

### Requirement: La paleta de gráficos depende de si la serie tiene objetivo cargado, no del tipo

El sistema SHALL indicar, en cada respuesta de métricas, el campo `axis` (el eje del objetivo) y si existe **objetivo cargado**:

- `axis`: `"distance_and_time"`, `"distance"`, `"time"` o `"none"`.

`"none"` SHALL devolverse **solo** para un `kind` que no está en la lista de los cinco vigentes —incluidos los legacy— y en ese caso `has_target` es `false`. No es un valor que se produzca por falta de datos en un tipo conocido.
- `has_target`: `true` cuando el campo del eje tiene valor válido en la `ExerciseInstance`; `false` cuando está en `null`.

Cuando `has_target` sea `false`, el sistema SHALL **NOT** calcular ni devolver `delta_pct`, `compliance_rate` ni velocidad esperada, y SHALL devolverlos en `null`. Presentarlos como `0` haría que el cliente leyera "no cumplió" donde en realidad nunca hubo un objetivo con el cual comparar.

La conmutación depende **del dato, no del `kind`**, porque el objetivo puede faltar en cualquier tipo: un `liviano` cargado solo con distancia no tiene objetivo temporal, y un `velocidad_distancia` creado sin distancia tampoco. Un tipo puede no tener objetivo aunque el `kind` lo declare como su eje.

> **Nota de modelo:** `minutes` es `INT`. Un sprint de 100 m en 12 s equivale a 0,2 minutos, que no es representable — redondeado da 0, y el validador de tiempo exige `> 0`. Por eso los ejercicios de velocidad máxima se modelan como `velocidad_distancia` (100 m, **sin objetivo temporal**), no como `velocidad_tiempo`: es el tipo cuyo eje temporal no puede expresar el sprint. Ojo con la distinción: esos ejercicios **sí** tienen objetivo (el de distancia), así que van a la paleta A reducida; la rama `has_target: false` no la ejercita ninguno de los 12 del seed y queda cubierta por tests del DAO, porque datos sin objetivo pueden llegar igual por filas previas a la migración de tipos.

#### Scenario: Ejercicio con objetivo cargado

- **WHEN** la `ExerciseInstance` tiene `kind=ritmo_continuo` con `distance_m=1000` y `minutes=4`
- **THEN** `axis` es `distance_and_time`, `has_target` es `true`, y se devuelven `delta_pct`, `compliance_rate` y `target_speed_kph`

#### Scenario: Ejercicio sin objetivo cargado

- **WHEN** la `ExerciseInstance` tiene `kind=velocidad_distancia` con `distance_m` en `null`
- **THEN** `axis` es `distance`, `has_target` es `false`, y `delta_pct`, `compliance_rate` y `target_speed_kph` vienen `null`

#### Scenario: Objetivo ausente en un tipo que sí lo declara

- **WHEN** una `ExerciseInstance` de `kind=liviano` tiene `distance_m=3000` y `minutes` en `null`
- **THEN** el sistema mide contra la distancia, declara `axis: "distance"` y `has_target: true`

#### Scenario: Velocidad máxima con objetivo solo de distancia

- **WHEN** una `ExerciseInstance` de `kind=velocidad_distancia` tiene `distance_m=100` y `minutes` en `null`
- **THEN** `axis` es `distance`, `has_target` es `true`, y `best_time_ms` y `speed_kph` vienen calculados a partir de las series

### Requirement: El detalle de series expone objetivo, cumplimiento y calidad por serie

`GET /dashboard/exercises/{id}/series` SHALL tomar `{id}` como el **`exercises.id` de catálogo**, el mismo identificador que expone `/filters`, para que el selector de la barra de filtros pueda encadenarse directamente. `{id}` **no** es `exercise_instances.id`.

El endpoint SHALL aceptar `from`, `to`, `tolerance`, `page` y `page_size`, igual que el resto de los endpoints acotados.

El objetivo con el que se evalúa cada serie es el **congelado en la `ExerciseInstance` de esa serie**, que puede venir de un `source_exercise_id` distinto o nulo (instancias anteriores al change de instancia-referencia-catálogo). Por eso la consulta SHALL unir por la cadena `exercises → exercise_instances.source_exercise_id`, y **no** descartar las instancias sin `source_exercise_id`: se resuelven por coincidencia de nombre normalizado. Perder ese histórico en silencio es el riesgo principal de este endpoint.

`GET /dashboard/exercises/{id}/series` SHALL devolver, por cada serie, los campos derivados que el cliente necesita para pintar cumplimiento y dispersión **sin recalcular nada**:

| Campo | Tipo | Origen |
|---|---|---|
| `session_instance_id` | int64 | de la serie |
| `session_date` | date | `session_date` |
| `athlete_user_id` | int64 | quién ejecutó |
| `feedback_owner_user_id` | int64 | quién reportó |
| `set_number` | int | número de serie |
| `duration_ms` / `active_duration_ms` | int64 nullable | crudos |
| `time_quality` | `"alta"` o `"degraded"` | derivado (`design.md` D4) |
| `distance_meters` | float nullable | crudo |
| `speed_kph` | float nullable | **derivada** de distancia y tiempo, nunca leída de `exercise_instances.speed_kph` |
| `target_minutes` | int nullable | objetivo congelado de la instancia |
| `target_distance_m` | int nullable | objetivo congelado de la instancia |
| `met_target` | bool nullable | `null` cuando `has_target` es `false` |
| `delta_pct` | float nullable | derivado; `null` sin objetivo |
| `completion_status` | string nullable | crudo |
| `band` | `"met"` o `"near"` o `"missed"` o `"unknown"` | banda de cumplimiento derivada, para el gráfico de barras apiladas |
| `tolerance_applied` | float | la tolerancia efectivamente usada |
| `series_count` | int | series válidas en el scope del ejercicio |

`band` SHALL ser `null` cuando `has_target` es `false`, porque sin objetivo no hay cumplimiento que clasificar.

La banda se deriva **según el eje**, porque "cumplir" significa cosas opuestas en tiempo y en distancia.

**La velocidad va del lado de la distancia, no del tiempo.** Ir más rápido es mejorar, igual que ir más lejos: con `velocidad` en la columna de "menos es mejor", correr más rápido que el objetivo caería en `missed`, que es exactamente el error que hace que un corredor que mejora vea su cumplimiento caer. `velocidad` es una magnitud de distancia por unidad de tiempo, no una de tiempo.

| `band` | Eje tiempo (menos es mejor) | Eje distancia **y velocidad** (más es mejor) |
|---|---|---|
| `met` | `logro ≤ objetivo × (1 + tolerancia)` | `logro ≥ objetivo × (1 − tolerancia)` |
| `near` | `objetivo × (1 + tolerancia) < logro ≤ objetivo × (1 + 2 × tolerancia)` | `objetivo × (1 − 2 × tolerancia) ≤ logro < objetivo × (1 − tolerancia)` |
| `missed` | `logro > objetivo × (1 + 2 × tolerancia)` | `logro < objetivo × (1 − 2 × tolerancia)` |
| `unknown` | el objetivo de la serie está cargado pero el campo de su eje está en `null` | ídem |

`unknown` es distinto del `null` de `has_target: false`: el primero es "el ejercicio tiene objetivo pero esta serie no se puede comparar", el segundo es "el ejercicio no tiene objetivo". La distinción importa porque `band_counts.unknown` cuenta series, y el front tiene que poder pintar una categoría que nunca ocurre en el seed.

La respuesta SHALL incluir `page`, `page_size`, `total` y el mismo bloque `units` que el resumen, porque el CSV escribe la unidad en la cabecera de cada columna y no puede pedirla de otro endpoint.

Cuando `has_target` es `false`, los cuatro contadores de `band_counts` van en cero.

#### Scenario: Serie con objetivo y cumplida

- **WHEN** la serie tiene `target_minutes=4`, `active_duration_ms=235000` y `distance_meters=1000`
- **THEN** `time_quality` es `alta`, `met_target` es `true`, `band` es `met` y `speed_kph` viene calculada

#### Scenario: Serie sin objetivo no tiene banda

- **WHEN** la `ExerciseInstance` no tiene objetivo cargado
- **THEN** `met_target`, `delta_pct` y `band` vienen `null`, no `false` ni `0`

### Requirement: El signo de `delta_pct` se invierte en el eje temporal

`delta_pct` SHALL ser siempre **"cuánto mejor o peor respecto del objetivo"**, y no la diferencia aritmética cruda. Eso obliga a **invertir el signo en el eje temporal**, donde menos es mejor:

| Eje | Fórmula | Ejemplo |
|---|---|---|
| `time` (menos es mejor) | `(objetivo − logro) / objetivo × 100` | objetivo 240 s, logro 250 s → **−4,2 %** |
| `distance` (más es mejor) | `(logro − objetivo) / objetivo × 100` | objetivo 4000 m, logro 4200 m → **+5,0 %** |
| `speed` (más es mejor) | `(logro − objetivo) / objetivo × 100` | objetivo 15 km/h, logro 15,3 km/h → **+2,0 %** |
| `distance_and_time` | **la de la velocidad derivada** | objetivo 1000 m en 4 min (15 km/h), logro 1000 m en 3:55 (15,3 km/h) → **+2,0 %** |

Para `axis = "distance_and_time"` el `delta_pct` SHALL medirse sobre la **velocidad esperada contra la velocidad lograda**, no sobre el tiempo: es la única magnitud única que representa a los dos ejes a la vez, y es la que el trainer quiere ver.

Es decir: `velocidad_esperada = distancia_objetivo / tiempo_objetivo`, `velocidad_lograda = distancia_lograda / tiempo_logrado`, y `delta_pct` aplica la fila `speed` sobre esos dos.

El motivo de usar velocidad y no tiempo es el **signo**: si el delta se midiera sobre el tiempo, un corredor que mejora su tiempo vería un número negativo con color de retroceso. Es el mismo argumento por el que `trend_pct` se mide sobre velocidad.

Con la fórmula cruda en el eje temporal, un corredor que mejoró su tiempo vería "−5 %" con color de retroceso. La pantalla delega el color en el signo, así que el signo tiene que significar lo mismo en todos los ejes.

#### Scenario: Mejora en el eje temporal

- **WHEN** el objetivo es 240 s y la serie logra 230 s
- **THEN** `delta_pct` es **positivo** (`+4,2 %`)

### Requirement: El resumen de métricas devuelve un contrato de respuesta completo

`GET /dashboard/metrics/summary` SHALL devolver:

```jsonc
{
  "axis": "distance_and_time",
  "has_target": true,
  "units": {
    "speed": "km/h", "time": "ms", "distance": "m",
    "delta": "%", "compliance": "%", "best_time": "ms", "percentage_point": "pp"
  },
  "totals": {
    "sessions": 18, "series": 72, "valid_series": 68,
    "distance_m": 96000, "time_ms": 32280000,
    "achieved_speed_kph": 10.7, "target_speed_kph": 11.1,
    "delta_pct": -3.6, "compliance_rate": 61.8,
    "best_time_ms": 231000
  },
  "consistency": { "cv": 0.042 },
  "trend": { "trend_pct": -4.1, "verdict": "regressing" },
  "tolerance_applied": 0.05,
  "series_count": 68,
  "data_quality": { "invalid_series": 4, "degraded_time": 7,
                    "unknown_status": 0, "open_sessions": 1,
                    "legacy_sessions": 0, "insufficient_data": false }
}
```

> **Verificación aritmética del ejemplo:** 96 000 m en 32 280 000 ms (8 h 58 min) dan 10,7 km/h ✓.
> `delta_pct` = (10,7 − 11,1) / 11,1 × 100 = **−3,6 %** ✓. Como el ejemplo es un `distance_and_time`, la fila que aplica es `speed` (más es mejor) y el signo negativo se lee directo: 10,7 < 11,1, se corrió más lento de lo previsto. Las 72 − 4 inválidas dan 68 válidas ✓, y 42/68 = **61,8 %** ✓ (el ejemplo se construye con una tasa reconstruible, para que un test pueda verificarlo).

**Escala declarada:** `compliance_rate` es un **porcentaje** (0–100), no una fracción, para que no compita con `delta_pct`. `compliance_pp` es la diferencia entre dos `compliance_rate` en **puntos porcentuales**.

`consistency.cv` SHALL ser el coeficiente de variación de la **velocidad** de las series válidas (`σ(v) / μ(v)`, en km/h), no del ritmo

Se elige velocidad y no ritmo (min/km) a propósito: son inversas, así que dan valores distintos y con la mejora del signo invertida. Con velocidad, menos dispersión y más alto coinciden en "mejor", que es lo que el KPI tiene que poder decir que no orienta ni engaña al usuario.

Las series sin `distance_meters` quedan **excluidas** del cálculo, no con hueco: no tienen velocidad definible. Si quedan menos de 2, `cv` viene `null`.

Si `μ` fuera `0`, `cv` SHALL venir `null`: dividir por cero devolvería `Infinity`, que el front no tiene forma de pintar. Con menos de 2 series válidas, `cv` también SHALL venir `null`. El cliente SHALL **NOT** recalcular consistencia: si el backend no lo expone, el front tiene que derivarlo y la pantalla puede diferir de la API.

`tolerance_applied` y `series_count` SHALL viajar en la respuesta porque el tooltip de KPI necesita mostrar la tolerancia usada y cuántas series entraron, y no hay otra fuente para eso.

#### Scenario: Resumen completo

- **WHEN** se pide el resumen de un `ritmo_continuo` con 68 series válidas
- **THEN** la respuesta trae `axis`, `has_target`, `units`, `totals`, `consistency`, `trend`, `tolerance_applied`, `series_count` y `data_quality`

#### Scenario: Consistencia sin volumen suficiente

- **WHEN** hay una sola serie válida
- **THEN** `consistency.cv` viene `null`

### Requirement: El timeseries devuelve buckets con la unidad y el objetivo resueltos

`GET /dashboard/metrics/timeseries` SHALL devolver `granularity`, el bloque `units`, `data_quality`, y `buckets[]` con `period_start` (fecha lógica), `sessions`, `series`, `valid_series`, `achieved_speed_kph`, `target_speed_kph`, `delta_pct`, `compliance_rate`, `best_time_ms` y `band_counts` (`met`, `near`, `missed`, `unknown`).

Los buckets SHALL incluir `target_speed_kph` solo cuando el bucket tiene objetivo cargado; en caso contrario `null`.

`band_counts` SHALL estar presente aunque los cuatro contadores sean cero, para que el gráfico de barras apiladas pueda pintar la estructura sin tratarlo como dato ausente.

#### Scenario: Bucket sin objetivo

- **WHEN** el bucket agrupa sesiones de un ejercicio sin objetivo cargado
- **THEN** `target_speed_kph` y `delta_pct` vienen `null` y `band_counts` tiene los cuatro contadores en cero

### Requirement: La comparación de períodos declara su respuesta completa

`POST /dashboard/comparisons/periods` compara el período pedido contra el período inmediatamente anterior **de la misma longitud**, y SHALL devolver:

```jsonc
{
  "current":  { "from": "2026-09-01", "to": "2026-09-30", "series": 24,
                "total_distance_m": 32000, "mean_speed_kph": 11.2,
                "compliance_rate": 58.0, "best_time_ms": 229000 },
  "previous": { "from": "2026-08-02", "to": "2026-08-31", "series": 21,
                "total_distance_m": 29500, "mean_speed_kph": 10.9,
                "compliance_rate": 64.0, "best_time_ms": 233000 },
  "delta": { "series": 3, "distance_pct": 8.5, "speed_pct": 2.8,
             "compliance_pp": -6.0, "best_time_pct": 1.7 },
  "verdict": "regressing",
  "units": { "speed": "km/h", "time": "ms", "distance": "m",
           "delta": "%", "compliance": "%", "best_time": "ms",
           "percentage_point": "pp" },
  "data_quality": { "invalid_series": 0, "degraded_time": 2,
                    "unknown_status": 0, "open_sessions": 1,
                    "legacy_sessions": 0, "insufficient_data": false }
}
```

`current` y `previous` son **períodos completos**, inclusivo de ambos extremos. El período anterior se calcula desplazando `current` hacia atrás exactamente su cantidad de días: un período de 30 días del 1 al 30 de septiembre se compara contra el 2 al 31 de agosto.

`delta.best_time_pct` SHALL expresarse como mejora **positiva cuando el tiempo bajó**: `(prev − curr) / prev × 100`. Con los valores del ejemplo, (233000 − 229000) / 233000 = **+1,7 %** ✓. `delta.distance_pct` y `delta.speed_pct` sí usan `(curr − prev) / prev × 100`, donde más alto es mejor. La asimetría es intencional y es la razón por la que `best_time` necesita un nombre propio en el bloque: invertir el signo de un campo sin declararlo es la forma más fácil de que un cliente muestre "retroceso" donde hubo mejora.

`verdict` SHALL ser `improving`, `regressing` o `stable` según `delta.compliance_pp`: `improving` si `> +2`, `regressing` si `< −2`, `stable` en el resto.

Si el período anterior no tiene ninguna serie, `previous` y `delta` SHALL venir en `null` y `verdict` SHALL ser `stable`: no hay con qué comparar, y presentar la comparación como estable sería mentir.

#### Scenario: Período anterior sin datos

- **WHEN** el período actual tiene series y el inmediatamente anterior no tiene ninguna
- **THEN** `previous` y `delta` vienen `null` y `verdict` es `stable`

#### Scenario: Mejora de mejor tiempo

- **WHEN** el mejor tiempo bajó de 233 s a 229 s
- **THEN** `delta.best_time_pct` es **positivo** (`+1,7 %`), porque bajar el tiempo es mejorar

#### Scenario: Período anterior calculado por el sistema

- **WHEN** se pide la comparación con `current = {2026-06-08 → 2026-06-14}` y sin período anterior
- **THEN** el sistema compara contra `2026-06-01 → 2026-06-07`

#### Scenario: Diferencia de cumplimiento en puntos porcentuales

- **WHEN** el cumplimiento del período actual es 58 % y el del anterior 64 %
- **THEN** `delta.compliance_pp` vale `-6.0`, y **no** `-9.4 %`: son puntos porcentuales, no porcentaje relativo

### Requirement: La comparación entre corredores devuelve una fila por corredor comparable

`POST /dashboard/comparisons` SHALL devolver `series[]` con, por cada `athlete_user_id` comparado: `sessions`, `valid_series`, `best_time_ms`, `mean_speed_kph`, `total_distance_m`, `compliance_rate` y `trend` (`trend_pct`, `verdict`).

La respuesta SHALL incluir el mismo bloque `data_quality` y el mismo bloque `units` que el resumen.

Los valores sin objetivo o sin volumen suficiente vendrán `null` bajo la misma regla del resumen.

#### Scenario: Comparar tres corredores

- **WHEN** se comparan tres corredores del scope sobre un `ritmo_continuo`
- **THEN** la respuesta trae una fila por corredor con sus métricas y el `data_quality` común

### Requirement: El leaderboard identifica la posición propia

`GET /dashboard/leaderboard` SHALL devolver `items[]` con `rank`, `athlete_user_id`, `name`, `surname`, `sessions`, `best_time_ms`, `mean_speed_kph`, `total_distance_m` y `compliance_rate`.

Cada ítem SHALL incluir `is_self` (`true` solo para el corredor solicitante) y `position` con la posición global de ese corredor **aunque no esté en la página actual**, de modo que el cliente pueda mostrar "vas 14°" sin tener que traer todas las páginas. `position` se calcula con `RANK()` sobre **todas** las filas del scope, no sobre la página, y con **la misma columna y el mismo desempate** que el `ORDER BY` de la página: si difieren, el número que ve el corredor no es su posición real.

La respuesta SHALL incluir el bloque `units` y el `data_quality`, como las demás respuestas métricas: el front rotula los ejes del ranking con la unidad y no puede inventarla.

La respuesta SHALL incluir `page`, `page_size`, `total` y `sort` (la columna efectivamente usada).

#### Scenario: El corredor no está en la primera página

- **WHEN** un corredor pide la página 1 y quedó en la posición 14 de 30
- **THEN** recibe `position: 14` para sí mismo, aunque su fila no esté en `items`

### Requirement: El bloque de calidad de datos tiene un conjunto cerrado

El bloque `data_quality` SHALL contener **exactamente** estos seis campos, sin más ni menos: `invalid_series`, `degraded_time`, `unknown_status`, `open_sessions`, `legacy_sessions`, `insufficient_data`.

Los cinco primeros son defecto; **`legacy_sessions` es informativo**: cuenta series válidas que se incluyen igual, y por eso el front SHALL mostrarlas como nota al pie y **no** como advertencia.

El campo `data_quality` SHALL estar presente en `summary`, `timeseries`, `comparisons`, `comparisons/periods`, `leaderboard` y `exercises/{id}/series`, aunque todos los contadores sean cero. El cliente necesita distinguir "no hay datos" de "los datos están limpios", y un conjunto abierto impide tipar el banner.

#### Scenario: Todo limpio

- **WHEN** todas las series son válidas, tienen tiempo activo, un estado reconocido y todas tienen `runner_session`
- **THEN** `data_quality` está presente con los seis campos en cero o `false`

#### Scenario: Solo histórico sin registro de sesión

- **WHEN** todas las series del período son válidas pero ninguna tiene `runner_session`
- **THEN** el resto de los contadores vale cero, `legacy_sessions` es el único distinto de cero, y los KPI se calculan normalmente

### Requirement: El filtro de exercises distingue el objetivo vigente del congelado

`GET /dashboard/filters` SHALL devolver, por ejercicio, el objetivo **vigente del catálogo**, en los campos `current_target_minutes` y `current_target_distance_m` y SHALL marcarlo explícitamente como tal. Ese valor es el que el entrenador editó, no el que se mide.

Los objetivos con los que se compara cada serie son los **congelados en la `ExerciseInstance`** y no aparecen en `/filters`. Un cliente SHALL **NOT** presentar el objetivo de `/filters` como el objetivo contra el que se evalúa una sesión histórica.

#### Scenario: Objetivo editado después del stamping

- **WHEN** un entrenador cambió el objetivo de un ejercicio después de que varias sesiones fueran estampadas
- **THEN** `/filters` muestra el objetivo nuevo, y la medición de esas sesiones sigue usando el congelado

### Requirement: El filtro de exercises es completo para las dos vistas

`GET /dashboard/filters` SHALL devolver, en la vista de entrenador, la lista de corredores del scope, porque la comparación y el leaderboard los requieren. En la vista de corredor SHALL devolver solo la entrada propia.

#### Scenario: Vista de entrenador

- **WHEN** un entrenador pide `/dashboard/filters`
- **THEN** recibe equipos, grupos y corredores de su scope

#### Scenario: Vista de corredor

- **WHEN** un corredor pide `/dashboard/filters`
- **THEN** recibe solo su propia entrada en `athletes`

### Requirement: Los parámetros inválidos se rechazan con 400 y sin consulta

El sistema SHALL rechazar con `400` y **sin ejecutar la consulta** cuando: `from` esté ausente; el rango supere los 2 años; `tolerance` esté fuera de `[0.01, 0.20]`; `sort` o `order` estén fuera de la whitelist; o `granularity` no sea uno de los cuatro valores admitidos.

Un recurso inexistente SHALL devolver `404`; uno existente fuera de scope SHALL devolver `403`.

#### Scenario: Falta el rango

- **WHEN** se pide el resumen sin `from`
- **THEN** el sistema responde `400` sin ejecutar la agregación

#### Scenario: Granularidad desconocida

- **WHEN** se pide el timeseries con `granularity=hour`
- **THEN** el sistema responde `400`

#### Scenario: Recurso inexistente

- **WHEN** se pide `GET /exercises/999999/series` con un id que no existe
- **THEN** el sistema responde `404`

#### Scenario: El 400 no es reintentable

- **WHEN** el cliente recibe `400` por un parámetro inválido
- **THEN** el sistema SHALL incluir en el cuerpo un `code` estable en la convención del repositorio (`INVALID_RANGE`, `INVALID_TOLERANCE`, `INVALID_SORT`, `INVALID_GRANULARITY`, `MISSING_FROM`, `INVALID_PAGE_SIZE`), expuesto en `APIError.Code` para que el cliente **no** ofrezca reintentar sin cambiar la request

### Requirement: El origen de la serie se distingue de su performer

El sistema SHALL exponer `athlete_user_id` (quién ejecutó) y `feedback_owner_user_id` (quién reportó) en el detalle de series. En la vista de entrenador, el leaderboard SHALL agrupar por `athlete_user_id`, ignorando `feedback_owner_user_id`.

#### Scenario: Entrenador reporta una sesión de corredor
- **WHEN** el entrenador cargó el feedback de una sesión de un corredor
- **THEN** el corredor figura en el leaderboard por sus propias series, no el entrenador
