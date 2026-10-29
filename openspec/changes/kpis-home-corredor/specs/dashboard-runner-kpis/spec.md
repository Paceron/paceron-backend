# Spec Delta

## Purpose

KPIs del Home del corredor: un endpoint de lectura que agrega sus series de
todos sus grupos y equipos en un período calendario, separado en cuatro
pestañas que se calculan bajo demanda. Las fórmulas viven en el backend y el
frontend solo las muestra.

## ADDED Requirements

### Requirement: El endpoint SHALL devolver solo los KPIs del corredor autenticado

El sistema SHALL exponer `GET /api/v1/dashboard/runner-kpis`. El corredor SHALL
identificarse exclusivamente por el token de autenticación; el contrato NO SHALL
aceptar `athlete_user_id`, `user_id`, `team_id` ni `group_id`.

La consulta SHALL partir de `workout_feedback` filtrando por
`athlete_user_id = callerID`, sin exigir que exista un grupo o equipo en el
scope actual. Por eso SHALL incluir las series de todos los grupos y equipos del
corredor, y también las de grupos de los que ya no es miembro. El dato es del
corredor y el Home no tiene un equipo como ancla de autorización.

El endpoint SHALL excluir:

- filas con `deleted_at` no nulo;
- sesiones con `runner_session.status = 'wip'` del mismo corredor;
- series cuya fecha quede fuera de `[from, to]`, ambos inclusive.

El endpoint SHALL incluir sesiones `finished`, `interrupted` y las que no tienen
fila en `runner_session`, con el mismo criterio de compatibilidad histórica que
usa el dashboard existente.

El endpoint NO SHALL filtrar por `session_exercise_instances.role`: el pedido es
sumar todos los ejercicios del corredor. Las pestañas por tipo sí acotan por
`kind`, pero la pestaña general incluye `warmup`, `main` y `cooldown`.

El endpoint SHALL ser de solo lectura: no SHALL escribir, sellar ni recalcular
filas.

#### Scenario: Corredor con dos equipos

- **WHEN** un corredor con series en el grupo 12 del equipo 4 y en el grupo 30 del equipo 9 pide sus KPIs
- **THEN** la respuesta suma las series de ambos grupos y no requiere ningún `team_id`

#### Scenario: Grupo del que ya no es miembro

- **WHEN** el corredor tiene series de julio en un grupo que dejó en agosto y pide el período de julio
- **THEN** esas series entran al agregado, porque son datos propios y la fecha está dentro del período

#### Scenario: Sesión en curso

- **WHEN** existe una `runner_session` del corredor en estado `wip` dentro del período
- **THEN** sus series no entran a ningún KPI

#### Scenario: Intento de leer a otro corredor

- **WHEN** un llamador agrega `athlete_user_id` a la request
- **THEN** el parámetro es ignorado y la respuesta sigue siendo la del corredor autenticado; si el parámetro se declara como desconocido en el binding, el sistema responde `400`, pero nunca devuelve datos de otro corredor

#### Scenario: Sin token

- **WHEN** se llama al endpoint sin un token válido
- **THEN** el sistema responde `401` y no ejecuta la consulta

### Requirement: El período SHALL ser un calendario explícito

El endpoint SHALL aceptar:

| Parámetro | Valores | Default | Significado |
|---|---|---|---|
| `period` | `month`, `week`, `day` | `month` | Calendario del período |
| `reference_date` | `YYYY-MM-DD` | fecha actual del servidor | Fecha que ancla el período |

Reglas de cálculo, sobre fechas de calendario y sin horas:

- **`month`:** desde el primer día del mes de `reference_date` hasta el último
  día de ese mes. `days_considered` SHALL ser la cantidad real de días del mes
  (28, 29, 30 o 31), no los días transcurridos.
- **`week`:** desde el lunes hasta el domingo de la semana ISO que contiene
  `reference_date`. `days_considered` SHALL ser 7.
- **`day`:** desde y hasta `reference_date`. `days_considered` SHALL ser 1.

La respuesta SHALL incluir `period`, `reference_date`, `from`, `to` y
`days_considered`. `from` y `to` SHALL viajar en ISO `YYYY-MM-DD` y SHALL ser
inclusivos en los dos extremos.

El frontend SHALL enviar siempre `reference_date` con la fecha local del
dispositivo. El default del servidor existe solo para que el endpoint sea
utilizable desde otras herramientas; no es la fuente de verdad para el Home.

El sistema SHALL rechazar con `400` y code estable:

- un `period` fuera de `month|week|day` (`INVALID_PERIOD`);
- una `reference_date` que no sea una fecha real de calendario (`INVALID_DATE`),
  incluido `2026-02-30`;
- un `tab` fuera de los cuatro valores admitidos (`INVALID_TAB`).

#### Scenario: Mes calendario completo

- **WHEN** se pide `period=month&reference_date=2026-10-28`
- **THEN** `from=2026-10-01`, `to=2026-10-31` y `days_considered=31`

#### Scenario: Mes de febrero bisiesto

- **WHEN** se pide `period=month&reference_date=2028-02-10`
- **THEN** `from=2028-02-01`, `to=2028-02-29` y `days_considered=29`

#### Scenario: Semana ISO

- **WHEN** se pide `period=week&reference_date=2026-10-28` (miércoles)
- **THEN** `from=2026-10-26` (lunes), `to=2026-11-01` (domingo) y `days_considered=7`

#### Scenario: Día

- **WHEN** se pide `period=day&reference_date=2026-10-28`
- **THEN** `from=2026-10-28`, `to=2026-10-28` y `days_considered=1`

#### Scenario: Fecha imposible

- **WHEN** se pide `reference_date=2026-02-30`
- **THEN** el sistema responde `400` con code `INVALID_DATE`

### Requirement: Las pestañas SHALL calcularse por request

El endpoint SHALL aceptar `tab` con estos valores y SHALL devolver solo el bloque
de métricas de esa pestaña:

| `tab` | Series incluidas | Uso en el Home |
|---|---|---|
| `general` | todas las series del período | Números generales |
| `ritmo_continuo` | `kind = 'ritmo_continuo'` | Ritmo continuo |
| `velocidad_tiempo` | `kind = 'velocidad_tiempo'` | Velocidad en tiempo |
| `velocidad_distancia` | `kind = 'velocidad_distancia'` | Velocidad en distancia |

`tab` SHALL ser opcional y su default SHALL ser `general`.

La pestaña `velocidad_distancia` se muestra como “Velocidad en distancia” y
consume `kind = 'velocidad_distancia'`: es el único tipo métrico que queda además
de `ritmo_continuo` y `velocidad_tiempo`, y sus tres métricas —distancia,
velocidad promedio y mejor velocidad— son las que el modelo puede calcular de
forma consistente. El label es una decisión de UI y puede cambiar sin tocar el
contrato; el `tab` no.

Cada request SHALL calcular una sola pestaña. El frontend NO SHALL recibir las
cuatro en una sola respuesta ni prefetcharlas al montar el Home; así el costo de
cada tab se paga recién cuando el corredor la abre.

#### Scenario: Cálculo bajo demanda

- **WHEN** se pide `tab=ritmo_continuo`
- **THEN** la respuesta trae solo las métricas de ritmo continuo y no las de velocidad

#### Scenario: Tab desconocido

- **WHEN** se pide `tab=comparacion`
- **THEN** el sistema responde `400` con code `INVALID_TAB`

#### Scenario: Default general

- **WHEN** se omite `tab`
- **THEN** el sistema responde como `tab=general`

### Requirement: Las métricas SHALL tener fórmulas únicas y declaradas

La respuesta SHALL incluir `units` con las unidades del contrato:

```json
{ "distance": "m", "speed": "km/h", "time": "ms" }
```

El bloque `metrics` SHALL contener siempre todas las claves, con `null` cuando la
métrica no es calculable en esa pestaña o no hay datos. Un `null` SHALL
significar “sin dato”, nunca `0`.

Las fórmulas SHALL ser:

| Campo | Pestaña | Fórmula |
|---|---|---|
| `total_distance_m` | todas | suma de `distance_meters` positivos de las series incluidas |
| `best_day_distance_m` | `general` | máximo de la suma de distancias positivas agrupada por `session_date` |
| `best_day_date` | `general` | fecha ISO del día que produjo `best_day_distance_m` |
| `average_speed_kph` | todas | en `general`, media ponderada por distancia de las series `ritmo_continuo`; en las demás pestañas, de las series incluidas |
| `best_speed_kph` | `general`, `ritmo_continuo`, `velocidad_distancia` | máximo de las velocidades promedio logradas por serie; en `general` considera todos los tipos |
| `best_time_ms` | `velocidad_tiempo` | mínimo del tiempo logrado por serie |
| `best_distance_m` | `ritmo_continuo` | máxima distancia lograda en una sola serie |

Reglas de cálculo:

- La velocidad lograda SHALL derivarse de `distance_meters` y del tiempo logrado;
  NO SHALL leerse de `exercise_instances.speed_kph`.
- El tiempo logrado SHALL ser `COALESCE(active_duration_ms, duration_ms)`, la
  misma regla del dashboard existente.
- La media de velocidad SHALL ponderar por distancia, no ser una media simple.
- `best_speed_kph` SHALL ser el máximo de las velocidades promedio calculadas
  serie por serie. NO SHALL ser un porcentaje ni una comparación contra el
  objetivo, y NO SHALL promediar las velocidades de varias series.
- En `general`, `average_speed_kph` SHALL calcularse solo con las series
  `kind = 'ritmo_continuo'`, porque la fila se rotula “Velocidad promedio en
  ritmo continuo”. Si no hay series de ese tipo, SHALL ser `null` aunque la
  pestaña tenga series de otros tipos.
- Una serie sin la magnitud necesaria para una métrica SHALL quedar fuera de esa
  métrica, pero SHALL seguir contando en `series_count` si pertenece a la
  pestaña.
- `valid_series` SHALL contar las series que pasan las reglas de validez del
  dashboard para su `kind`.
- `has_data` SHALL ser `false` cuando no hay ninguna serie en la pestaña; en ese
  caso todas las métricas SHALL ser `null` y los contadores SHALL ser `0`.
- Los empates de `best_day_distance_m` SHALL resolverse por la fecha más antigua.
- Los valores SHALL viajar sin redondear; el frontend decide el formato visible.

#### Scenario: Distancia total de todos los tipos

- **WHEN** el corredor tiene 5 km de `ritmo_continuo`, 2 km de `velocidad_tiempo` y 1 km de `liviano` en el mes
- **THEN** `general.total_distance_m` es 8000, sin importar grupos ni roles

#### Scenario: Mejor día

- **WHEN** el corredor sumó 8 km el 3 de octubre y 12 km el 11 de octubre
- **THEN** `best_day_distance_m` es 12000 y `best_day_date` es `2026-10-11`

#### Scenario: Mejor velocidad por serie

- **WHEN** una serie promedió 12,5 km/h y otra 14 km/h, ambas con distancia y tiempo calculables
- **THEN** `best_speed_kph` es 14, porque es el máximo de las velocidades promedio por serie

#### Scenario: Mejor velocidad general entre tipos

- **WHEN** en `general` la serie más rápida fue de `velocidad_distancia` a 16 km/h
- **THEN** `best_speed_kph` es 16, sin importar el tipo de la serie

#### Scenario: Sin velocidad calculable

- **WHEN** ninguna serie del período tiene distancia y tiempo calculables
- **THEN** `best_speed_kph` es `null`

#### Scenario: Velocidad promedio ponderada

- **WHEN** hay una serie de 21 km a 5 km/h y otra de 4 km a 15 km/h
- **THEN** `average_speed_kph` es cercana a 6,6 km/h y no el promedio simple de 10 km/h

#### Scenario: Promedio general solo de ritmo continuo

- **WHEN** en `general` hay 10 km de `ritmo_continuo` a 10 km/h y 5 km de `velocidad_distancia` a 20 km/h
- **THEN** `average_speed_kph` es 10 km/h, porque la fila se rotula “Velocidad promedio en ritmo continuo” y excluye las series de otros tipos

#### Scenario: General sin ritmo continuo

- **WHEN** en `general` solo hay series de `velocidad_tiempo`
- **THEN** `has_data` es `true`, `total_distance_m` suma esas series y `average_speed_kph` es `null`

#### Scenario: Pestaña sin datos

- **WHEN** el corredor no tiene series de `velocidad_tiempo` en el período
- **THEN** `has_data` es `false`, `series_count` es `0` y `best_time_ms` es `null`

#### Scenario: Cero real

- **WHEN** hay una serie con `distance_meters = 0` y otra con 3000 m
- **THEN** `total_distance_m` es 3000; el cero aporta cero y no convierte la métrica en `null`

### Requirement: La respuesta SHALL ser estable y compatible

La respuesta SHALL tener esta forma, con las claves de `metrics` siempre
presentes aunque su valor sea `null`:

```json
{
  "period": "month",
  "reference_date": "2026-10-28",
  "from": "2026-10-01",
  "to": "2026-10-31",
  "days_considered": 31,
  "tab": "general",
  "units": { "distance": "m", "speed": "km/h", "time": "ms" },
  "has_data": true,
  "series_count": 42,
  "valid_series": 35,
  "sessions": 12,
  "metrics": {
    "total_distance_m": 84250.5,
    "best_day_distance_m": 12500,
    "best_day_date": "2026-10-12",
    "average_speed_kph": 10.8,
    "best_speed_kph": 16.4,
    "best_time_ms": null,
    "best_distance_m": null
  }
}
```

`series_count` SHALL contar todas las filas incluidas en la pestaña.
`valid_series` SHALL contar las calculables. `sessions` SHALL contar
`session_instance_id` distintos entre las filas incluidas.

La ruta nueva SHALL agregarse sin modificar los DTOs, rutas, permisos ni
respuestas de los ocho endpoints de `/dashboard` existentes. La suite actual del
backend SHALL seguir pasando sin cambios de comportamiento.

#### Scenario: Compatibilidad

- **WHEN** se ejecuta la suite existente del dashboard después de agregar la ruta
- **THEN** los tests de `/dashboard/filters`, `/metrics/summary`, `/metrics/timeseries`, `/leaderboard`, `/exercises/{id}/series`, `/comparisons`, `/comparisons/periods` y `/comparisons/sessions` siguen pasando

#### Scenario: Claves estables

- **WHEN** se pide `tab=velocidad_tiempo`
- **THEN** `metrics.total_distance_m`, `metrics.average_speed_kph` y `metrics.best_time_ms` están presentes, y las métricas de otras pestañas están en `null`
