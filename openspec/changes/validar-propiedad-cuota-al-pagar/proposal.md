# Propuesta: Validar la propiedad de la cuota al procesar un pago

## Why

Al procesar un pago de cuota (`POST /api/v1/payments`), el backend confía en el
`installment_id` que envía el front sin verificar que la cuota exista ni que
pertenezca al usuario autenticado. Esto permite pagar y activar (o confirmar)
cuotas de **otros usuarios** (`IDOR`) y, si la cuota no existe, dejar un pago de
Mercado Pago aprobado sin poder confirmarse (estado inconsistente).

## What Changes

- `ProcessPayment` valida el `installment_id` recibido **antes** de contactar a
  Mercado Pago:
  - Si la cuota **no existe** → `404` con código tipificado.
  - Si la cuota **existe pero pertenece a otro usuario** (comparando el
    `installment.user_id` contra el usuario autenticado vía
    `utils.GetAuthUserID`) → `403` con código tipificado.
- El controller `paymentController.ProcessPayment` pasa el id del usuario
  autenticado (`utils.GetAuthUserID`) al service para hacer la verificación.
- La validación aplica de forma **uniforme** a cuotas de tier (`subscription_id`)
  y de equipo (`team_id`): `Installment` ya guarda `user_id` en ambas, así el
  chequeo es una sola consulta (`FindByID`) y una comparación.
- La confirmación por **webhook** (`HandleWebhook`) **no** se toca: el backend
  ahí resuelve el pago por id de Mercado Pago (no por input del front), y la
  verificación de propiedad ya quedó hecha en el paso previo. La idempotencia
  (`MarkPaidConditional`) se conserva.
- **No rompe** la API: los campos del request no cambian. Solo se suman respuestas
  de error para casos inválidos que antes pasaban o fallaban tarde.

## Capabilities

### New Capabilities

- `pago-de-cuota-protegido`: garantiza que una cuota solo pueda pagarse por su
  dueño y que la cuota exista, en el concepto de pago de suscripciones
  (tier y equipo).

### Modified Capabilities

- (ninguna — la validación es nueva, no cambia requisitos preexistentes de un spec)

## Impact

- **Código**: `cmd/api/controllers/payment_controller.go`
  (`ProcessPayment`), `cmd/api/services/payment_service.go`
  (`ProcessPayment`), interfaz del service de pagos si hace falta firmar el
  método con el user id, `cmd/api/domains/constants/error_code.go` (dos códigos
  nuevos), y tests de controller + service.
- **DTOs**: no cambian campos de request/response.
- **API**: se agregan errores tipificados `PAYMENT_INSTALLMENT_NOT_FOUND` (404) y
  `PAYMENT_INSTALLMENT_FORBIDDEN` (403) en `POST /api/v1/payments`.
- **Dependencias**: ninguna nueva.
- **Documentación**: la colección de Bruno y el doc del caso de uso ya cubren el
  flujo correcto; no requieren cambios salvo nota de los nuevos errores.
- **Rama**: se implementa y mergea junto con la feature `suscipcion-teams-split`
  (PR #39), antes de que esa feature llegue a `develop`.