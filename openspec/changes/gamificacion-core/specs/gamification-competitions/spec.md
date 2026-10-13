# Spec Delta — gamification-competitions

## ADDED Requirements

### Requirement: Modelo de competencias y participation

El sistema DEBE cumplir lo siguiente (MUST):

- Tabla `competition`: id, team_id, group_id nullable, type varchar ('metric'|'route'),
  metric nullable, start_at, end_at nullable, route_geojson nullable (GeoJSON LineString),
  acceptance_radius_meters nullable, created_by_user_id, created_by_role varchar
  ('trainer'|'runner'), status varchar ('pending_approval'|'active'|'finished'|'rejected'),
  approved_by_user_id nullable, rejection_reason nullable, finished_at nullable. Sin campo
  title (los pushes usan texto genérico).
- Tabla `competition_participant`: competition_id, user_id. PK compuesta; index user_id.
- Tabla `competition_winner`: competition_id, user_id. PK compuesta; index user_id (el
  empate "todos ganan" inserta N filas).
- `runner_session` suma la columna nullable `competition_id` int64, FK opaca (BIGINT > 0 sin
  constraint de DB, patrón transparent igual que `attendances.training_session_id`).

#### Scenario: Alta con participantes

- **WHEN** se crea una competencia válida con 3 participantes
- **THEN** se persiste la fila de competition y 3 filas en competition_participant.

#### Scenario: Winner múltiple por empate

- **WHEN** al cerrar hay empate exacto entre 2 participantes
- **THEN** competition_winner queda con 2 filas para esa competencia.

#### Scenario: FK opaca sin constraint

- **WHEN** el esquema de la tabla se inspecciona tras la migración
- **THEN** `runner_session.competition_id` es BIGINT nullable sin FK constraint declarada
  (patrón transparent del repo).

### Requirement: Creación y moderación de competencias

El sistema DEBE cumplir lo siguiente (MUST):

- `POST /teams/{id}/competitions` disponible. El creador entrenador (owner del team) crea
  con status `active` directo; el creador corredor (miembro del team) crea con status
  `pending_approval`.
- Validaciones 400: type=metric exige metric y end_at posterior a start_at; type=route exige
  route_geojson y acceptance_radius_meters > 0; participantes al menos 2; todos los
  participantes miembros del team; sin incluir al trainer entre los participantes.
- `PATCH /competitions/{id}/approve` y `PATCH /competitions/{id}/reject` `{reason?}`: caller
  owner del team (o el trainer creador, caso defensivo); solo sobre `pending`;
  approve → `active` (+ push al creador); reject → `rejected` con reason opcional (+ push al
  creador).
- `DELETE /competitions/{id}/cancel`: solo el creador corredor, solo sobre `pending`; borra
  físicamente la competencia junto con sus participantes.

#### Scenario: Entrenador crea activa

- **WHEN** el owner del team crea una competencia metric válida
- **THEN** responde `200` con la competencia en status `active` (shape `CompetitionDetailResponse`, como el resto de las mutaciones).

#### Scenario: Corredor propone

- **WHEN** un corredor miembro del team crea una competencia válida
- **THEN** la competencia queda en status `pending_approval` esperando moderación.

#### Scenario: Validación route incompleta

- **WHEN** se crea una competencia route sin route_geojson o con
  acceptance_radius_meters <= 0
- **THEN** responde `400`.

#### Scenario: Participantes insuficientes o ajenos

- **WHEN** se crea una competencia con 1 solo participante, o con un participante que no es
  miembro del team
- **THEN** responde `400`.

#### Scenario: Trainer entre los participantes

- **WHEN** la creación incluye al owner del team (trainer) en la lista de participantes
- **THEN** responde `400` (los entrenadores no compiten).

#### Scenario: Aprobación de propuesta

- **WHEN** el owner realiza approve sobre una competencia en pending_approval
- **THEN** la competencia pasa a `active`, y el creador recibe un push best-effort avisando
  la aprobación.

#### Scenario: Rechazo con motivo

- **WHEN** el owner realiza reject con `reason` sobre una propuesta del corredor
- **THEN** la competencia pasa a `rejected` con el reason persistido y el creador recibe
  un push best-effort.

#### Scenario: Cancelación por el creador corredor

- **WHEN** el corredor creador cancela su propuesta aún pendiente
- **THEN** se borra físicamente la fila de competition y sus filas de
  competition_participant.

#### Scenario: Moderación solo sobre pendientes

- **WHEN** el owner intenta approve sobre una competencia que ya está `active` o
  `finished`
- **THEN** responde `400` (la transición de estado es inválida desde pending).

### Requirement: Consulta de competencias (listado y detalle con progreso)

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /teams/{id}/competitions` disponible para miembro de un grupo del team u owner del
  team (403 otro caso), con filtros opcionales `status` y `group_id`; status fuera de la
  whitelist responde `400`. Embebe `has_unseen` de la sección competitions bajo el team_id.
- Shape de competencia: `{id, team_id, group_id, type, metric?, start_at, end_at?, route?,
  acceptance_radius_meters?, participants: [] (user_ids), created_by_user_id,
  created_by_role, status, approved_by_user_id?, rejection_reason?, winner_user_ids: [],
  finished_at?}`.
- `GET /competitions/{id}` disponible para miembro del team u owner; 404 si no existe.
  Detalle con progreso en vivo por participante: type=metric → acumulado de la métrica de
  cada participante dentro de `[start_at, end_at o now]`; type=route → mejor % de match de
  cada participante hasta ahora.
- El progreso se computa on-demand en la consulta; NO hay broadcast WS en ese flujo.

#### Scenario: Listado con filtro de status

- **WHEN** el miembro consulta el listado con `status=active`
- **THEN** responde solo las competencias activas del team.

#### Scenario: Listado con filtro de grupo

- **WHEN** el miembro consulta el listado con `group_id` de un grupo del team
- **THEN** responde solo las competencias de ese grupo.

#### Scenario: Novedad embebida en el listado

- **WHEN** la respuesta del listado se arma para un caller con unseen rows de la sección
  competitions con ese team_id
- **THEN** la respuesta incluye `has_unseen: true`.

#### Scenario: Detalle metric con acumulado

- **WHEN** se consulta el detalle de una competencia metric activa
- **THEN** cada participante muestra el acumulado de su métrica desde start_at hasta ahora.

#### Scenario: Detalle route con mejor match

- **WHEN** se consulta el detalle de una competencia route activa
- **THEN** cada participante muestra su mejor % de match hasta ahora (el matching corre
  on-demand como en D10, sin persistirlo).

#### Scenario: Consulta fuera del team

- **WHEN** un usuario sin membership en el team consulta el listado o el detalle
- **THEN** responde `403` (o `404` si la competencia no existe) sin tocar el DAO.

### Requirement: Sesiones de corredor asociadas a competencia

El sistema DEBE cumplir lo siguiente (MUST):

- `POST /session-instances/:id/runner` acepta `competition_id` opcional en el body. Si NO
  viene, el flujo queda byte-idéntico al actual (única modificación aditiva a un endpoint
  existente).
- Si viene: la competencia debe estar `active`, el caller debe ser participante, y la
  fecha/hora actual debe estar dentro del rango activo `[start_at, end_at si set]`; otra
  cosa responde `400`.
- La marca se persiste en `runner_session.competition_id` (FK opaca, D8) al crear el intento.

#### Scenario: Intento tagueado a competencia

- **WHEN** el participante crea su intento corredor con `competition_id` siendo competencia
  activa dentro de rango
- **THEN** la sesión runner queda asociada a la competencia.

#### Scenario: Caller no participante

- **WHEN** un corredor fuera de la lista de participantes envía `competition_id`
- **THEN** responde `400` y la sesión no se crea.

#### Scenario: Competencia fuera de rango

- **WHEN** el intento se crea antes de `start_at` o después de `end_at`
- **THEN** responde `400`.

#### Scenario: Competencia no activa

- **WHEN** se envía `competition_id` de una competencia en `pending` o `finished`
- **THEN** responde `400`.

#### Scenario: Sin el campo no cambia nada

- **WHEN** el frontend crea un intento corredor sin `competition_id`
- **THEN** el comportamiento y la respuesta son idénticos a los de hoy (regresión cubierta
  por los tests preexistentes del endpoint).

### Requirement: Cierre manual y winners

El sistema DEBE cumplir lo siguiente (MUST):

- `PATCH /competitions/{id}/close`: caller creador (trainer o runner) u owner del team.
  SIEMPRE manual; el auto-cierre programado queda como deuda documentada. Solo sobre
  `active` y, con `end_at` seteado, solo cuando la ventana ya terminó (cerrar antes de
  `end_at` responde `400`).
- Al cerrar: status = `finished`, finished_at = now, y winners:
  - type=metric: MAX de la métrica por participante en la ventana
    `[start_at, end_at o now]`; empates exactos → todos ganan (`winner_user_ids` con N
    ids).
  - type=route: mejor % de match; tie-break mayor % → a igual %, menor duración del
    intento → a igual duración exacta, todos ganan.
  - Sin intentos → finished sin winner (`winner_user_ids` vacía).
- Ganador/es reciben fila en `gamification_stamp` (type `competition`, source_id
  competencia, source_name "Competencia N", count 1).
- push best-effort a los participantes con `data.route` placeholder.
- Inserta unseen por participante con section `competitions` y team_id del team.
- `control:competition_finished` por WS al cerrar (ver requirement de WS).

#### Scenario: Cierre metric con empate

- **WHEN** dos participantes acumulan exactamente el mismo máximo de la métrica en la
  ventana
- **THEN** ambos quedan en winner_user_ids con filas en competition_winner y ambos reciben
  su stamp de competencia.

#### Scenario: Cierre route con tie-break de duración

- **WHEN** dos participantes alcanzan el mismo % de match y uno lo hizo en una duración
  menor
- **THEN** gana el del intento más corto; a igual duración exacta ganan todos.

#### Scenario: Cierre sin intentos

- **WHEN** el cierre corre sobre una competencia sin ningún intento registrado
- **THEN** la competencia queda `finished` con winner_user_ids vacía y sin stamps ni pushes
  de ganador.

#### Scenario: Cierre antes de que termine la ventana

- **WHEN** el creador intenta cerrar una competencia active con `end_at` en el futuro
- **THEN** responde `400` (el cierre es manual pero exige la ventana vencida).

#### Scenario: Stamp al ganador

- **WHEN** la competencia se cierra con un ganador único
- **THEN** el ganador recibe una fila en gamification_stamp (type competition, source_id
  competencia, source_name "Competencia N", count 1).

#### Scenario: Unseen a participantes del cierre

- **WHEN** el cierre termina y persiste el resultado
- **THEN** cada participante de la competencia recibe una fila de unseen section
  competitions con el team_id del team (punto rojo en la sección).

#### Scenario: Cierre por quien no corresponde

- **WHEN** un corredor miembro que no es el creador intenta cerrar la competencia
- **THEN** responde `403` (solo creador u owner del team).

### Requirement: Route matching server-side

El sistema DEBE cumplir lo siguiente (MUST):

- El route matching corre ÚNICAMENTE al computar winners (close) o en el detalle on-demand;
  no se recalcula por WS ni por push.
- La ruta objetivo se samplea a pasos fijos (aprox cada 30-50 m por segmento) y el match % =
  fracción de samples dentro de acceptance_radius_meters de cualquier punto GPS del intento
  (Haversine en Go).
- Sin GPS suficiente en el intento → match % = 0.
- Sin umbral mínimo de coincidencia: el mejor match gana si hay intentos; sin intentos →
  winner vacía.

#### Scenario: Intento que cubre la ruta

- **WHEN** los puntos GPS del intento rodean todos los samples de la ruta dentro del radio
- **THEN** el match % del intento es 100.

#### Scenario: Intento sin GPS

- **WHEN** el intento no tiene puntos GPS suficientes
- **THEN** su match % se computa como 0.

#### Scenario: Match parcial

- **WHEN** el intento cubre solo parte de los samples dentro del radio
- **THEN** el match % es proporcional a la fracción de samples cubiertos.

### Requirement: Frames WS de competencia

El sistema DEBE cumplir lo siguiente (MUST):

- Al terminar un intento asociado a una competencia, el backend publica
  `update:competition_progress` por el channel `session:{id}` de la instancia del intento,
  con payload `{competition_id, user_id}` (sin contenido de la métrica; mismo patrón del
  broadcast de feedback, sin exclusión del emisor).
- Al cerrar la competencia, el backend publica `control:competition_finished` por los
  channels `session:{instance}` de los intentos registrados; solo se emite si hay intentos
  (si no hay, el push alcanza al resto de los participantes).

#### Scenario: Broadcast al terminar intento

- **WHEN** un intento de competencia pasa a finished
- **THEN** el channel `session:{instance}` recibe `update:competition_progress` con
  `{competition_id, user_id}`.

#### Scenario: Payload mínimo sin métrica

- **WHEN** se arma el frame de progreso
- **THEN** solo lleva competition_id y user_id; el detalle se refresca por consulta.

#### Scenario: Cierre con intentos

- **WHEN** la competencia se cierra y tenía intentos registrados
- **THEN** cada channel correspondiente a esas instancias recibe
  `control:competition_finished`.

#### Scenario: Cierre sin intentos no emite frames

- **WHEN** la competencia se cierra sin intentos
- **THEN** no se emite `control:competition_finished` y el punto rojo/push alcanza a los
  participantes.
