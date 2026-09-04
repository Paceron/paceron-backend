# Spec: Pago de cuota protegido

Garantiza que al procesar un pago de cuota, la cuota exista y pertenezca al
usuario autenticado, tanto para suscripciones de tier como de equipo.

## ADDED Requirements

### Requirement: Validar que la cuota existe al pagar

Al procesar un pago (`POST /api/v1/payments`), si el `installment_id` enviado no existe, el sistema MUST rechazar el pago con `404` y código `PAYMENT_INSTALLMENT_NOT_FOUND` **antes** de contactar a Mercado Pago, y no debe crearse un pago inválido.

#### Scenario: Cuota inexistente

- **WHEN** el front envía un `installment_id` que no existe en la base
- **THEN** el backend responde `404` con `code = PAYMENT_INSTALLMENT_NOT_FOUND`
- **AND** no se llama a Mercado Pago y no se registra un pago

#### Scenario: Cuota existente

- **WHEN** el `installment_id` existe y el usuario autenticado es su dueño
- **THEN** el pago continúa el flujo normal (creación de preferencia / pago)

### Requirement: Validar la propiedad de la cuota al pagar

Al procesar un pago, si la cuota existe pero su `user_id` no coincide con el usuario autenticado, el sistema MUST rechazar el pago con `403` y código `PAYMENT_INSTALLMENT_FORBIDDEN`. Aplica por igual a cuotas de suscripción de tier y de membresía de equipo.

#### Scenario: Cuota de otro usuario

- **WHEN** el front envía un `installment_id` cuya `user_id` difiere del usuario
  autenticado
- **THEN** el backend responde `403` con `code = PAYMENT_INSTALLMENT_FORBIDDEN`
- **AND** el pago no se registra ni se contacta a Mercado Pago

#### Scenario: Cuota del propio usuario (tier)

- **WHEN** el front paga una cuota de su suscripción de tier (su `user_id`)
- **THEN** el backend acepta y el pago avanza al flujo normal

#### Scenario: Cuota del propio usuario (equipo)

- **WHEN** el front paga una cuota de su membresía a un equipo (su `user_id`)
- **THEN** el backend acepta y el pago avanza al flujo normal

### Requirement: Obtener la identidad del usuario autenticado

Para la validación de propiedad, el controller MUST obtener el id del usuario
autenticado desde el contexto de autenticación (`utils.GetAuthUserID`) y
pasarlo al servicio de pagos.

#### Scenario: Usuario sin identity en el contexto

- **WHEN** `ProcessPayment` se invoca sin un usuario autenticado en el contexto
- **THEN** el backend rechaza el pago sin contactar a Mercado Pago
- **AND** el sistema no permite pagar una cuota anónimamente

### Requirement: Mantener la confirmación por webhook como confiable

La verificación de propiedad debe hacerse en el flujo **directo** de pago (`ProcessPayment`) y el flujo de **webhook** (`HandleWebhook`) que confirma el pago y activa la cuota MUST conservar su comportamiento actual (resuelve el pago por id de Mercado Pago y mantiene la idempotencia con `MarkPaidConditional`).

#### Scenario: Notificación de webhook para pago ya validado

- **WHEN** Mercado Pago notifica un pago aprobado ya validado por su dueño
- **THEN** el backend confirma la cuota, activa la suscripción y marca el pago de
  forma idempotente (la doble notificación no duplica efectos)