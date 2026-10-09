# team-payment-visibility delta

## ADDED Requirements

### Requirement: membership_fee y can_receive_payments visibles antes de unirse

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /api/v1/teams/search` incluye en cada card `membership_fee` (float, vigente del equipo) y `can_receive_payments` (bool).
- `GET /api/v1/teams/{id}` incluye `can_receive_payments` (además del `membership_fee` ya existente).
- `InvitationResponse` (todas las respuestas de invitación que exponen `team_name`) incluye `membership_fee` (vigente del equipo al consultar, no el frozen de la invitación) y `can_receive_payments`.
- `can_receive_payments` es derivado: true si el owner del equipo tiene una `seller_connection` con `status=authorized` y `public_key` presente; nunca es columna, y si el lookup de conexiones falla el flag queda `false` (la falla real no puede depender de un campo informativo).
- El fee congelado al momento de unirse (`init_amount`) queda fuera de estos endpoints.

#### Scenario: Card de búsqueda con fee y capacidad

- **WHEN** un corredor consulta `GET /teams/search`
- **THEN** cada card incluye `membership_fee` vigente y `can_receive_payments` derivado de la conexión MP del owner.

#### Scenario: Owner sin conexión MP

- **WHEN** el owner del equipo no tiene seller_connection authorized con public key
- **THEN** `can_receive_payments` es `false` en search, detalle e invitaciones.

#### Scenario: Invitación con precio y capacidad

- **WHEN** un corredor lista sus invitaciones recibidas
- **THEN** cada invitación incluye `membership_fee` vigente del equipo y `can_receive_payments` del owner.
