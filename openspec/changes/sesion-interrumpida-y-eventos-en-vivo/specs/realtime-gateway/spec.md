# Delta spec: realtime-gateway (Gaps 26 y 27)

Nota: capability existente (ver `ws-gateway-sesiones`). El resto de la capability queda vigente: upgrade/token, heartbeat, límites, payload objeto para presence/control, canales canónicos `session:{id}`.

## ADDED Requirements

### Requirement: Mensajes dirigidos dentro del canal

El sistema DEBE cumplir lo siguiente (MUST):

- El relay de `presence`/`control` respeta un destino opcional `to` DENTRO de `payload`:
  - `payload.to` numérico (userId) → el frame se entrega SOLO a las conexiones de ese usuario suscriptas al canal (todas sus conexiones).
  - `payload.to == "all"` o ausente → comportamiento actual: todos los suscriptores del canal excepto el emisor.
  - `payload.to` de otro tipo (string distinto de "all", `null`) → se ignora y el frame va a todos.
- El frame entregado mantiene el shape actual `{type, from, payload}` completo (el `to` viaja dentro del payload).
- La autorización no cambia: el emisor debe estar suscripto al canal. La entrega sigue siendo fire-and-forget, sin persistencia.

#### Scenario: mensaje dirigido a un corredor

- **WHEN** un cliente suscripto al canal envía `control` con `payload.to = 7` y el usuario 7 está suscripto al mismo canal con 2 conexiones
- **THEN** SOLO esas 2 conexiones reciben el frame `{"type":"control","from":<emisor>,"payload":{...,"to":7}}` y el resto de la sala no lo ve.

#### Scenario: sin destino se comporta como hoy

- **WHEN** un cliente envía `presence`/`control` sin `to` o con `to:"all"`
- **THEN** el frame llega a todos los suscriptores del canal excepto el emisor (comportamiento actual intacto).

#### Scenario: destino inexistente en la sala

- **WHEN** un cliente envía `payload.to` de un usuario NO suscripto al canal
- **THEN** el frame no llega a nadie y la conexión del emisor sigue viva (entrega silenciosamente vacía, sin error).
