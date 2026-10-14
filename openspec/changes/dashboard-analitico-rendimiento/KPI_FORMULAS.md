# Fórmulas de KPI por tipificación

Contrato de cálculo del dashboard de rendimiento, para que el front **no las
reimplemente**. Todo lo que está acá está implementado y testeado en
`cmd/api/domains/dashboard/` (funciones puras, sin HTTP ni DB) y alimentado por la
query de `cmd/api/daos/dashboard_dao.go`. Si el front necesita un número que no está
en esta tabla, es un número nuevo: hay que cambiar el backend.

Referencia de diseño: `design.md` D1-D11. Spec: `specs/dashboard-analytics/spec.md`.

## 1. Qué series entran al cálculo

Una fila de `workout_feedback` es una **serie**. Entra al cálculo si cumple las dos
cosas:

1. Es **bloque principal**: `session_exercise_instances.role = 'main'` (D9). El rol
   vive en el link, no en el feedback ni en la instancia — sin ese JOIN un
   calentamiento entra al dashboard como principal.
2. Su sesión no está `wip`: `runner_session.status IN ('finished','interrupted')` **o
   no hay fila `runner_session`** (D7). El `LEFT JOIN` a `runner_session` no es
   negociable: el feedback se crea sin esa fila, y con `INNER JOIN` todo el
   histórico previo al feature desaparecería sin que ningún contador lo señale.

El objetivo se mide contra el **valor congelado de la `exercise_instance`** —`minutes`
y `distance_m` de esa fila—, nunca contra el objetivo vigente del catálogo (D3): si
el entrenador cambia el objetivo después, el histórico ya reportado no se reescribe.

## 2. Eje del objetivo según el tipo (D1)

| `kind` | Eje | Qué necesita |
|---|---|---|
| `liviano` | distancia **o** tiempo, el que esté cargado | cualquiera de los dos |
| `elongacion` | tiempo | `minutes` |
| `ritmo_continuo` | distancia **y** tiempo | los dos |
| `velocidad_tiempo` | tiempo | `minutes` |
| `velocidad_distancia` | distancia | `distance_m` |

`velocidad` no existe en el catálogo vigente; queda por compatibilidad de filas
viejas.

La **velocidad nunca se lee**: `speed_kph` es `numeric(4,1)` (techo 999,9,
resolución 0,1 km/h) y siempre se **deriva** de distancia y tiempo
(D2): `speed_kph = distance_m / 1000 / (duration_ms / 3_600_000)`.

## 3. Series válidas (D6)

| `kind` | Válida si |
|---|---|
| `liviano` | tiene tiempo **o** distancia |
| `elongacion`, `velocidad_tiempo` | tiene tiempo |
| `ritmo_continuo` | tiene tiempo **y** distancia |
| `velocidad_distancia` | tiene distancia |
| kind legacy o desconocido | nunca (no es calculable: no rompe, no entra) |

El tiempo logrado es `COALESCE(active_duration_ms, duration_ms)`: se usa el tiempo
activo porque comparar contra el reloj corrido penaliza las pausas de semáforo
(D4). Si el tiempo activo no es usable (> 0), la serie cae al fallback y queda
marcada `degraded`.

Las inválidas **no entran** a promedios, cumplimiento ni rankings, pero se cuentan
en `data_quality.invalid_series`: descartarlas en silencio haría que el dashboard
mintiera ("60 % de cumplimiento" sobre 6 de 10 series parece completo).

## 4. Cumplimiento (D5)

Tolerancia **±5 %** por defecto, ajustable por query entre `0.01` y `0.20`.

`isCompliant` devuelve **true si se cumple cualquiera** de los criterios que el
`kind` declara:

- `ritmo_continuo` exige tiempo **y** distancia: correr más rápido quedándose corto
  de distancia **no** es cumplimiento.
- Pasarse **cuenta como cumplimiento** (castigar a quien se pasó sería absurdo).
- `compliance_rate` es un **porcentaje 0-100**, no una fracción, y sale desde 1
  sola serie (es un conteo, no una tendencia).

`delta_pct` invierte el signo en el eje temporal: bajar el tiempo es mejorar. 240 s
de objetivo con 230 s logrados es **+4,2 %**, no −4,2 %.

## 5. Tendencia y consistencia (D11)

- `trend_pct`: tercil más reciente contra el más antiguo. Con **menos de 3** series
  devuelve `null` y marca `insufficient_data` — un porcentaje sobre dos puntos es
  ruido con formato de dato.
- `consistency.cv`: coeficiente de variación de la **velocidad** de las series
  válidas (`σ(v)/μ(v)`, en km/h), no del ritmo. `null` con menos de 2 valores o con
  μ = 0.

## 6. Bandas por serie (3.4b)

`band(kind, targets, achieved, tolerance)` clasifica cada serie contra su objetivo:
`below` / `near` / `above` / `unknown`. El caso `unknown` (objetivo cargado pero el
campo del eje de la serie viene `null`) **no** es lo mismo que el `null` de
`has_target: false`: el primero cuenta serie, el segundo no cuenta banda.

## 7. `data_quality` (los seis contadores)

| Campo | Qué cuenta |
|---|---|
| `invalid_series` | series válidas que no son calculables (se excluyen del cálculo) |
| `degraded_time` | series cuyo tiempo vino del fallback (`active_duration_ms` no usable) |
| `unknown_status` | `completion_status` fuera de `{completa, partial, skipped}`, o `null` |
| `open_sessions` | sesiones `(sesión, corredor)` en `wip` — **segunda query**, el WHERE de series ya las sacó |
| `legacy_sessions` | series válidas sin respaldo completo: sin fila `runner_session`, o de una `ExerciseInstance` sin `source_exercise_id` resuelta por nombre |
| `insufficient_data` | menos de 3 series válidas con velocidad en el conjunto filtrado |

`unknown_status` **no descarta** la serie: `completion_status` es texto libre sin
validar, y descartar datos por un string no controlado sería peor que incluirlos
(D10).

Los cinco primeros vienen en **todas** las respuestas de métricas aunque valgan
cero, para que el cliente distinga "no hay datos" de "los datos están limpios".
`legacy_sessions` es informativo, no un defecto: el front lo muestra como nota al
pie, no como advertencia.

## 8. El seed es la referencia

`cmd/api/scripts/seed_dashboard/` genera un dataset que produce estos seis
contadores con valores conocidos y verificables (`RUNBOOK.md` los lista), con
arquetipos de corredor, progresión semanal y feedback contextual. Es el dataset
contra el que developing del front.