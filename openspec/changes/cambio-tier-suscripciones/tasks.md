# Tasks

## Tasks

### 1. Modelos de DB y migración

- [ ] Crear `cmd/api/domains/dbs/user_role_tier_subscription.go` (modelo GORM según D1: status, init_amount, paid_installments, start_date, ended_date).
- [ ] Crear `cmd/api/domains/dbs/installment.go` (modelo GORM según D1: subscription_id/team_id nullable, user_id, installment_number, status, internal/external_payment_id, amount, due_date, blocked_date).
- [ ] Agregar `hierarchy` a `cmd/api/domains/dbs/tier.go` (ya existen `payment_required`/`tier_amount`).
- [ ] Agregar `installment_id` nullable (FK → installments.id) a `cmd/api/domains/dbs/payment.go`.
- [ ] Registrar los modelos nuevos en AutoMigrate (`cmd/api/infrastructure/postgresdb/postgres.go`).
- [ ] Agregar SQL crudo post-AutoMigrate: índice único parcial `(user_id, role_id) WHERE status IN ('active','first_payment_pending')` y CHECK `num_nonnulls(subscription_id, team_id) = 1` en `installments`.
- [ ] Versión corta: validar con `go build ./...` y test de DAO con Postgres real.

### 2. DAOs

- [ ] Crear `cmd/api/daos/tier_subscription_dao.go`: `Create`, `FindActiveByUserRole` (status en active/first_payment_pending), `SetEnded`, `IncrementPaidInstallments`, `FindLatestByUserRole`.
- [ ] Crear `cmd/api/daos/installment_dao.go`: `Create`, `MarkPaidConditional` (update `WHERE status='pending'`), `FindPendingBySubscription`, `FindPendingByUserTeam` (para change 2), `FindNext`.
- [ ] Tests unitarios con mocks para ambos DAOs.

### 3. DTOs y dominios

- [ ] Crear `cmd/api/domains/tiersubscription/` con request `ChangeTierRequest {tier_id}` y response de próxima cuota/estado (shape de D9).
- [ ] Agregar `installment_id` (opcional) a `CreatePreferenceRequest` y `ProcessPaymentRequest` en `cmd/api/domains/payment/payment.go`.
- [ ] Agregar constantes de concepto `subscription` (cuota de tier) reutilizando/ampliando las existentes.
- [ ] Agregar custom codes de error (D11): `TIER_ROLE_MISMATCH`, `SUBSCRIPTION_PENDING_FIRST_PAYMENT`, `DEBT_BLOCKS_OPERATION`.

### 4. Servicios

- [ ] `user_role_service.go` AssignRole: si `payment_required=true`, crear `user_roles` con tier de menor jerarquía del rol + suscripción `first_payment_pending` + cuota #1, todo en una transacción GORM.
- [ ] Crear `cmd/api/services/tier_subscription_service.go`: `ChangeTier` (validaciones D4 en orden + transacción) y `GetCurrentSubscription` (próxima cuota según D9).
- [ ] `payment_service.go`: crear preference/process payment seteando `installment_id` cuando viene; webhook → lógica D7 (marcar cuota, contador, activación, sync `tier_id`, generar cuota siguiente).
- [ ] Tests unitarios de los servicios (testify + mocks).

### 5. Controllers y rutas

- [ ] Crear `cmd/api/controllers/tier_subscription_controller.go`: `PUT /api/v1/users/:id/roles/:role_id/tier` y `GET /api/v1/users/:id/subscriptions/current`.
- [ ] Registrar rutas en `cmd/api/app/url_mappings.go`.
- [ ] Tests de controller.

### 6. Webhook / integración MP

- [ ] Verificar que los endpoints de preference y ProcessPayment propagan `installment_id` y que el webhook recibe el mapping correcto en Bruno (pago con tarjeta de test + doble notificación).
- [ ] Documentar flujo Bricks (frontend): `GET current` → `POST preference` con `installment_id` → Bricks.

### 7. Swagger y documentación

- [ ] Regenerar `cmd/api/docs` con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- [ ] Actualizar `README.md` (tabla de endpoints) y `.agentics/payments-integration.md` si aplica.

### 8. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` en verde.
- [ ] Validar escenarios de la spec con tests (asignación paga/gratis, activación por webhook, doble notificación, cambio con/sin deuda, cambio cross-rol, cuota #1 sin vencimiento, índice único parcial).