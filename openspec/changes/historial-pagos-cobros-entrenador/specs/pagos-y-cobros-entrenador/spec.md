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

### Requirement: Listar el historial de pagos del usuario

El sistema MUST devolver, paginados de a 20 y del más reciente al más antiguo, los pagos cuya cuota tiene `user_id` igual al usuario autenticado, excluyendo los que no tienen `payment_id` de Mercado Pago. Cada pago MUST incluir fecha, monto, método de pago, `type` (`subscription` o `trainer_payment`) y estado. El `type` SHALL derivarse del padre de la cuota, no del `concept` del pago.

#### Scenario: Suscripción y pago a entrenador en el mismo historial

- **WHEN** el usuario pagó una cuota de su tier y una cuota de membresía de un equipo
- **THEN** los dos pagos aparecen en el historial
- **AND** el de tier trae `type = subscription` y el `tier`
- **AND** el de membresía trae `type = trainer_payment`, el `team` y el `trainer`

#### Scenario: Pago de tier guardado como `order`

- **WHEN** un pago tiene `concept = order` y está vinculado a una cuota de tier del usuario
- **THEN** aparece con `type = subscription`

#### Scenario: Pagos de otro usuario

- **WHEN** otro usuario tiene pagos
- **THEN** no aparecen en el historial

#### Scenario: Filtros por tipo y estado

- **WHEN** se pide `type=subscription&status=rejected`
- **THEN** solo vuelven los pagos de tier con estado `rejected` o `cancelled`

#### Scenario: Tipo inválido

- **WHEN** se pide `type=order`
- **THEN** el backend responde `400` con `code = INVALID_QUERY`

### Requirement: Resumir los cobros por mes y por equipo

El sistema MUST devolver `months` meses seguidos que terminan en el mes `until` (o en el mes actual si no se manda) en hora argentina, con el bruto aprobado, el neto conocido y la cantidad de cobros de cada mes. MUST incluir los totales por equipo y la cantidad de cuotas pendientes y rechazadas de esa ventana, contadas sobre el último intento de cada cuota, y el primer mes con cobros del vendedor (`earliest_month`, o `null`). `months` SHALL estar entre 2 y 12. `until` SHALL tener formato `YYYY-MM` y no ser posterior al mes actual.

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

#### Scenario: Ventana corrida hacia atrás

- **WHEN** se pide `months=6&until=2026-05`
- **THEN** `monthly` va de `2025-12` a `2026-05`
- **AND** los cobros de `2026-06` en adelante no suman en ningún total

#### Scenario: `until` en el futuro o mal formado

- **WHEN** se pide `until=2099-01` o `until=mayo`
- **THEN** el backend responde `400` con `code = INVALID_QUERY`

#### Scenario: Primer mes con cobros

- **WHEN** el vendedor tiene cobros desde `2026-02`
- **THEN** `earliest_month = "2026-02"`, sin importar la ventana pedida
- **AND** si no tiene ningún cobro, `earliest_month = null`

### Requirement: Requerir autenticación

Los tres endpoints MUST requerir un usuario autenticado y tomar su identidad del token, nunca de un parámetro.

#### Scenario: Sin token

- **WHEN** se llama a cualquiera de los tres endpoints sin usuario autenticado
- **THEN** el backend responde `401`
