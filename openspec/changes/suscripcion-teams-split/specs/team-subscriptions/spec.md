## ADDED Requirements

### Requirement: El monto mensual se define por equipo

Cada equipo SHALL tener un `membership_fee` (numeric, default 0) que define lo que paga cada corredor al entrenador por mes. Un valor de `0` SHALL significar equipo gratis, sin suscripción ni cuotas. El sistema SHALL permitir al dueño del equipo (`teams.owner_id`) actualizar este monto.

#### Scenario: Equipo con mensualidad
- **WHEN** se crea/actualiza un equipo con `membership_fee = 5000`
- **THEN** el equipo queda con mensualidad de 5000 para futuros miembros

#### Scenario: Equipo gratis
- **WHEN** un equipo tiene `membership_fee = 0`
- **THEN** sus miembros no generan suscripción ni cuotas

### Requirement: Unirse a un equipo con mensualidad exige el primer pago

Al sumar un corredor a un equipo con `membership_fee > 0` (tanto por `POST /api/v1/teams/:id/users` como por aceptar una invitación `POST /api/v1/invitations/:id/accept`), el sistema SHALL crear el `team_user` con `subscription_status = first_payment_pending`, `init_amount = membership_fee` y `paid_installments = 0`, y SHALL generar la cuota #1 en `installments` con `team_id` seteado y `subscription_id` nulo (arco exclusivo definido en `cambio-tier-suscripciones`), `installment_number = 1`, `status = pending`, `amount = membership_fee` y `due_date`/`blocked_date` nulos. El rol de acceso pleno al equipo (`active`) SHALL activarse recién al pagarse la cuota #1. Si el equipo es gratis (`membership_fee = 0`), el `team_user` se crea directamente con `subscription_status = active` y sin cuotas.

#### Scenario: Agregar usuario a un equipo pago
- **WHEN** se llama a `POST /api/v1/teams/:id/users` en un equipo con `membership_fee = 5000`
- **THEN** el `team_user` se crea con `subscription_status = first_payment_pending`
- **AND** se crea la cuota #1 con `team_id`, `status = pending` y `amount = 5000`, sin `due_date` ni `blocked_date`

#### Scenario: Aceptar invitación a un equipo pago
- **WHEN** un corredor acepta una invitación a un equipo con `membership_fee > 0`
- **THEN** el `team_user` se crea en `first_payment_pending` con su cuota #1 (mismas reglas)

#### Scenario: Unirse a un equipo gratis
- **WHEN** el equipo tiene `membership_fee = 0`
- **THEN** el `team_user` se crea con `subscription_status = active` sin generar cuotas

### Requirement: La membresía activa se alcanza al pagar la primera cuota

Al confirmarse el pago de la cuota #1 de un `team_user` en `first_payment_pending`, el sistema SHALL marcar la cuota como `paid`, incrementar `paid_installments` a 1 y pasar el `team_user` a `subscription_status = active`. El marcado SHALL ser idempotente (update condicional sobre `status = pending`).

#### Scenario: Confirmación del primer pago de un miembro
- **WHEN** el webhook de Mercado Pago confirma el pago de la cuota #1 de un `team_user` `first_payment_pending`
- **THEN** la cuota #1 pasa a `paid`
- **AND** `paid_installments` pasa a 1
- **AND** el `team_user` pasa a `subscription_status = active`

#### Scenario: Doble notificación del mismo primer pago
- **WHEN** el webhook reenvía la confirmación de la misma cuota #1
- **THEN** la segunda notificación no modifica el estado

### Requirement: Cuotas mensuales siguientes del equipo

Al confirmarse el pago de la cuota `N` de un `team_user` activo, el sistema SHALL generar la cuota `N+1` con `team_id`, `status = pending`, `amount = init_amount` y `due_date` igual a `start_date + 1 mes` si `N == 1` o a la `due_date` de la cuota `N` + 1 mes en caso contrario, y `blocked_date = due_date + 7 días`. Sin pago, no se generan cuotas nuevas.

#### Scenario: Generación de la cuota siguiente
- **WHEN** se paga la cuota #1 de un miembro activo
- **THEN** se genera la cuota #2 con `due_date = start_date + 1 mes` y `blocked_date = due_date + 7 días`

### Requirement: La deuda impide dejar el equipo

Antes de quitar a un usuario de un equipo (`DELETE /api/v1/teams/:id/users/:user_id`), el sistema SHALL bloquear la operación si existe deuda, definida como una cuota `pending` con `blocked_date` (o `due_date`) anterior a la fecha actual del `team_user` vigente. La cuota #1 (sin `due_date`/`blocked_date`) SHALL NO contar como deuda. Al salir sin deuda, el sistema SHALL permitir el alta baja, dejando el historial financiero en `installments`.

#### Scenario: Quitar miembro con deuda
- **WHEN** existe una cuota `pending` vencida (pasada su `blocked_date`) y se intenta quitar al miembro
- **THEN** el sistema rechaza la operación con un error de deuda pendiente

#### Scenario: Quitar miembro sin deuda
- **WHEN** no existen cuotas `pending` vencidas
- **THEN** el sistema permite quitar al miembro y conserva el historial en `installments`

### Requirement: Endpoint de estado de cuenta del equipo

El sistema SHALL exponer `GET /api/v1/users/:id/teams/:team_id/subscription` que devuelva la membresía y la próxima cuota a pagar del corredor para ese equipo, sin procesar el pago. La respuesta SHALL incluir: datos del equipo (`team_id`, `team_name`, `membership_fee`), estado de la membresía (`subscription_status`, `init_amount`, `paid_installments`, `start_date`), la próxima cuota (`installment_id`, `installment_number`, `installment_amount`, `next_due_date`, `blocked_date`), un indicador `has_debt` y los datos para el checkout de Mercado Pago (`public_key`, `concept = team_subscription`, `marketplace = true`). Si el equipo es gratis, SHALL devolver `subscription_status = active` sin cuota pendiente.

#### Scenario: Consultar estado de un miembro pending de pago
- **WHEN** se consulta el endpoint para un miembro en `first_payment_pending`
- **THEN** devuelve la cuota #1 con `next_due_date = null` y `blocked_date = null`, con `concept = team_subscription` y `marketplace = true`

#### Scenario: Consultar estado de un miembro activo con cuotas mensuales
- **WHEN** se consulta el endpoint para un miembro activo con una cuota recurrente pendiente
- **THEN** devuelve la cuota pendiente con su `next_due_date` y `blocked_date`

#### Scenario: Consultar estado de un equipo gratis
- **WHEN** se consulta el endpoint para un corredor de un equipo con `membership_fee = 0`
- **THEN** devuelve `subscription_status = active` sin cuota pendiente ni datos de checkout