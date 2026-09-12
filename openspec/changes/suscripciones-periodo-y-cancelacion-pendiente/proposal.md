# Propuesta: Suscripciones de tier — periodo current|next y cancelación de pago pendiente

## Why

El endpoint `GET /api/v1/users/:id/subscriptions/current` solo expone la suscripción vigente, y una sub en `first_payment_pending` (resultado de un cambio de tier o de una asignación de rol pago cuyo primer pago nunca se concretó) es indistinguible de la activa para el frontend: hoy no hay forma de saber si quedó un primer pago sin resolver ni de salir de ese estado. Además, un `first_payment_pending` colgado bloquea cualquier intento de cambio de tier (`409 SUBSCRIPTION_PENDING_FIRST_PAYMENT`) sin vía de escape, y los estados de las entidades están dispersos en constantes difíciles de leer.

## What Changes

- **Período en la URL de consulta** (`GET /api/v1/users/:id/subscriptions/:period`): donde antes estaba fijo `current`, ahora la ruta usa `:period` y el mismo handler decide qué sub devolver según el valor:
  - `current` → sub en `status = active` de ese `(seller, role_id)` (la de mayor `hierarchy` del tier; el índice único parcial garantiza a lo sumo una).
  - `next` → sub en `status = first_payment_pending` de ese `(seller, role_id)`.
  - Si no existe sub en ese estado → `200` con body vacío (`{}`).
  - `period` inválido → `400`. `role_id` sigue siendo query param obligatorio, igual que hoy.
  - Se propaga el `status` a filtrar hasta la query del DAO (`FindActiveByUserRole` se parametriza por estado).
  - **BREAKING de comportamiento** para `current`: cuando no hay sub `active` ya no se devuelve el tier/rol del rol gratis — se devuelve `{}` según lo pedido.
- **Validación de primer pago pendiente en el cambio de tier** (`PUT /api/v1/users/:id/roles/:role_id/tier`): se mantiene y documenta explícitamente la regla que ya existe — si para ese `(user_id, role_id)` ya hay una sub `first_payment_pending`, el cambio se rechaza con `409` + `SUBSCRIPTION_PENDING_FIRST_PAYMENT`. Se cubre con tests del comportamiento resultante.
- **Cancela suscripción en `first_payment_pending`** (nuevo `DELETE /api/v1/users/:id/roles/:role_id/subscriptions/pending?tier_id=`): recibe `user_id` + `role_id` (path) y `tier_id` (query). Si existe una sub `first_payment_pending` para esa terna, la pasa a `canceled` y sus cuotas `pending` a `canceled`; **liberando** el slot del índice único parcial (solo cubre `active`/`first_payment_pending`) y habilitando reintentar el cambio de tier. Si no existe → `404`; si existe pero está en otro estado (ej. `active`) → `409`.
- **Nuevo estado `canceled`** para suscripciones (`SubscriptionStatusCanceled`) y para cuotas (`InstallmentStatusCanceled`). No se renombra ningún estado existente: la cancelación es terminal e intermedia entre la creación (`first_payment_pending`) y el resto del ciclo.
- **Máquina de estados legible y centralizada**: nueva referencia única `docs/STATE_MACHINES.md` que documenta, por entidad, los estados, las transiciones válidas y qué las dispara (subscriptions, installments, user_roles, users, teams, etc.). Constantemente actualizada; sin renombrar estados.
- **Swagger y tests**: se regenera `cmd/api/docs` (ruta nueva del GET + endpoint DELETE nuevo) y se suman/ajustan unit tests (controller/service/dao) para que el coverage no baje del gate.

## Capabilities

### New Capabilities

- (ninguna — todo el comportamiento pertenece a la capability de suscripciones de tier ya existente)

### Modified Capabilities

- `tier-subscriptions`: cambia el requisito de "Endpoint de próxima cuota a pagar" (ahora con `:period` `current`/`next` y body vacío cuando no hay sub en ese estado), agrega el requisito de "cancelación de suscripción con primer pago pendiente" y ajusta el ciclo de vida de estados (nuevo estado terminal `canceled` para sub y cuota).

## Impact

- **Rutas**: `cmd/api/app/url_mappings.go` — GET `/api/v1/users/:id/subscriptions/current` → `/api/v1/users/:id/subscriptions/:period`; nuevo DELETE `/api/v1/users/:id/roles/:role_id/subscriptions/pending`.
- **Controller**: `cmd/api/controllers/tier_subscription_controller.go` — parseo de `:period`, validación de valor, handler nuevo `CancelPendingSubscription`, anotaciones swagger.
- **Servicio**: `cmd/api/services/tier_subscription_service.go` — `GetCurrentSubscription` recibe el período/estado y devuelve `{}` si no hay sub; nuevo `CancelPendingSubscription`.
- **DAOs**: `cmd/api/daos/tier_subscription_dao.go` — `FindActiveByUserRole` parametrizada por estado y `CancelPendingByUserRoleTier`; `cmd/api/daos/installment_dao.go` — cancelar cuotas de una sub.
- **Constantes**: `cmd/api/domains/constants/subscription_status.go` y `installment_status.go` — estado `canceled`; `error_code.go` si hiciera falta un código de error nuevo.
- **Modelos**: comentarios en `cmd/api/domains/dbs/user_role_tier_subscription.go` y `installment.go` (nuevo estado en la doc). Sin migración: son estados, no columnas ni índices (el índice único parcial ya excluye `canceled` por su `WHERE`).
- **Documentación**: nuevo `docs/STATE_MACHINES.md`; se actualizan `README.md` (tabla de endpoints), `.agentics/payments-integration.md`, `docs/CAMBITIER_SUBSCRIPTION_TESTING.md`/casos de uso que citan `subscriptions/current`.
- **Swagger**: se regenera `cmd/api/docs` con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- **Tests**: controller, service y dao (+ `_test.go` reales contra Postgres si corresponde); correr `go test ./...` y coverage verde.
- **API**: el GET es **BREAKING** de contrato (cambia la URL de `.../current` a `.../:period`; `current` sigue siendo válido como valor). El frontend llama períodos `current` o `next`.

## No Alcance

- No se toca el webhook de pagos ni `applyApprovedInstallment`/`UpdateTier`.
- No se agregan ni renombran otros estados de entidades que no sean los dos `canceled` nuevos.
- No cambia `PUT tier` en su firma ni su lógica de validación de deuda.
- No se implementa paginación/historial de suscripciones.

## Métrica de éxito

- `go test ./...` en verde; coverage ≥ 80% (gate de `.testcoverage.yml`).
- Los endpoints `?period=current`, `?period=next` y el DELETE de pendiente responden como especifica este change en una base real (tests de DAO + smoke con `testutils.SetupTestDB`).
- Un usuario con `first_payment_pending` colgado puede: consultarlo con `next`, cancelarlo con el DELETE, y reintentar el cambio de tier sin bloqueo.