# Design

## Context

Estado actual que condiciona el diseño. La motivación está en `proposal.md` — Why.

**Lo que ya existe y se reusa, no se reimplementa:**

- La capa de autorización: `isGroupOwner` (`calendar_service.go:359`, `team.OwnerID == callerID`) e `isGroupOwnerOrMember` (`:380`). El dashboard reusa esos criterios, no crea un modelo de permisos nuevo.
- El scope de grupos del entrenador: `groupDao.FindByOwnerID(userID)` (`group_dao.go:95`).
- Los tres middlewares actuales (`CORSMiddleware`, `SetRequestID`, `AuthMiddleware` en `url_mappings.go:8-9,51`).

**La query core y sus índices — verificados uno por uno:**

```sql
FROM group_calendar_days gcd
JOIN workout_feedback wf           ON wf.assigned_session_id = gcd.session_instance_id
JOIN exercise_instances ei         ON ei.id = wf.assigned_exercise_id
JOIN session_exercise_instances sei ON sei.session_instance_id  = wf.assigned_session_id
                                AND sei.exercise_instance_id = wf.assigned_exercise_id
LEFT JOIN runner_session rs        ON rs.session_instance_id = wf.assigned_session_id
                                AND rs.athlete_user_id     = wf.athlete_user_id
WHERE gcd.group_id = $1 AND gcd.date BETWEEN $2 AND $3
  AND sei.role = 'main' AND wf.deleted_at IS NULL
  AND (rs.status IN ('finished','interrupted') OR rs.status IS NULL)
```

**El `LEFT JOIN` a `runner_session` es deliberado y no es negociable:

`workout_feedback` se crea **sin** fila `runner_session` — el servicio de feedback no la exige. Con `INNER JOIN`, todo el feedback cargado antes del feature de registro de sesión desaparece del dashboard, y como no hay contador que lo señale, el síntoma es "el dashboard está vacío y no sé por qué". Es la peor forma de perder datos.

La condición del `WHERE` tiene que caber en las dos ramas: `rs.status IS NULL` **no** significa "sin sesión sin terminar", significa "sin fila de registro", y esas series cuentan. `open_sessions` mide lo otro.

| Salto | Índice existente |
|---|---|
| `gcd.group_id + date` | `idx_group_calendar_day_group_date` — UNIQUE |
| `gcd.session_instance_id` | simple |
| `wf.assigned_session_id` | `idx_feedback_session_exercise` p1 |
| `wf.athlete_user_id + session_date` | `idx_feedback_athlete_date` |
| `sei.exercise_instance_id` | simple |
| `rs.session_instance_id + athlete_user_id` | `uq_runner_session_session_athlete` (sirve el join; **no** el filtro por corredor solo, de ahí el índice nuevo) |

**El schema ya está bien indexado para esta consulta.** No se está parcheando un problema de performance; se está aprovechando lo que ya estaba.

**Restricciones heredadas:**

- El modelo lógico es inmutable. No hay columna de objetivo, ni de ritmo, ni de calorías.
- `workout_feedback.completion_status` es `VARCHAR(20)` sin enum, sin CHECK y sin validación (`workout_feedback.go:37`; el service lo pasa recto en `:150` y `:632-633`).
- `speed_kph` es `numeric(4,1)`: tope 999.9, resolución 0.1 km/h.
- `isCalendarDayClosed` (`calendar_service.go:399-404`) devuelve `true` para **toda** fecha pasada. Sin excepción, el seed de histórico es imposible vía API.

## Goals / Non-Goals

**Goals:**

- Que las fórmulas de KPI sean **funciones puras testeables en aislamiento**, sin HTTP ni DB — es el mismo criterio que aplicó el frontend en `utils/exercise-field-matrix.js`.
- Que el scope se resuelva una vez por request y se pase ya resuelto a los DAOs, de modo que ningún DAO pueda aceptar un `group_id` crudo.
- Que el seed sea **reutilizable** para otro equipo/grupo cambiando dos variables.

**Non-Goals:**

- No se crea un modelo de permisos nuevo ni una tabla de precomputados.
- No se toca el service de feedback ni su contrato actual.
- No se repara el índice faltante de `exercises(owner_id)`: es un bug del catálogo y mezclarlo acá metería dos problemas en una PR.

## Decisions

### D1: El eje del objetivo lo declara el `kind`, no una columna nueva

El modelo no tiene concepto de objetivo y es inmutable. La opción era reinterpretar `minutes`/`distance_m` como objetivo según el tipo:

| `kind` | eje | velocidad |
|---|---|---|
| `liviano` | distancia **o** tiempo (el que esté) | derivada si hay ambos |
| `elongacion` | tiempo | no aplica |
| `ritmo_continuo` | distancia **y** tiempo | **derivada** |
| `velocidad_tiempo` | tiempo | derivada si hay distancia |
| `velocidad_distancia` | distancia | derivada si hay tiempo |

**Alternativa descartada:** agregar una columna `target_*`. Rompe la inmutabilidad del modelo y obligaría a migrar datos que hoy significan otra cosa.

**Consecuencia asumida:** cambia la semántica de datos ya cargados. "Distancia 10 km · 60 min" pasa a significar *correr 10 km en 60 min*. Es aceptable porque los objetivos son recientes y el modelo nuevo de tipos (PR #99) aún no tiene volumen.

### D2: `speed_kph` nunca es eje, siempre derivado

`speed_kph` es `numeric(4,1)`. Un objetivo de 4:35/km son 13.04 km/h: **no se puede representar**. Como eje, pondría un techo de resolución justo donde el corredor la necesita.

**Alternativa descartada:** cambiar el tipo de la columna a `numeric(6,3)`. Es un cambio de schema, prohibido acá.

Como salida derivada, la pérdida de resolución no afecta: se calcula desde `duration_ms` y `distance_m`, que son `BIGINT` y `FLOAT`.

### D3: El objetivo se mide contra el valor congelado de la instancia

`exercise_instances` es una copia del catálogo hecha en el stamp (`calendar_service.go:659-676`). Cada día se juzga contra lo que el corredor **intentaba** ese día.

**Alternativa descartada:** leer el objetivo del catálogo vigente. Si el entrenador lo cambia, reescribiría el historial ya reportado — el dashboard passería a descalificar al corredor por un objetivo que nunca tuvo.

### D4: `active_duration_ms` primero, `duration_ms` de fallback

Hay tres fuentes de tiempo redundantes y nada valida que sean coherentes (lo único chequeado es `ended_at >= started_at`, `workout_feedback_service.go:536-538`).

Se usa `active_duration_ms` porque comparar contra un ritmo objetivo con el reloj corrido penaliza las pausas en semáforos. Las series que caen al fallback se marcan `degraded` y se cuentan aparte: el número entra en los KPIs pero la degradación queda visible.

**Alternativa descartada:** usar siempre `duration_ms`. Es más simple, pero produce ritmos artificialmente lentos en cualquier sesión con pausas, que en running es lo normal.

### D5: Tolerancia ±5% por defecto, ajustable por query

Sin tolerancia, "cumplió el objetivo" es una línea binaria donde el 5.001 % falla. Con ±5 % se absorbe el ruido de medición sin perder la señal.

Pasarse **cuenta como cumplimiento**: castigar a quien se pasó sería absurdo.

Rango acotado 0.01–0.20 para que nadie lo use como atajo de "no compares nada".

### D6: Las series no calculables se excluyen pero se cuentan

Una serie sin el dato que su tipo exige no puede entrar a un promedio. Pero descartarla en silencio haría que el dashboard mintiera: "60 % de cumplimiento" sobre 6 de 10 series parece completo.

Se excluye del cálculo y se cuenta en `data_quality.invalid_series`.

### D7: Solo sesiones `finished` / `interrupted`

Un `wip` con 2 de 5 series reportadas aparecería como "cumplimiento bajo" cuando en realidad es una sesión a medias. Las `wip` van a `data_quality.open_sessions`.

### D8: Buckets por `session_date`, nunca por `started_at`

`session_date` es una columna `date` — fecha lógica parseada de `"YYYY-MM-DD"` (`parseSessionDate`, `workout_feedback_service.go:463-472`). Agrupar por ella no tiene líos de timezone.

Agrupar por `started_at` (timestamptz) movería las sesiones de 23:30 al día equivocado para medio mundo. Es un bug invisible en desarrollo (una máquina, un timezone) y visible en producción.

### D9: El rol exige JOIN, y no es opcional

`workout_feedback.assigned_exercise_id` apunta a `exercise_instances`, **no** a `session_exercise_instances` — el rol vive en el link. Para filtrar por rol `main` hay que unir por `(session_instance_id, exercise_instance_id)`.

Que el gráfico sea "solo de bloque principal" es justamente filtrar por rol. Sin el JOIN, un calentamiento reportado entra al dashboard como principal.

### D10: `completion_status` desconocidos entran a los KPIs

El campo es texto libre sin validación. Descartar series por un string no controlado sería peor que incluirlas: el backend no garantiza el vocabulario y un cliente futuro podría escribir cualquier cosa.

Se reconocen `completa`/`partial`/`skipped`; lo demás (o `null`) cuenta como `unknown`, se registra en `data_quality.unknown_status` y **entra** al cálculo.

**Alternativa descartada:** agregar constantes + validación en el service. Sería lo correcto a futuro, pero es un cambio en el service del feedback fuera del alcance del dashboard.

### D11: Sin término "porcentaje de tendencia" con pocos datos

`trend_pct` compara el tercil más reciente contra el más antiguo. Con menos de 3 series se devuelve `null` y se marca `insufficient_data`: un porcentaje sobre dos puntos es ruido con formato de dato.

`compliance_rate` sí se muestra desde 1 serie — es un conteo, no una tendencia.

### D12: Cotas de query en lugar de rate limiter

El pedido original (`FASE 2.1`) pedía rate limiting. El backend **no tiene ninguno** — los únicos middlewares son CORS, RequestID y Auth. Agregarlo es dependencia nueva (`golang.org/x/time/rate`) o patrón nuevo, y `FASE 1.3` prohíbe ambos.

Se acota el peor caso por request: `from` obligatorio, rango máximo 2 años, `page_size` máximo 100. Un rate limiter real queda como decisión de plataforma.

### D13: Code-first, no un `.yaml` escrito a mano

El backend genera el OpenAPI con `swag init` desde anotaciones de los controllers (`SETUP.md:116`). Escribir un `.yaml` a mano **y** agregar anotaciones deja dos fuentes de verdad que divergen sin avisar, y el `swag init` regenera el suyo sin detectar el choque.

Este documento y las specs son el artefacto compartido; el OpenAPI canónico lo genera `swag init` desde el código, como el resto del repo.

### D14: El seed se habilita con opt-in explícito de stage, no con una lista de environments prohibidos

`SEEDING_ENABLED=true` **no basta**: el seed solo corre con `config.IsLocalStage()` (el flag `--stage=local`).

La alternativa obvia —"coro salvo en producción", chequeando `config.IsProduction()`— **es insuficiente acá**, y por dos razones concretas del repo:

1. **El repo tiene tres conceptos de stage, no dos.** `IsProduction()` mira que la env var `environment` contenga `"prod"`. `IsProductionStage()` mira el flag `--stage=production`. `IsLocalStage()` mira `--stage=local`. **Staging no es ninguno de los dos primeros**: no contiene `"prod"` en la env var, y sin `--stage=production` arranca como testing. Staging es un entorno desplegado real que por defecto apunta al proyecto de Supabase de testing **compartido**.
2. **Una lista de prohibidos envejece.** Cada environment nuevo que se agregue al repo nace permitido por default. Pedir `--stage=local` invierte el default: un environment nuevo nace **prohibido**, y hay que habilitarlo a propósito.

Además, `SEEDING_ENABLED` requiere **las dos** condiciones, no una:

```
SEEDING_ENABLED=true  AND  config.IsLocalStage()
```

Con una sola, un deploy de producción con la variable copiada del `.env.example` borraría sesiones reales.

El rechazo es un **panic al arrancar**, no un `log` de advertencia: un seed que corre a medias en un entorno compartido es peor que un proceso que no levanta. El mensaje nombra el stage detectado, para que el error sea accionable sin abrir el código.

### D15: Solo 2 índices, y el seed no valida performance

Los índices que realmente faltan:

| Índice | Por qué |
|---|---|
| `idx_runner_session_athlete (athlete_user_id)` | Solo existe el **prefijo** del UNIQUE `(session_instance_id, athlete_user_id)`. La vista de corredor filtra por corredor sin saber la sesión → seq scan |
| `idx_feedback_session_date (assigned_session_id, session_date) WHERE deleted_at IS NULL` | El dashboard filtra por sesión **y** rango de fechas; el índice vigente no trae la fecha, así que el rango se filtra en memoria |

`CREATE INDEX CONCURRENTLY` cumple "sin downtime". Es viable porque la función de inicialización **no abre transacciones** — los `db.Exec` corren en autocommit. **No envolver en `db.Transaction` sin quitarlo**: `CONCURRENTLY` está prohibido dentro de un bloque transaccional. Si falla a mitad deja un índice `INVALID`, que `IF NOT EXISTS` no reintenta — hay que dropearlo.

**No se agrega** `exercises(owner_id)` (bug del catálogo, otra PR) ni `exercise_instances(kind)` (el filtro por tipología corre después del join, sobre un set ya acotado al grupo).

**Expectativa realista:** el seed deja `workout_feedback` en miles de filas — Postgres responde eso sin ningún índice. Los objetivos de p95 se validan contra volumen de producción, no contra el seed.

## Risks / Trade-offs

| Riesgo | Mitigación |
|---|---|
| **El JOIN por rol se olvida** en algún endpoint y entran calentamientos | El rol va en un filtro compartido, no en cada query. Cubierto por un test que mete un `warmup` con series cargadas y verifica que no aparece |
| **Regresión de semántica de D1** — datos viejos significando otra cosa | El cambio de tipos de ejercicios (PR #99) migró las filas; el volumen es acotado y el dashboard es nuevo |
| **`completion_status` desconocido entra al cálculo** y distorsiona | Contado en `data_quality.unknown_status`; visible en pantalla, no escondido |
| **`CONCURRENTLY` deja índice `INVALID`** si falla a mitad | Runbook de rollback con el `DROP`; el índice `INVALID` no lo usa el planner pero ocupa disco |
| **Scope mal filtrado** = fuga de datos de otros equipos | El scope viaja en el `WHERE`; ningún DAO acepta un id crudo. Test de integración: propio → 200, ajeno → 403 |
| **`EXPLAIN` no demuestra nada con el volumen del seed** | Se documenta el plan esperado por estructura de índices, y el p95 se mide contra producción |
| **El formateo de export vive en el front** y puede divergir de la pantalla | Se arma desde la misma respuesta que la pantalla, no desde un endpoint aparte |
