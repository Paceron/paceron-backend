# Design

## Context

Estado actual que condiciona el diseño. La motivación está en `proposal.md` —
Why.

**Lo que ya existe y se reutiliza:**

- Las fórmulas canónicas viven en `cmd/api/domains/dashboard/` y están
  documentadas en `openspec/changes/dashboard-analitico-rendimiento/KPI_FORMULAS.md`:
  tiempo logrado (`COALESCE(active_duration_ms, duration_ms)`), velocidad
  derivada, validez por `kind`, media ponderada por distancia y `null` como “sin
  dato”.
- `workout_feedback` ya tiene el índice `idx_feedback_athlete_date`
  (`athlete_user_id`, `session_date`), que cubre el filtro principal de este
  endpoint.
- El patrón de controller + service + DAO + dominio puro ya está establecido por
  `dashboard_controller.go`, `dashboard_service.go`, `dashboard_dao.go` y
  `domains/dashboard/`.
- El proyecto no tiene rate limiter. El endpoint se acota por período calendario
  (1 día, 7 días o un mes) y por un único corredor.

**Lo que NO se puede reutilizar tal cual:**

`DashboardDao.FindSeries` exige `GroupIDs` y aplica
`session_exercise_instances.role = 'main'`. El Home necesita exactamente lo
contrario en dos dimensiones: no tiene un grupo como ancla y debe sumar todos los
roles. Forzar ese DAO con “todos los grupos” obligaría a resolver membresías
actuales y perdería el histórico de grupos dejados, además de excluir
calentamientos y vueltas a la calma.

## Goals / Non-Goals

**Goals:**

- Un endpoint de lectura, propio, con período calendario y cuatro pestañas.
- Fórmulas puras, testeables sin HTTP ni base de datos.
- Cero cambios de comportamiento en los endpoints existentes.
- Una sola query por request, acotada por `athlete_user_id` y fecha.

**Non-Goals:**

- No se reutiliza `DashboardScope` ni se agrega un nuevo modelo de permisos: el
  token ya identifica al corredor y el endpoint no acepta ids de otras personas.
- No se agregan gráficos, comparaciones ni tendencias.
- No se toca `runner_session` ni se crean sesiones.
- No se modifica el esquema.

## Decisions

### D1: Consulta propia desde `workout_feedback`, no desde el calendario

La consulta SHALL partir de `workout_feedback` y unir `exercise_instances` para
obtener `kind` y el objetivo congelado. NO SHALL unir `group_calendar_days` ni
`session_exercise_instances`, porque el objetivo es el dato del corredor, no el
grupo, y porque esos joins impondrían membresía y rol.

Esbozo:

```sql
FROM workout_feedback wf
JOIN exercise_instances ei ON ei.id = wf.assigned_exercise_id
LEFT JOIN runner_session rs
  ON rs.session_instance_id = wf.assigned_session_id
 AND rs.athlete_user_id = wf.athlete_user_id
WHERE wf.athlete_user_id = ?
  AND wf.session_date BETWEEN ? AND ?
  AND wf.deleted_at IS NULL
  AND (rs.status IN ('finished', 'interrupted') OR rs.status IS NULL)
ORDER BY wf.session_date, wf.id
```

No se filtra por `team_id`, `group_id` ni `role`. La consulta usa
`idx_feedback_athlete_date`.

**Alternativa descartada:** resolver todos los grupos actuales del corredor y
llamar a `FindSeries`. Perdería el histórico de grupos dejados, excluiría
`warmup`/`cooldown` y agregaría una consulta de membresías que el Home no
necesita.

### D2: El período es una función pura de calendario

`ResolveRunnerPeriod(period, referenceDate)` devuelve `from`, `to`,
`daysConsidered` y `referenceDate` normalizada. No usa la hora del servidor
después de resolver el default. El mes usa `time.Date(year, month+1, 0, ...)`
para obtener el último día real; la semana usa el lunes ISO calculado con
`(weekday+6)%7`.

Las fechas se comparan como `DATE`, no como instantes. `from` y `to` son
inclusivos.

**Por qué el mes completo y no los días transcurridos:** el pedido dice
“cantidad de días del mes”. Mostrar 28 cuando octubre tiene 31 haría que el
título y el período no coincidieran. La UI puede aclarar que el período es
calendario; el contrato no inventa un recorte por “hoy”.

### D3: Un builder por pestaña, sobre las mismas filas

`BuildRunnerKpis(rows, tab)` recibe las filas ya scoped por el DAO y devuelve el
DTO. La selección de pestaña ocurre en el dominio:

- `general`: todas las filas;
- `ritmo_continuo`, `velocidad_tiempo`, `velocidad_distancia`: filtra por
  `kind`.

El builder reutiliza `achievedFromSeries`, `speedFrom` e `isValidSeries` del
paquete `dashboard`. No duplica la derivación de velocidad ni el fallback de
tiempo.

`best_speed_kph` es el máximo de las velocidades promedio calculadas serie por
serie: no es un porcentaje ni una comparación contra el objetivo, y no promedia
varias series. En `general` considera todos los tipos.

### D4: Todas las claves, `null` cuando no hay dato

El DTO tiene un único `RunnerKpisMetrics` con todas las claves. Las que no
aplican a la pestaña o no son calculables viajan `null`. Esto evita que el front
tenga que distinguir “la clave no existe porque es otra pestaña” de “no hay
dato”, y mantiene el contrato estable si mañana se agrega una métrica.

Los contadores (`series_count`, `valid_series`, `sessions`) y `has_data` sí
viajan siempre como números/booleano. Con `has_data=false`, las métricas son
`null` y los contadores 0.

### D5: La pestaña se paga cuando se abre

El frontend abre el Home con `general`. Al tocar otra pestaña hace una request
con ese `tab`. El backend no devuelve las cuatro pestañas ni el frontend las
precarga: el pedido es explícito sobre el cálculo bajo demanda y el costo de
cada agregado se paga una vez.

Esto también hace que el loading sea observable: la pestaña nueva no tiene datos
previos, así que la UI muestra la animación del corredor mientras resuelve.

### D6: Sin migraciones

No hace falta un índice nuevo:

- `idx_feedback_athlete_date (athlete_user_id, session_date)` ya existe por el
  change de gamificación.
- El JOIN a `exercise_instances` va por PK.
- El LEFT JOIN a `runner_session` usa el índice por `(session_instance_id,
  athlete_user_id)` que ya respalda el unique vigente.

Si en producción el plan muestra un scan por el rango, se evalúa un índice
parcial en otro change; no se agrega “por las dudas”.

### D7: Ruta y compatibilidad

La ruta va en el bloque del dashboard, después de los endpoints existentes:

```go
r.GET("/api/v1/dashboard/runner-kpis", app.dashboardController.RunnerKpis)
```

No se toca ninguna ruta previa. El controller nuevo usa el mismo
`respondDashboardError` para que los `400`, `401` y `500` mantengan el formato
`apierror.APIError` del resto del módulo.

### D8: Errores estables

Se agregan tres codes a `domains/constants/error_code.go`:

| Code | Cuándo |
|---|---|
| `INVALID_PERIOD` | `period` fuera de `month|week|day` |
| `INVALID_DATE` | `reference_date` no es una fecha real |
| `INVALID_TAB` | `tab` fuera de los cuatro valores |

No se reutiliza `INVALID_RANGE` porque en este endpoint no hay un rango de dos
fechas: el rango lo deriva el servidor del período. Un code específico permite
que el front distinga un error de programación de un filtro de usuario.

## API Contract

```http
GET /api/v1/dashboard/runner-kpis?period=month&reference_date=2026-10-28&tab=general
Authorization: Bearer <token>
```

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

## Data Rules

| Concepto | Regla |
|---|---|
| Serie incluida | `athlete_user_id = caller`, fecha en rango, no borrada, sesión no `wip` |
| Rol | sin filtro: entran `warmup`, `main` y `cooldown` |
| Tiempo logrado | `COALESCE(active_duration_ms, duration_ms)` |
| Velocidad | derivada de distancia y tiempo, nunca leída de la instancia |
| Distancia total | suma de distancias positivas de las series incluidas |
| Mejor día | máximo de la suma diaria de distancias |
| Velocidad promedio | media ponderada por distancia; en `general`, solo series `ritmo_continuo` |
| Mejor velocidad | máximo de las velocidades promedio por serie; en `general`, de todos los tipos |
| Mejor tiempo | mínimo del tiempo logrado |
| Mejor distancia | máximo de distancia por serie |
| Sin datos | `has_data=false`, métricas `null`, contadores 0 |

## Risks / Trade-offs

| Riesgo | Mitigación |
|---|---|
| El agregado “todos los grupos” sorprende al corredor | El título muestra el período y el endpoint no acepta otro corredor; no hay cambio de alcance silencioso |
| Incluir `warmup`/`cooldown` infla la distancia general | Es el pedido explícito (“todos los ejercicios”); las pestañas por tipo siguen separando el análisis |
| Un `reference_date` del servidor cruza de día respecto del dispositivo | El frontend siempre envía la fecha local; el default del servidor es solo un fallback |
| El cálculo se vuelve costoso en un mes con muchas series | Una query por request, índice por atleta+fecha y una sola pestaña por vez |
| El contrato se desvía del dashboard existente | Se reutilizan las funciones puras y se cubren con tests comparativos de fórmula, no se crea un segundo cálculo |

## Testing Strategy

- **Dominio:** tabla de casos para período (mes bisiesto, semana que cruza de
  mes/año, día), cada pestaña, `null` vs `0`, empates, series inválidas y
  ponderación por distancia.
- **DAO:** test con Postgres real del repo (mismo patrón que
  `dashboard_dao_test.go`): dos equipos, grupo dejado, `wip`, soft-delete y
  filas de otros corredores.
- **Service:** validación de `period`, `reference_date` y `tab`; default
  `general`; que no acepte ids de corredor.
- **Controller:** ruta y status; `401` sin token; `400` con code; respuesta JSON.
- **Regresión:** los tests de los ocho endpoints existentes se ejecutan sin
  modificar expectativas.
