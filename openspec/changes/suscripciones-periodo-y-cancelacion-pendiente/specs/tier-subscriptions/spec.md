## ADDED Requirements

### Requirement: El cambio de tier queda bloqueado con un primer pago pendiente

El sistema SHALL rechazar `PUT /api/v1/users/:id/roles/:role_id/tier` con `409` + código `SUBSCRIPTION_PENDING_FIRST_PAYMENT` cuando, para ese `(user_id, role_id)`, ya existe una suscripción con `status = first_payment_pending` antes de aplicar el cambio. El cambio de tier SHALL solo ser posible cuando no exista sub en ese estado; la cancelación de esa sub pendiente (ver "Cancela una suscripción con primer pago pendiente") vuelve a habilitar el cambio.

#### Scenario: Cambio de tier con una suscripción pendiente existente
- **WHEN** se llama a `PUT /api/v1/users/:id/roles/:role_id/tier` y para ese `(user_id, role_id)` existe una suscripción `first_payment_pending`
- **THEN** el sistema rechaza la operación con `409` y código `SUBSCRIPTION_PENDING_FIRST_PAYMENT`

#### Scenario: Cambio de tier tras cancelar la pendiente
- **WHEN** la suscripción pendiente fue cancelada (estado `canceled`) y se reintenta el cambio de tier
- **THEN** el sistema permite aplicar el cambio de tier

### Requirement: Cancela una suscripción con primer pago pendiente

El sistema SHALL exponer `DELETE /api/v1/users/:id/roles/:role_id/subscriptions/pending` con `tier_id` como query param obligatorio, reservado al propio usuario (mismo criterio self-only que el resto de la gestión de suscripciones). Si existe una suscripción en `first_payment_pending` para esa `(user_id, role_id, tier_id)`, SHALL pasarla a `canceled` y pasar sus cuotas pendientes a `canceled`, liberando el slot del índice único parcial (que solo cubre `active`/`first_payment_pending`) para permitir una nueva suscripción. Si no existe ninguna suscripción para la terna → `404`; si existe pero no está en `first_payment_pending` (por ejemplo `active`) → `409` y no se modifica nada.

#### Scenario: Cancelar una suscripción pendiente
- **WHEN** se llama al DELETE sobre una sub `first_payment_pending` de la terna `(user_id, role_id, tier_id)`
- **THEN** la suscripción pasa a `status = canceled`
- **AND** todas sus cuotas `pending` pasan a `status = canceled`
- **AND** queda habilitada la creación de una nueva suscripción vigente para ese `(user_id, role_id)`

#### Scenario: No existe la suscripción de esa terna
- **WHEN** se llama al DELETE y no existe ninguna suscripción para `(user_id, role_id, tier_id)`
- **THEN** el sistema responde `404` sin modificar nada

#### Scenario: La suscripción no está en primer pago pendiente
- **WHEN** se llama al DELETE y la sub de la terna existe pero su estado no es `first_payment_pending`
- **THEN** el sistema responde `409` y no modifica el estado

#### Scenario: Cancelar la suscripción de otro usuario
- **WHEN** un usuario intenta cancelar una sub pendiente de otro usuario (user id del path distinto al autenticado)
- **THEN** el sistema responde `403` (self-only)

### Requirement: Estados cancelados para suscripción y cuota

El sistema SHALL reconocer el estado terminal `canceled` tanto para suscripciones (`SubscriptionStatusCanceled` = `canceled`) como para cuotas (`InstallmentStatusCanceled` = `canceled`). El estado `canceled` de una suscripción SHALL quedar fuera del conjunto de "vigentes" (no cubierto por el índice único parcial ni por `FindActiveByUserRole`). Una suscripción `first_payment_pending` SHALL poder transicionar a `canceled` únicamente vía el endpoint de cancelación; una suscripción `active` SHALL NO poder cancelarse por ese medio.

#### Scenario: La suscripción cancelada deja de ser vigente
- **WHEN** una suscripción pasa a `canceled`
- **THEN** deja de ser retornada por la consulta de período `current` y `next`, y ya no bloquea la creación de una suscripción vigente para el mismo `(user_id, role_id)`

#### Scenario: Cuota de una suscripción cancelada
- **WHEN** una suscripción `first_payment_pending` se cancela
- **THEN** su cuota pendiente #1 pasa a `canceled` y ya no se ofrece como próxima cuota a pagar

## MODIFIED Requirements

### Requirement: Endpoint de próxima cuota a pagar

El sistema SHALL exponer un endpoint `GET /api/v1/users/:id/subscriptions/:period` que, dado un `user_id`, el `role_id` (query param obligatorio) y un `period`, devuelva la información de la próxima cuota a pagar de la suscripción correspondiente a ese período, sin procesar el pago:

- `period = current`: suscripción con `status = active` para ese `(user_id, role_id)`; si existiera más de una, la del tier de mayor `hierarchy` (el índice único parcial garantiza a lo sumo una).
- `period = next`: suscripción con `status = first_payment_pending` para ese `(user_id, role_id)`.
- Si no existe suscripción en el estado del período → SHALL responder `200` con body vacío (`{}`).
- `period` con valor distinto de `current`/`next` → SHALL responder `400`.
- `role_id` no provisto o inválido → `400` (igual que hoy).

La respuesta SHALL incluir `subscription_id`, `subscription_status`, `installment_id`, `installment_number`, `installment_amount`, `next_due_date`, `blocked_date`, datos del tier y rol (`tier_id`, `tier_name`, `hierarchy`, `role_id`, `role_name`, `payment_required`) y la `public_key` de Mercado Pago. Cuando la suscripción es `first_payment_pending`, `next_due_date` y `blocked_date` SHALL ser nulos.

#### Scenario: Consultar la suscripción activa actual
- **WHEN** se consulta el endpoint con `period = current` para un `user_id` y `role_id` con sub en `active`
- **THEN** el sistema devuelve la próxima cuota a pagar de esa sub (o el estado del tier/rol si no hubiera cuota) con la `public_key` si el tier es pago

#### Scenario: Consultar la suscripción con primer pago pendiente
- **WHEN** se consulta el endpoint con `period = next` para un `user_id` y `role_id` cuya suscripción está en `first_payment_pending`
- **THEN** el sistema devuelve la cuota #1 con `next_due_date = null`, `blocked_date = null`, `installment_amount`, `installment_number = 1` y la `public_key`

#### Scenario: Sin suscripción en el período consultado
- **WHEN** se consulta `period = current` (o `next`) y no existe sub en ese estado para `(user_id, role_id)` (por ejemplo rol gratis sin sub activa, o pendiente ya cancelada)
- **THEN** el sistema responde `200` con body vacío (`{}`)

#### Scenario: Período no soportado
- **WHEN** se consulta el endpoint con un `period` distinto de `current` o `next`
- **THEN** el sistema responde `400`