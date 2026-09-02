## Why

Los tiers de cada rol necesitan cobrarse (suscripción paga) y controlarse: hoy `POST /api/v1/users/:id/roles` asigna rol+tier sin registrar cómo se paga ni permite cambiar de tier con reglas de negocio. Sin un ledger de suscripciones y cuotas, no se puede validar deuda antes de un cambio de tier, ni saber si el acceso a un tier está efectivamente habilitado.

## What Changes

- **Nueva tabla** `user_role_tier_subscriptions` (historial/ledger de suscripciones de tier por usuario/rol). **Nueva tabla** `installments` (cuotas) con diseño de **arco exclusivo**: referencia a la suscripción (`subscription_id`) o a un equipo (`team_id`, para la futura suscripción de corredor → entrenador, que va en otro change) — nunca ambas. La columna `team_id` queda definida desde ahora para no migrar la tabla después; el flujo de equipos se especifica en `suscripcion-teams-split`.
- **Nueva columna** `hierarchy` (int) en `tiers` para ordenar base < medium < premium. El concepto de tier "gratis" deja de derivarse del nombre: se gobierna por `payment_required` (ya existente); al crear/actualizar un tier `base` se fuerza `payment_required = false`.
- **AssignRole con tier pago** (`POST /api/v1/users/:id/roles`): crea suscripción con `status = first_payment_pending` + cuota #1 `pending` sin `due_date`, solo cuando `tier.PaymentRequired == true`. El acceso al tier no se activa hasta que la primera cuota quede pagada.
- **Nuevo endpoint de cambio de tier** (mismo rol, hacia arriba o hacia abajo): valida (1) target tier del mismo rol, (2) sin deuda — no existe cuota `pending` vencida/superada de la suscripción vigente. Cierra la suscripción anterior (`ended`), crea la nueva y, si es paga, genera la cuota #1.
- **Endpoint "próxima cuota a pagar"** (`user_id` + `role`): devuelve `subscription_id`, `subscription_status`, `installment_id`, `installment_number`, `installment_amount`, `next_due_date`, `blocked_date` + datos del tier/rol y la `public_key` de MP para armar el checkout Bricks, sin procesar el pago en esta etapa.
- **Webhook de pago**: al confirmar un pago, marca la cuota correspondiente como `paid` (update condicional sobre `status = pending` para idempotencia), incrementa `paid_installments` de la suscripción y, si era la cuota #1, pasa la suscripción a `active`.

## Capabilities

### New Capabilities

- `tier-subscriptions`: suscripciones de tier por usuario/rol (creación en AssignRole, ciclo de vida `first_payment_pending`/`active`/`ended`), cuotas (`installments`), validación de deuda, cambio de tier con misma regla de rol y jerarquía, y endpoint de próxima cuota a pagar.

### Modified Capabilities

- Ninguna: `user-bank-alias` no cambia; el comportamiento de permisos/servicios existentes se mantiene.

## Impact

- **Schema/DB**: `tiers.hierarchy`; nuevas tablas `user_role_tier_subscriptions` e `installments` (con `subscription_id` y `team_id` nullable + CHECK de arco exclusivo); AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- **Modelos**: `cmd/api/domains/dbs/tier.go`, `dbs/user_role_tier_subscription.go`, `dbs/installment.go`.
- **Dominios/DTOs**: nuevos `domains/tiersubscription/` (request/response) y ajustes en `domains/userrole/` (AssignRole), `domains/payment/` (ayerout a suscripción/cuota).
- **Servicios**: `services/user_role_service.go` (AssignRole), nuevos `services/tier_subscription_service.go` (cambio de tier, próxima cuota), `services/payment_service.go` (webhook → installments).
- **DAOs**: nuevos `daos/tier_subscription_dao.go`, `daos/installment_dao.go`.
- **Controllers/rutas**: `controllers/tier_subscription_controller.go`; rutas nuevas en `cmd/api/app/url_mappings.go`.
- **API**: endpoints nuevos (cambio de tier, próxima cuota); `POST /api/v1/users/:id/roles` modifica su comportamiento para tiers pagos. **BREAKING** de comportamiento (no de contrato): al asignar un tier pago, el acceso no queda activo hasta el primer pago.
- **Integración**: Mercado Pago (webhook existente) para marcar cuotas pagadas; checkout Bricks en el frontend (consumidor del payload de próxima cuota).
- **Swagger**: regenerar `cmd/api/docs` con los endpoints nuevos.
- **Tests**: unitarios (services/controllers/daos con mocks) y DAO con Postgres real vía `testutils.SetupTestDB` para constraints de unicidad.