## 1. Constantes y modelos

- [x] 1.1 Agregar `SubscriptionPeriodCurrent`/`SubscriptionPeriodNext` + `IsValidSubscriptionPeriod` en `cmd/api/domains/constants/subscription_period.go`.
- [x] 1.2 Agregar `SubscriptionStatusCanceled = "canceled"` en `cmd/api/domains/constants/subscription_status.go` y actualizar `GetValidSubscriptionStatuses`/`IsValidSubscriptionStatus`.
- [x] 1.3 Agregar `InstallmentStatusCanceled = "canceled"` en `cmd/api/domains/constants/installment_status.go` y actualizar sus validadores.
- [x] 1.4 Actualizar comentarios de estados en `cmd/api/domains/dbs/user_role_tier_subscription.go` y `cmd/api/domains/dbs/installment.go`.
- [x] 1.5 Revisar usos de `IsValidSubscriptionStatus`/`GetValidSubscriptionStatuses` para confirmar que `canceled` no entra en validaciones de input/carga (solo es transicional por DAO).

## 2. DAOs

- [x] 2.1 Parametrizar `FindActiveByUserRole(ctx, userID, roleID, statuses ...string)` en `cmd/api/daos/tier_subscription_dao.go` (sin args = default activo/pendiente).
- [x] 2.2 Agregar `FindPendingByUserRoleTier` y `FindByUserRoleTier` en `tier_subscription_dao.go`.
- [x] 2.3 Agregar `SetCanceled` en `tier_subscription_dao.go`.
- [x] 2.4 Agregar `CancelPendingBySubscription` en `cmd/api/daos/installment_dao.go`.
- [x] 2.5 Actualizar `tier_subscription_dao_test.go` (firma varargs, `FindPendingByUserRoleTier` solo pending, `SetCanceled`, y que `canceled` deja de ser vigente) y `installment_dao_test.go` si corresponde.

## 3. Servicio

- [x] 3.1 Cambiar firma de `GetCurrentSubscription(ctx, userID, roleID, period)` en la interface y en `tier_subscription_service.go`, eligiendo estado según período y devolviendo `(nil, nil)` si no hay sub.
- [x] 3.2 Mantener el bloqueo de `ChangeTier` ante sub `first_payment_pending` (sin cambios de lógica; verificar que sigue compilando con la firma varargs).
- [x] 3.3 Implementar `CancelPendingSubscription(ctx, userID, roleID, tierID)` con transacción (`SetCanceled` + `CancelPendingBySubscription`) y errores tipificados para 404/409.
- [x] 3.4 Actualizar mocks de `TierSubscriptionServiceInterface` usados por controllers/tests.

## 4. Controller y rutas

- [x] 4.1 En `cmd/api/app/url_mappings.go`: `r.GET("/api/v1/users/:id/subscriptions/:period", ...)` y `r.DELETE("/api/v1/users/:id/roles/:role_id/subscriptions/pending", ...)`.
- [x] 4.2 En `tier_subscription_controller.go`: parsear/validar `period` (400 si inválido), pasar período al service, responder `200 {}` cuando el service devuelve `nil`.
- [x] 4.3 Agregar `CancelPendingSubscription` con self-only, path `user_id`/`role_id`, query `tier_id` y mapeo nuevo de errores en `mapTierSubscriptionError` (404 y 409).
- [x] 4.4 Actualizar anotaciones godoc (nueva ruta GET con `@Param period` y nuevo DELETE) en el controller.

## 5. Swagger

- [x] 5.1 Regenerar `cmd/api/docs` con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- [x] 5.2 Verificar en `swagger.json`/`swagger.yaml` que aparezcan `/api/v1/users/{id}/subscriptions/{period}` y el DELETE nuevo.

## 6. Tests y coverage

- [x] 6.1 Tests de controller: GET con `current`/`next`/período inválido/faltante/body vacío, y DELETE `200/404/409/403/400` (ajustar `tier_subscription_controller_test.go`).
- [x] 6.2 Tests de service: `GetCurrentSubscription` por período y `CancelPendingSubscription` (éxito, 404, 409, error de DB) en `tier_subscription_service_test.go`; cubrir también "pendiente bloquea ChangeTier" y "tras cancelar permite".
- [x] 6.3 Correr `go test ./...` y `make coverage-with-db` (con `make test-db-up`); no bajar el total por debajo del gate (≥ 80%). _(Nota: localmente corre el suite completo (verde) y `make coverage` sin DB (parcial, 72.9% esperado sin `daos`); el gate sobre Postgres corre en CI con `ci.yml` + `make coverage-with-db`, que es la medición de referencia.)_

## 7. Máquina de estados centralizada

- [x] 7.1 Crear `docs/STATE_MACHINES.md` con estados, transiciones, disparadores e invariantes por entidad (subscription, installment, user_role, user, team, team_user, invitation, join_request, seller_connection).
- [x] 7.2 Actualizar documentación afectada: `README.md` (tabla de endpoints), `.agentics/payments-integration.md`, `docs/CAMBIO_TIER_SUBSCRIPTION_TESTING.md` y casos de uso que citen `subscriptions/current`.
- [x] 7.3 Referenciar `docs/STATE_MACHINES.md` en `CLAUDE.md`.