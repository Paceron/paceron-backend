# session-messages delta

## ADDED Requirements

### Requirement: Persistencia de mensajes de sesión

El sistema DEBE cumplir lo siguiente (MUST):

- Tabla `session_messages` con: `id`, `session_instance_id`, `sender_user_id`, `sender_role` (`trainer`|`runner`), `type` (`info`|`aviso`|`alerta`), `recipient_mode` (`all`|`multiple`|`direct`), `body`, `reply_to_message_id` (nullable), `created_at`.
- Tabla join `session_message_recipients` (`message_id`, `user_id`) para los destinatarios de `multiple`/`direct`.
- `sender_role` derivado al persistir: `trainer` si el emisor es owner del team del grupo del día de la instancia; `runner` en cualquier otro caso.
- Los mensajes no se editan ni borran (fuera de alcance confirmado).

#### Scenario: Entrenador avisa a todos

- **WHEN** el entrenador emite `POST /session-instances/85/messages` con `recipient_mode:"all"` y cuerpo
- **THEN** la fila se persiste con `sender_role:"trainer"` y sin destinatarios, y la respuesta `201` contiene el mensaje completo.

#### Scenario: Corredor responde al entrenador

- **WHEN** un corredor responde con `recipient_mode:"direct"`, un destinatario y `reply_to_message_id` del mensaje original
- **THEN** la fila se persiste con `sender_role:"runner"` y el reply válido.

## ADDED Requirements

### Requirement: Autorización y validaciones de mensajes

El sistema DEBE cumplir lo siguiente (MUST):

- POST y GET requieren que el usuario autenticado tenga acceso a la instancia (`HasInstanceAccess`: miembro activo del grupo del día u owner del equipo, O feedback activo como atleta/reportante/owner) — `403` en otro caso, `404` si la instancia no existe.
- Validaciones `400` del POST: `type` fuera del enum, `recipient_mode` fuera del enum, `all` con destinatarios, `direct` sin exactamente 1 destinatario, `multiple` con menos de 2, cuerpo vacío, destinatario que no es participante de la sesión, `reply_to_message_id` inexistente/de otra sesión/no visible para el emisor.

#### Scenario: Emisor ajeno a la sesión

- **WHEN** un usuario sin acceso a la instancia intenta enviar o listar mensajes
- **THEN** recibe `403` (o `404` si además la instancia no existe).

#### Scenario: Destinatario que no participa

- **WHEN** el emisor incluye en `recipient_user_ids` un usuario sin acceso a la instancia
- **THEN** el POST falla con `400` y no persiste nada.

#### Scenario: Reply de otra sesión

- **WHEN** el emisor responde con un `reply_to_message_id` de otra sesión
- **THEN** el POST falla con `400`.

## ADDED Requirements

### Requirement: Historial con visibilidad filtrada

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /session-instances/:id/messages?since=<id>` devuelve los mensajes con `id > since` (omitido/0 = todo el historial) ordenados `id` ASC, sin paginación.
- Visibilidad por mensaje: el consultante lo ve si es el emisor, O `recipient_mode='all'`, O su userId está en los destinatarios. Un DM corredor-corredor no lo ve ni el entrenador ni otros corredores.
- Cada mensaje serializado: `{id, session_instance_id, sender_user_id, sender_role, type, recipient_mode, recipient_user_ids: number[], body, reply_to_message_id, created_at}` — `recipient_user_ids: []` cuando `all`.
- El frontend no re-filtra nada: lo que llega ya es visible para él.

#### Scenario: Privacidad del DM corredor-corredor

- **WHEN** un corredor envía un mensaje `direct` a otro corredor y el entrenador pide el historial
- **THEN** el historial del entrenador NO incluye ese mensaje (no es emisor, no es all, no es destinatario).

#### Scenario: Catch-up tras reconexión

- **WHEN** un participante reconecta y pide `GET .../messages?since=<último visto>`
- **THEN** recibe solo los mensajes posteriores a su cursor, filtrados por su visibilidad.

## ADDED Requirements

### Requirement: Aviso WS sin contenido

El sistema DEBE cumplir lo siguiente (MUST):

- Tras un POST exitoso, el backend emite al canal `session:{id}` el frame `{"type":"control:message_created","channel":"session:{id}","payload":{"sessionMessageId":<id>}}` — sin contenido del mensaje (la privacidad la aplica el filtro del GET).
- La emisión no cambia la respuesta HTTP del POST ni excluye al emisor; es fire-and-forget nil-safe (sin notifier o sin suscriptores, no-op).

#### Scenario: Aviso llega a todos, contenido solo a destinatarios

- **WHEN** se persiste un mensaje dirigido a un solo corredor
- **THEN** todos los suscriptos del canal reciben el aviso `{sessionMessageId}`, pero solo el destinatario (y el emisor, vía su propia respuesta) obtiene el contenido al consultar el GET.
