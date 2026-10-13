# Spec Delta — gamification-competitions

> Delta sobre la capability `gamification-competitions` ya implementada por
> `gamificacion-core` (commit `21a5e84`). El requirement "Cierre manual y winners" se
> MODIFICA para relajar el guard de `end_at` cuando la competencia es de tipo route; el
> resto de los requirements nuevos se declara bajo `## ADDED Requirements`. Al mergear,
> el requirement MODIFICADO "Cierre manual y winners" reemplaza al original del change
> `gamificacion-core` (la capability principal nunca se archivó — convención del repo de
> declarar el requirement completo).

## ADDED Requirements

### Requirement: Instancia sintética de competencia route

El sistema DEBE cumplir lo siguiente (MUST):

- Al crear una competencia `type='route'`, la MISMA transacción del
  `POST /teams/{id}/competitions` crea UNA `session_instance` sintética compartida por
  todos los participantes (no una por competidor): sin día de calendario, name descriptivo
  del estilo "Competencia ruta N", con un único `exercise_instance` sintético de nombre
  "Corrida" (kind `velocidad_distancia`) y su link en `session_exercise_instances`.
- El id de esa instancia se persiste en la columna `competition.route_session_instance_id`
  (BIGINT nullable, FK opaca sin constraint — seteada solo para type route; NULL para
  metric).
- Las filas de feedback del intento (y sus puntos GPS en `workout_feedback_points`) se
  anclan a `assigned_session_id`/`assigned_exercise_id` de esa instancia, de modo que el
  tracking GPS hereda el flujo pre-start → live → resultado del runner service sin
  duplicar infraestructura; los broadcasts de feedback del intento reutilizan el canal
  `session:{route_session_instance_id}` existente.

#### Scenario: Create route crea la instancia sintética

- **WHEN** se crea una competencia route válida
- **THEN** en la misma transacción existen session_instance + exercise_instance "Corrida"
  + session_exercise_instances link, y `competition.route_session_instance_id` referencia
  esa instancia.

#### Scenario: Instancia compartida única

- **WHEN** la competencia route admite N participantes
- **THEN** existe UNA sola session_instance de competencia para todos (no una por
  competidor).

#### Scenario: Create metric no crea instancia

- **WHEN** se crea una competencia `type='metric'`
- **THEN** no se crea instancia sintética y `route_session_instance_id` queda NULL (sin
  filas nuevas en session_instances).

#### Scenario: Sin FK de DB

- **WHEN** se inspecciona el esquema de `competition` tras la migración
- **THEN** `route_session_instance_id` es BIGINT nullable sin FK constraint declarada
  (patrón transparente del repo).

### Requirement: Intake del intento (POST /competitions/{id}/attempts)

El sistema DEBE cumplir lo siguiente (MUST):

- `POST /api/v1/competitions/:id/attempts` disponible, autenticado, en orden:
  `404` si la competencia no existe; `400` ("solo las competencias de ruta admiten
  intentos") si `type != 'route'`; `400` ("la competencia no está activa") si
  `status != 'active'`; `403` si el caller autenticado no figura en
  `competition_participant`; `400` si el caller ya tiene un intento previo en esa
  competencia (runner_session con ese `competition_id` + `athlete_user_id`).
- Fecha/rango: no aplica — route no usa ventana temporal.
- Acción: inserta `runner_session {session_instance_id = route_session_instance_id,
  athlete_user_id = caller, status='wip', competition_id}` SIN pasar por el gate
  presencial (la instancia sintética no tiene día/grupo).
- Responde `201` con `{data: {runner_session_id, session_instance_id, competition_id}}`.
- El finish del intento sigue siendo `PATCH /session-instances/:id/runner` (flujo
  existente, sin cambios en el servicio de feedback); `syncRun` sube los feedback points
  a la instancia sintética sin cambios.

#### Scenario: Happy path de intento

- **WHEN** un participante de una competencia route active abre su intento
- **AND** no tiene intento previo.
- **THEN** responde `201` con `{data: {runner_session_id, session_instance_id,
  competition_id}}` y la runner_session queda creada con status `wip`.

#### Scenario: 404 competencia inexistente

- **WHEN** se intenta abrir un intento sobre un id de competencia inexistente
- **THEN** responde `404` y el DAO de escritura no se toca.

#### Scenario: 400 type no es route

- **WHEN** se abre un intento sobre una competencia `type='metric'`
- **THEN** responde `400` con "solo las competencias de ruta admiten intentos".

#### Scenario: 400 competencia no activa

- **WHEN** la competencia está en `pending_approval`, `finished` o `rejected`
- **THEN** responde `400` con "la competencia no está activa".

#### Scenario: 403 caller no participante

- **WHEN** un usuario autenticado que NO figura en competition_participant abre un intento
- **THEN** responde `403` y no se crea ninguna runner_session.

#### Scenario: 400 doble intento

- **WHEN** el mismo participante intenta abrir un segundo intento para la competencia
- **THEN** responde `400` (la unicidad de un intento por atleta queda garantizada por el
  unique (session_instance_id, athlete_user_id) + la instancia compartida única).

### Requirement: Límites y validación de ruta en el create

El sistema DEBE cumplir lo siguiente (MUST):

- En `POST /teams/{id}/competitions` con `type='route'`, además del shape existente:
  `route_geojson` debe ser un LineString GeoJSON parseable (`routematch.ParseLineString`);
  máximo **500 vértices** (400 "la ruta admite hasta 500 puntos"); longitud Haversine
  total ≤ **100 km** (400 "la ruta no puede superar los 100 km").
- `acceptance_radius_meters` es requerido para route y su rango válido es **[5, 500]**
  metros (400 "el radio de aceptación debe estar entre 5 y 500 metros"); sin default
  backend-mandado (el front sugiere 50).
- La ruta es inmutable post-create: no hay edición de ruta (cambiar la ruta = crear
  competencia nueva).
- Sampleo del matcher sin cambios: `SampleRoute` (step 40 m) sobre el límite 100 km/500
  vértices da ~2500 samples in-memory — sin gate de performance.

#### Scenario: GeoJSON inválido

- **WHEN** el create route envía route_geojson que no parsea como LineString
- **THEN** responde `400`.

#### Scenario: Demasiados vértices

- **WHEN** la ruta tiene 501 vértices
- **THEN** responde `400` con "la ruta admite hasta 500 puntos".

#### Scenario: Ruta demasiado larga

- **WHEN** la longitud Haversine sumada de la ruta supera 100 km
- **THEN** responde `400` con "la ruta no puede superar los 100 km".

#### Scenario: Radio fuera de rango

- **WHEN** el create route envía acceptance_radius_meters = 3 o 600
- **THEN** responde `400` con "el radio de aceptación debe estar entre 5 y 500 metros".

#### Scenario: Radio requerido

- **WHEN** el create route omite acceptance_radius_meters
- **THEN** responde `400` (no hay default backend-mandado).

### Requirement: Frames WS sin canal nuevo para el intento en curso

El sistema DEBE cumplir lo siguiente (MUST):

- El progreso live de competencias se limita a los 2 frames ya implementados:
  `update:competition_progress` al terminar un intento y `control:competition_finished`
  al cerrar. NO se crea canal ni frame nuevo para el intento en curso (leaderboard en
  vivo = deuda/roadmap).
- Al terminar un intento nacido con `competition_id` seteado (intento de ruta vía
  POST attempts), el frame `update:competition_progress {competition_id, user_id}`
  se emite igual por `session:{route_session_instance_id}` — sin código nuevo de
  protocolo.

#### Scenario: Frame al terminar intento de ruta

- **WHEN** el participante TERMINA su intento de ruta (runner_session finished)
- **THEN** el channel `session:{route_session_instance_id}` recibe
  `update:competition_progress` con `{competition_id, user_id}`.

#### Scenario: Sin WS en vivo durante el intento

- **WHEN** el intento está en curso (status wip)
- **AND** no hay broadcast de leaderboard ni de traza GPS.
- **THEN** el front refresca el resultado post-close vía `control:competition_finished`.

## MODIFIED Requirements

### Requirement: Cierre manual y winners

El sistema DEBE cumplir lo siguiente (MUST):

- `PATCH /competitions/{id}/close`: caller creador (trainer o runner) u owner del team.
  SIEMPRE manual; el auto-cierre programado queda como deuda documentada. Solo sobre
  `active`.
- **Guard de ventana ya vencida: NO aplica cuando `type='route'`** (route no tiene
  end_at). Para `type='metric'` con `end_at` seteado, cerrar antes de que la ventana
  termine responde `400` (comportamiento preexistente, sin cambios).
- Al cerrar: status = `finished`, finished_at = now, y winners:
  - type=metric: MAX de la métrica por participante en la ventana
    `[start_at, end_at o now]`; empates exactos → todos ganan.
  - type=route: mejor % de match (matching LAZY ya existente en el close:
    `routeAttempts` → `SampleRoute` + `MatchRatio`); tie-break mayor % → a igual %,
    menor duración del intento → a igual duración exacta, todos ganan. Un intento por
    participante (intake D2 + unique (session_instance_id, athlete_user_id)).
  - Sin intentos → finished sin winner (`winner_user_ids` vacía).
- Ganador/es reciben fila en `gamification_stamp` (type `competition`, source_id
  competencia, source_name "Competencia N", count 1); push best-effort a los
  participantes; unseen por participante (section `competitions`, team_id del team);
  `control:competition_finished` por WS al cerrar.

#### Scenario: Cierre route sin end_at no exige ventana

- **WHEN** el creador cierra una competencia route `active` (que nunca tuvo end_at)
- **THEN** responde `200` con la competencia `finished` y sus winners computados por
  match % de los intentos finished contra la ruta.

#### Scenario: Cierre metric antes de end_at sigue bloqueado

- **WHEN** el creador cierra una competencia metric con `end_at` en el futuro
- **THEN** responde `400` (regresión del guard original).

#### Scenario: Cierre route sin intentos

- **WHEN** el cierre corre sobre una competencia route sin ningún intento
- **THEN** queda `finished` con `winner_user_ids` vacía y sin stamps ni pushes de ganador.

#### Scenario: Matching en el close usando la instancia sintética

- **WHEN** al cerrar hay N intentos finished anclados a la instancia sintética de la
  competencia
- **THEN** el % de match de cada participante se computa de sus GPS points contra los
  samples de la ruta (reuso del flujo existente), sin recalcular datasets previos.

#### Scenario: Detalle route con progreso existente

- **WHEN** el front consulta `GET /competitions/{id}` de una competencia route
- **AND** no se suman métricas ni shape nuevo en este change.
- **THEN** el progreso mantiene el shape existente del detalle (el resultado final se
  refresca post-close vía `control:competition_finished`).
