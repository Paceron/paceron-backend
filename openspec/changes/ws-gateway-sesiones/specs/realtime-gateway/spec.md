# Delta de specs: realtime-gateway

## ADDED Requirements

### Requirement: Gateway WebSocket autenticado

El sistema DEBE cumplir lo siguiente (MUST):

La ruta `GET /api/v1/ws` aceptará conexiones WebSocket de clientes autenticados, validando el access token JWT provisto en el query param `token` al momento del upgrade. Una conexión establecida estará asociada a un único usuario autenticado y soportará mensajes JSON con la forma `{type, ...}` según el protocolo definido.

#### Scenario: Upgrade sin token

- **WHEN** un cliente abre la conexión sin el query param `token` o con un token inválido o expirado
- **THEN** el servidor responde `401` (respuesta HTTP antes del upgrade) y no establece la conexión WebSocket

#### Scenario: Upgrade con token válido

- **WHEN** un cliente abre la conexión con `?token=<access_token_válido>`, desde un orígen permitido o sin header `Origin` (cliente nativo)
- **THEN** el servidor completa el upgrade y la conexión queda asociada a ese usuario

#### Scenario: Heartbeat

- **WHEN** el cliente envía `{"type":"ping"}`
- **THEN** el servidor responde `{"type":"pong"}` y refresca el deadline de lectura; una conexión que no envía ningún mensaje dentro de ~45s es cerrada por el servidor

#### Scenario: Mensaje inválido

- **WHEN** el cliente envía un mensaje con `type` desconocido, JSON inválido o excediendo el límite de tamaño (~4KB)
- **THEN** el servidor responde `{"type":"error", ...}` sin cortar la conexión (el límite de tamaño excedido puede cerrar la conexión)

### Requirement: Suscripción autorizada por canal

El sistema DEBE cumplir lo siguiente (MUST):

El gateway validará la autorización del canal al momento del `subscribe`, mediante un registro de patrones. El patrón `session:{id}` (con `{id}` = sessionInstanceId) aplicará la misma regla dual que `GET /session-instances/{id}`: miembro activo del grupo del día con esa instancia u owner del equipo, O feedback activo sobre la instancia como atleta, reportante u owner del equipo.

#### Scenario: Suscripción a canal autorizado

- **WHEN** un usuario autorizado envía `{"type":"subscribe","channel":"session:123"}` para una instancia que puede ver
- **THEN** el servidor responde `{"type":"subscribed","channel":"session:123"}` y desde entonces recibe los mensajes dirigidos a ese canal

#### Scenario: Suscripción a canal no autorizado

- **WHEN** un usuario sin acceso a la instancia envía `subscribe` a `session:{id}`
- **THEN** el servidor responde `{"type":"error", ...}` y la conexión permanece abierta; el usuario no recibe mensajes de ese canal

#### Scenario: Canal desconocido o inválido

- **WHEN** un cliente envía `subscribe` a un canal sin patrón registrado o con formato inválido (ej. `session:abc`)
- **THEN** el servidor responde `{"type":"error", ...}` sin cortar la conexión

#### Scenario: Tope de canales

- **WHEN** una conexión intenta suscribirse a más de ~20 canales
- **THEN** el servidor rechaza el subscribe de más con `{"type":"error", ...}`

### Requirement: Reenvío de presence/control

El sistema DEBE cumplir lo siguiente (MUST):

Los mensajes `presence` y `control` enviados por un suscriptor serán reenviados tal cual (con `from` = userID del emisor y el `payload` opaco sin modificación) a todos los demás suscriptores del mismo canal, excluyendo al emisor.

#### Scenario: presence llega al resto del canal

- **WHEN** un suscriptor envía `{"type":"presence","payload":{...}}` a un canal con otros suscriptores
- **THEN** cada otro suscriptor del canal recibe `{"type":"presence","from":<userID_emisor>,"payload":{...}}`, y el emisor no recibe su propio mensaje

#### Scenario: Canal sin otros suscriptores

- **WHEN** el único suscriptor del canal envía `presence`
- **THEN** ningún cliente recibe el reenvío (no hay error)

### Requirement: Broadcast update:set_event en creación de feedback

El sistema DEBE cumplir lo siguiente (MUST):

Al crear un feedback vía `POST /workout-feedback`, el backend emitirá al canal `session:{assigned_session_id}` un mensaje `{"type":"update:set_event","data":{"message":"...","data":{...WorkoutFeedbackResponse...}}}` cuyo campo `data` sea exactamente el mismo body HTTP que responde el endpoint (incluyendo `athlete_user_id`). La emisión será asíncrona y best-effort: no alterará ni bloqueará la respuesta HTTP, y un evento sin destinatarios se descarta.

#### Scenario: Corredor persiste una serie y el entrenador lo ve en vivo

- **WHEN** el atleta (o un entrenador por él) crea un feedback con `assigned_session_id` válido, mientras otra conexión suscripta al canal `session:{assigned_session_id}` está abierta
- **THEN** la HTTP responde `201` sin cambios y la conexión suscripta recibe `update:set_event` con el mismo `WorkoutFeedbackResponse` que devuelve la HTTP

#### Scenario: Nadie suscripto

- **WHEN** se crea un feedback y ninguna conexión está suscripta al canal correspondiente
- **THEN** la creación se comporta exactamente igual que hoy (evento descartado)

#### Scenario: Operaciones que no broadcastean

- **WHEN** se persisten puntos (`POST /workout-feedback/:id/points`), se edita un feedback (`PUT /workout-feedback/:id`) o se elimina (`DELETE /workout-feedback/:id`)
- **THEN** no se emite ningún evento WS
