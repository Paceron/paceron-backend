# team-subscriptions

## ADDED Requirements

### Requirement: Estado de cuota batch por equipo

El sistema DEBE (MUST) exponer `GET /api/v1/teams/{id}/subscriptions` para que el owner del equipo o un entrenador del equipo obtenga en una sola respuesta el estado de cuota de todos los corredores activos. El endpoint DEBE ser aditivo y no modificar el estado de cuenta self-only existente.

#### Scenario: Owner consulta el estado del equipo

- **WHEN** el owner del equipo autenticado llama `GET /api/v1/teams/{id}/subscriptions`
- **THEN** recibe `200` con `team_id`, `membership_fee` y `members` con los corredores activos
- **AND** cada miembro incluye `user_id`, `status`, `subscription_status`, `has_debt`, `init_amount` y `paid_installments`

#### Scenario: Entrenador del equipo consulta el estado

- **WHEN** un `team_user` activo con `role_in_team='entrenador'` llama el endpoint
- **THEN** recibe `200` con la misma respuesta batch

#### Scenario: Corredor o usuario ajeno es rechazado

- **WHEN** un corredor del equipo o un usuario que no pertenece al equipo llama el endpoint
- **THEN** recibe `403 Forbidden`
- **AND** no se ejecuta la consulta de cuotas

#### Scenario: Equipo inexistente

- **WHEN** se llama el endpoint con un `team_id` que no existe
- **THEN** recibe `404` con `equipo no encontrado`

#### Scenario: Equipo gratis

- **WHEN** el equipo tiene `membership_fee=0` y el corredor no tiene `init_amount` pendiente
- **THEN** el miembro devuelve `status='active'` y `has_debt=false`

#### Scenario: Primer pago pendiente

- **WHEN** el corredor tiene `subscription_status='first_payment_pending'` y no tiene deuda vencida
- **THEN** el miembro devuelve `status='first_payment_pending'`

#### Scenario: Deuda vencida

- **WHEN** el corredor tiene una cuota pendiente vencida o una cuota esperada por el ciclo de membresía ya vencida
- **THEN** el miembro devuelve `has_debt=true` y `status='overdue'`

#### Scenario: Solo corredores activos

- **WHEN** el equipo tiene owner, entrenadores y corredores
- **THEN** `members` incluye únicamente filas activas con `role_in_team='corredor'`
- **AND** no incluye al owner ni a entrenadores

#### Scenario: Consulta batch sin N+1

- **WHEN** el equipo tiene varios corredores con cuotas pendientes
- **THEN** el service obtiene todas las cuotas del equipo en una sola consulta
- **AND** agrupa por `user_id` en memoria
