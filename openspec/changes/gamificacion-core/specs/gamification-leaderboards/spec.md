# Spec Delta — gamification-leaderboards

## ADDED Requirements

### Requirement: Denormalización de pace y peak_speed en feedback

El sistema DEBE cumplir lo siguiente (MUST):

- Columnas nuevas nullable en `workout_feedback`: `pace_sec_per_km` (float, menor mejor) y
  `peak_speed_kph` (float).
- Al subir points de un feedback (batch con count > 0), el backend recalcula de forma
  idempotente: `pace_sec_per_km = duration_ms / distance_meters` (segundos por km) y
  `peak_speed_kph = máximo de velocidades instantáneas entre segmentos de points
  consecutivos`.
- Sin backfill histórico (los feedbacks viejos quedan con NULL y no puntúan para
  peak_speed).
- Las columnas NO se exponen en el DTO de feedback (decisión interna: solo alimentan el
  leaderboard).

#### Scenario: Subida de points con GPS

- **WHEN** se sube un batch de points a un feedback con duration_ms y distance_meters
- **THEN** el backend persiste pace = duration_ms/distance_meters y peak = max de las
  velocidades instantáneas entre segmentos de points consecutivos.

#### Scenario: Recompute idempotente

- **WHEN** se vuelve a subir (o resubir) points del mismo feedback
- **THEN** las columnas se recalculan y quedan con el mismo valor determinista derivado de
  los datos, sin duplicar ni acumular.

#### Scenario: Feedback sin distance

- **WHEN** el feedback tiene duration_ms pero distance_meters en 0 o NULL
- **THEN** `pace_sec_per_km` queda NULL (evita división por cero); si el GPS no alcanza,
  `peak_speed_kph` también NULL.

#### Scenario: DTO no expone columnas internas

- **WHEN** el frontend consume answers de feedback existentes
- **THEN** la respuesta NO incluye pace_sec_per_km ni peak_speed_kph; quedan solo para el
  leaderboard.

### Requirement: Consulta de leaderboard por equipo

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /teams/{id}/leaderboard?metric=&period=&from=&to=` disponible.
- Authorization del caller: miembro de algún grupo del team O owner del team; otro caso
  responde `403` (DAO no tocado).
- `metric` valida la whitelist `distance|duration|pace|peak_speed|sessions_count|
  attendance_count`; otro valor responde `400`.
- `period` valida la whitelist `week|month|year|custom|all`; otro valor responde `400`.
- `from`/`to` en YYYY-MM-DD: `custom` exige ambos y `from <= to` (400 si falta uno o violan
  el orden); en los otros periods se derivan de la fecha "hoy" en TZ
  `America/Argentina/Cordoba`.
- Métricas del período:
  - distance = SUM distance_meters.
  - duration = SUM duration_ms (reloj total).
  - pace = SUM duration / SUM distance del período (razón agregada; NO promedio de ritmos).
  - peak_speed = MAX peak_speed_kph.
  - sessions_count = COUNT de runner sessions finished en el período.
  - attendance_count = COUNT de asistencias confirmadas en el período.
- Solo corredores puntúan: el owner/entrenador del team no genera fila propia.
- Aggregation sin N+1: 1 query GROUP BY user_id + batch de names/avatar.

#### Scenario: Board de semana por distance

- **WHEN** el miembro consulta `metric=distance&period=week`
- **THEN** responde `200` con las filas del período semanal, cuyo rango se deriva de hoy en
  TZ Cordoba.

#### Scenario: Metric inválida

- **WHEN** se envía `metric=hour`
- **THEN** responde `400` sin ejecutar la consulta.

#### Scenario: Custom incompleto o inconsistente

- **WHEN** se envía `period=custom` sin `from` o sin `to`, o con `from > to`
- **THEN** responde `400`.

#### Scenario: Ajeno al equipo

- **WHEN** un usuario sin membership en ningún grupo del team y sin ser owner consulta el
  board
- **THEN** responde `403` sin tocar el DAO.

#### Scenario: Owner sin fila propia

- **WHEN** el owner del team consulta el board
- **THEN** las filas solo contienen corredores con actividad del período; el owner no tiene
  fila propia aunque registre entrenamientos en el sistema.

#### Scenario: viewer_rank fuera del top 5

- **WHEN** el caller corredor queda en la posición 9 del ranking
- **THEN** `rows` contiene el top 5 y `viewer_rank` trae su posición 9 con su valor.

### Requirement: Ranking, desempate y record

El sistema DEBE cumplir lo siguiente (MUST):

- `rows` devuelve el top 5 con shape `{user_id, name, avatar_url, value, rank}`; ranks
  comparten posición a igual valor exacto y la siguiente posición saltea N.
- Desempate general: a igual valor, gana quien usó MENOS entrenamientos que contribuyeron al
  valor (COUNT de actividades con métrica > 0 en el período).
- Desempate específico de `sessions_count`: a igual valor (ya con la misma cantidad de
  sesiones), gana quien empezó antes (la start_date de la última de sus sesiones es
  anterior).
- Desempate específico de `attendance_count`: gana quien registró la última asistencia que
  le alcanzó con fecha más temprana.
- `record` incluido SOLO con `period=all`: `{user_id, name, value, achieved_at}` del mejor
  valor histórico del team para la métrica (para `pace` el MEJOR ritmo es el MÁS BAJO, no un
  máximo); achieved_at = session_date de la fila que alcanzó el valor — para métricas SUM
  (distance/duration) es la última fila contribuyente — y a empate la más reciente.

#### Scenario: Empate comparte rank

- **WHEN** dos corredores suman exactamente el mismo valor del período
- **THEN** ambos reciben la misma posición (por ejemplo rank 1) y la fila siguiente saltea
  a la 3.

#### Scenario: Desempate por menos entrenamientos

- **WHEN** dos corredores suman el mismo distance y uno lo hizo en 2 sesiones y el otro en
  5
- **THEN** el de 2 sesiones queda arriba en el ranking.

#### Scenario: Desempate de attendance_count

- **WHEN** dos corredores empatan attendance_count con la misma cantidad de asistencias que
  contribuyeron
- **THEN** queda arriba quien registró en fecha más temprana la última asistencia que le
  alcanzó para llegar a ese valor.

#### Scenario: Record solo en all

- **WHEN** se consulta con `period=week`
- **THEN** la respuesta NO incluye `record`; solo `period=all` lo incluye.

#### Scenario: Record con empate histórico

- **WHEN** dos corredores comparten el máximo histórico del team
- **THEN** `record` apunta al corredor de la fila con achieved_at más reciente (la fecha de
  sesión que alcanzó el valor).
