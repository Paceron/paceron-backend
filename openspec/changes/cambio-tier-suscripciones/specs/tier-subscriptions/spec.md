## ADDED Requirements

### Requirement: Asignación de rol con tier pago crea suscripción y cuota inicial

Cuando se asigna un rol a un usuario (`POST /api/v1/users/:id/roles`) con un tier cuyo `payment_required` es `true`, el sistema SHALL crear una suscripción en `user_role_tier_subscriptions` con `status = first_payment_pending`, `start_date` = fecha actual e `init_amount` = `tier_amount` del tier, y generar la cuota #1 en `installments` con `installment_number = 1`, `status = pending`, `amount` = `tier_amount`, y `due_date`/`blocked_date` nulos. El sistema SHALL verificar que el tier pertenezca al rol antes de asignarlo.

Cuando el tier tiene `payment_required = false` (gratis, ej. "base"), el sistema SHALL asignar el rol y el tier directamente sin crear suscripción ni cuota.

#### Scenario: Asignar rol con tier pago
- **WHEN** se llama a `POST /api/v1/users/:id/roles` con `tier_id` de un tier con `payment_required = true`
- **THEN** el sistema crea la suscripción con `status = first_payment_pending` e `init_amount = tier_amount`
- **AND** crea la cuota #1 con `status = pending`, `amount = tier_amount` y `due_date`/`blocked_date` nulos

#### Scenario: Asignar rol con tier gratis
- **WHEN** se llama a `POST /api/v1/users/:id/roles` con un tier con `payment_required = false`
- **THEN** el sistema asigna el rol y el tier sin crear suscripción ni cuota

#### Scenario: Tier que no pertenece al rol
- **WHEN** se llama a `POST /api/v1/users/:id/roles` con un `tier_id` cuyo `role_id` no coincide con el rol asignado
- **THEN** el sistema rechaza la operación con un error de validación

### Requirement: El acceso al tier pago se activa al pagar la primera cuota

Una suscripción con `status = first_payment_pending` SHALL otorgar acceso al tier solo cuando su cuota #1 quede pagada. Al confirmarse el pago de la cuota #1 (vía webhook de Mercado Pago), el sistema SHALL marcar la cuota como `paid`, incrementar `paid_installments` a 1 y cambiar la suscripción a `status = active`.

#### Scenario: Confirmación del pago de la primera cuota
- **WHEN** el webhook de Mercado Pago confirma el pago correspondiente a la cuota #1 de una suscripción `first_payment_pending`
- **THEN** la cuota #1 pasa a `status = paid`
- **AND** `paid_installments` de la suscripción pasa a 1
- **AND** la suscripción pasa a `status = active`

#### Scenario: Doble notificación del mismo pago
- **WHEN** el webhook de Mercado Pago reenvía la confirmación del mismo pago
- **THEN** la segunda notificación no modifica el estado (update condicional sobre cuota `status = pending`)

### Requirement: El sistema valida deuda antes de un cambio de tier

Antes de cambiar el tier de un usuario dentro de un rol, el sistema SHALL bloquear el cambio si existe una deuda, definida como al menos una cuota `pending` con `blocked_date` (o `due_date`) anterior a la fecha actual, perteneciente a la suscripción vigente (`status` en `active` o `first_payment_pending`). La cuota #1 (sin `due_date`/`blocked_date`) SHALL NO contar como deuda. Las cuotas de suscripciones cerradas (`ended`) SHALL NO contar como deuda.

#### Scenario: Usuario con deuda intenta cambiar de tier
- **WHEN** existe una cuota `pending` vencida (pasada su `blocked_date`) de la suscripción vigente y se solicita un cambio de tier
- **THEN** el sistema rechaza el cambio con un error de deuda pendiente

#### Scenario: Usuario sin deuda cambia de tier
- **WHEN** no existen cuotas `pending` vencidas de la suscripción vigente y se solicita un cambio de tier
- **THEN** el sistema permite el cambio

#### Scenario: Deuda de una suscripción cerrada no bloquea
- **WHEN** las únicas cuotas `pending` pertenecen a una suscripción con `status = ended`
- **THEN** el sistema no las considera deuda y permite el cambio

### Requirement: Cambio de tier dentro del mismo rol

El sistema SHALL permitir cambiar el tier de un usuario hacia arriba, hacia abajo o a otro tier del mismo rol, según la jerarquía (`base < medium < premium`), siempre que el target tier tenga el mismo `role_id` que la suscripción vigente. Al cambiar, el sistema SHALL cerrar la suscripción anterior con `status = ended`, crear una nueva suscripción para el target tier y, si el target tier es pago, generar una nueva cuota #1 (`pending`, sin `due_date`). Si el target tier es gratis, la nueva suscripción pasa directamente a `active` sin generar cuotas.

#### Scenario: Cambio a otro tier con el mismo role_id
- **WHEN** se solicita un cambio de tier a un target con el mismo `role_id` y sin deuda
- **THEN** el sistema cierra la suscripción anterior (`ended`), crea la nueva suscripción y actualiza el tier vigente del usuario

#### Scenario: Cambio a un tier de otro rol
- **WHEN** se solicita un cambio de tier cuyo `role_id` difiere de la suscripción vigente
- **THEN** el sistema rechaza la operación con un error de validación

#### Scenario: Cambio a un tier pago
- **WHEN** el target tier tiene `payment_required = true`
- **THEN** la nueva suscripción se crea con `status = first_payment_pending`
- **AND** se genera una cuota #1 `pending` sin `due_date` ni `blocked_date`

#### Scenario: Cambio a un tier gratis
- **WHEN** el target tier tiene `payment_required = false`
- **THEN** la nueva suscripción se crea con `status = active`
- **AND** no se generan cuotas

### Requirement: La tabla installments es compartida y de arco exclusivo

La tabla `installments` SHALL ser la única fuente de cuotas tanto para suscripciones de tier individuales como para las futuras suscripciones de corredor a equipo. Los campos `subscription_id` (FK → `user_role_tier_subscriptions.id`) y `team_id` (FK → `teams.id`) SHALL ser nullable y el registro SHALL cumplir un CHECK de arco exclusivo: exactamente uno de los dos debe estar seteado. El sistema SHALL escribir en `installments` la cuota con `subscription_id` para suscripciones de tier y dejar `team_id` nulo; el vínculo a equipos se agrega funcionalmente en el change `suscripcion-teams-split` (la columna ya existe para no migrar la tabla).

#### Scenario: Cuota de una suscripción de tier
- **WHEN** el sistema genera una cuota para una suscripción de tier
- **THEN** el registro queda con `subscription_id` seteado y `team_id` nulo

#### Scenario: Un registro no puede referenciar ambos padres
- **WHEN** se intenta insertar una cuota con `subscription_id` y `team_id` ambos no nulos (o ambos nulos)
- **THEN** la base rechaza la inserción (CHECK de arco exclusivo)

### Requirement: Una sola suscripción vigente por usuario y rol

El sistema SHALL garantizar que un usuario tenga, como máximo, una suscripción vigente (`status` en `active` o `first_payment_pending`) por cada `(user_id, role_id)`. La tabla `user_role_tier_subscriptions` SHALL definir un índice único parcial sobre `(user_id, role_id)` para los registros vigentes, y la terna `(user_id, role_id, tier_id)` SHALL ser requerida en todo registro.

#### Scenario: Crear una suscripción con una vigente existente
- **WHEN** se intenta crear una suscripción vigente para un `(user_id, role_id)` que ya tiene una
- **THEN** el sistema rechaza la operación (violación del índice único parcial)

### Requirement: Endpoint de próxima cuota a pagar

El sistema SHALL exponer un endpoint que, dado un `user_id` y un `role_id`, devuelva la información de la próxima cuota a pagar de la suscripción vigente, sin procesar el pago. La respuesta SHALL incluir: `subscription_id`, `subscription_status`, `installment_id`, `installment_number`, `installment_amount`, `next_due_date`, `blocked_date`, datos del tier y rol (`tier_id`, `tier_name`, `hierarchy`, `role_id`, `role_name`, `payment_required`) y la `public_key` de Mercado Pago. Cuando la suscripción es `first_payment_pending`, `next_due_date` y `blocked_date` SHALL ser nulos.

#### Scenario: Consultar próxima cuota de una suscripción pendiente
- **WHEN** se consulta el endpoint de próxima cuota para un `user_id` y `role_id` cuya suscripción está en `first_payment_pending`
- **THEN** el sistema devuelve la cuota #1 con `next_due_date = null`, `blocked_date = null`, `installment_amount` y la `public_key`

#### Scenario: Consultar próxima cuota de un rol gratis
- **WHEN** se consulta el endpoint para un `user_id` y `role_id` cuyo tier es gratis (sin suscripción)
- **THEN** el sistema devuelve un estado sin cuota pendiente e indica que el tier es `payment_required = false`

### Requirement: El webhook de pago registra cuotas pagadas

Cuando el webhook de Mercado Pago confirma un pago vinculado a una suscripción de tier, el sistema SHALL identificar la cuota asociada, marcarla como `paid` de forma idempotente (update condicional sobre `status = pending`), incrementar `paid_installments` de la suscripción y, si la cuota es la #1, activar la suscripción.

#### Scenario: Webhook confirma pago de cuota posterior a la primera
- **WHEN** el webhook confirma un pago correspondiente a una cuota con `installment_number > 1`
- **THEN** el sistema marca la cuota como `paid`
- **AND** incrementa `paid_installments` sin cambiar el `status` de la suscripción (si ya estaba `active`)

### Requirement: La gratuidad de un tier se define por payment_required

El sistema SHALL usar la columna `payment_required` de `tiers` como única fuente de verdad para determinar si un tier es gratis o pago, en lugar de derivarlo del nombre. Al crear o actualizar un tier cuyo nombre es `base` (valores posibles: `base`, `medium`, `premium`), el sistema SHALL forzar `payment_required = false` si el nombre es `base` y asignar el valor de `hierarchy` acorde (base = 1, medium = 2, premium = 3).

#### Scenario: Crear un tier llamado "base"
- **WHEN** se crea un tier con `name = base`
- **THEN** el sistema fuerza `payment_required = false`
- **AND** asigna `hierarchy = 1`

#### Scenario: Crear un tier pago
- **WHEN** se crea un tier con `name = premium`
- **THEN** el sistema asigna `hierarchy = 3`
- **AND** mantiene el `payment_required` provisto