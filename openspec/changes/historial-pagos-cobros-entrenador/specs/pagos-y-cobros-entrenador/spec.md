# Spec: Pagos y cobros del entrenador

Consulta de los cobros de membresía de equipo que recibió un entrenador, de sus
propios pagos de suscripción de tier y de un resumen mensual de cobros.

## ADDED Requirements

### Requirement: Listar los cobros recibidos

El sistema MUST devolver, paginados de a 20 y ordenados del más reciente al más antiguo, los pagos con `seller_user_id` igual al usuario autenticado y `concept = team_subscription`. El sistema MUST excluir los pagos sin `payment_id` de Mercado Pago.

#### Scenario: Solo cobros propios

- **WHEN** existen cobros de otro entrenador
- **THEN** no aparecen en la respuesta

#### Scenario: Preferencia que nunca se pagó

- **WHEN** existe un pago con `payment_id` vacío
- **THEN** no aparece en el listado
- **AND** no cuenta en el resumen

#### Scenario: Paginación

- **WHEN** el entrenador tiene 21 cobros
- **THEN** la página 1 trae 20 con `has_more = true`
- **AND** la página 2 trae 1 con `has_more = false`

#### Scenario: Filtro por equipo y por estado

- **WHEN** se pide `team_id=12&status=rejected`
- **THEN** solo vuelven los cobros de ese equipo con estado `rejected` o `cancelled`

#### Scenario: Parámetros inválidos

- **WHEN** se pide `page=0`, un `team_id` no numérico o un `status` desconocido
- **THEN** el backend responde `400` con `code = INVALID_QUERY`

### Requirement: Exponer el monto bruto y el neto real

Cada cobro MUST incluir `gross_amount`. El campo `net_amount` SHALL ser `null` salvo que el pago esté `approved` y su `raw_response` tenga `transaction_details.net_received_amount` numérico y mayor que 0. El sistema MUST NOT exponer `marketplace_fee`.

#### Scenario: Pago aprobado que pasó por el webhook

- **WHEN** el pago está `approved` y `raw_response` trae `net_received_amount = 14101.5`
- **THEN** `net_amount = 14101.5`

#### Scenario: Pago aprobado sin datos del webhook

- **WHEN** el pago está `approved` y `raw_response` no trae `transaction_details`
- **THEN** `net_amount = null`

#### Scenario: Pago pendiente con neto en cero

- **WHEN** el pago está `pending` y `raw_response` trae `net_received_amount = 0`
- **THEN** `net_amount = null`

### Requirement: Listar los pagos de tier propios

El sistema MUST devolver los pagos cuya cuota tiene `subscription_id` y `user_id` igual al usuario autenticado, sin importar el `concept` del pago. Con `role`, el sistema SHALL devolver solo los tiers de ese rol.

#### Scenario: Pago de tier guardado como `order`

- **WHEN** un pago tiene `concept = order` y está vinculado a una cuota de tier del usuario
- **THEN** aparece en el listado

#### Scenario: Cuota de equipo del usuario

- **WHEN** el usuario pagó una cuota de membresía de equipo
- **THEN** ese pago no aparece en sus pagos de tier

#### Scenario: Filtro por rol

- **WHEN** se pide `role=entrenador`
- **THEN** solo vuelven los pagos de tiers del rol entrenador

### Requirement: Resumir los cobros por mes y por equipo

El sistema MUST devolver `months` meses seguidos que terminan en el mes actual en hora argentina, con el bruto aprobado, el neto conocido y la cantidad de cobros de cada mes. MUST incluir los totales por equipo y la cantidad de cuotas pendientes y rechazadas, contadas sobre el último intento de cada cuota. `months` SHALL estar entre 2 y 12.

#### Scenario: Mes sin cobros

- **WHEN** en un mes de la ventana no hubo cobros
- **THEN** ese mes aparece con `gross_amount = 0` y `net_amount = null`

#### Scenario: Borde de mes en hora argentina

- **WHEN** un cobro se creó el `2026-09-01T02:00:00Z`
- **THEN** cuenta en `2026-08`

#### Scenario: Rechazo seguido de un pago aprobado

- **WHEN** una cuota tiene un intento `rejected` y después uno `approved`
- **THEN** no suma a `rejected_count`

#### Scenario: Cuota cuyo último intento está pendiente

- **WHEN** el último intento de una cuota está `in_process`
- **THEN** suma a `pending_count`

#### Scenario: Ventana fuera de rango

- **WHEN** se pide `months=13`
- **THEN** el backend responde `400` con `code = INVALID_QUERY`

### Requirement: Requerir autenticación

Los tres endpoints MUST requerir un usuario autenticado y tomar su identidad del token, nunca de un parámetro.

#### Scenario: Sin token

- **WHEN** se llama a cualquiera de los tres endpoints sin usuario autenticado
- **THEN** el backend responde `401`
