# Design: Suscripciones de tier — periodo `current`/`next` y cancelación de pago pendiente

## Context

El ledger de suscripciones de tier (capability `tier-subscriptions`) expone hoy un único GET `GET /api/v1/users/:id/subscriptions/current` (controller `tier_subscription_controller.go`, service `tier_subscription_service.GetCurrentSubscription`, DAO `tier_subscription_dao.FindActiveByUserRole`) que devuelve la sub vigente (`active` **o** `first_payment_pending`) y su próxima cuota. Problemas detectados en el flujo real:

- El frontend no puede distinguir una sub `first_payment_pending` de una `active` con una URL "current", porque el endpoint mezcla ambos estados.
- Un `first_payment_pending` colgado (cambio de tier hacia pago cuya cuota #1 nunca se pagó) **bloquea** el `PUT tier` con `409 SUBSCRIPTION_PENDING_FIRST_PAYMENT` y no existe ninguna vía para salir de ese estado (ni resolver ni cancelar).
- La máquina de estados de cada entidad está dispersa en archivos de constantes sin una vista legible de transiciones.

Este cambio: hace la URL de consulta paramétrica (`:period` = `current`|`next`), agrega la cancelación de la sub con primer pago pendiente (nuevo estado terminal `canceled` en sub y cuota), no toca la lógica de cambio de tier salvo documentar/mantener el bloqueo existente, y deja la máquina de estados documentada y centralizada.

## Goals / Non-Goals

**Goals:**

- `GET /api/v1/users/:id/subscriptions/:period` con `current` (sub `active`) y `next` (sub `first_payment_pending`); sin sub en el estado → `200 {}`.
- `DELETE /api/v1/users/:id/roles/:role_id/subscriptions/pending?tier_id=` que cancela la sub `first_payment_pending` de la terna (sub + cuotas → `canceled`) y habilita reintentar el cambio de tier.
- Mantener explícito el bloqueo de `ChangeTier` ante una sub pendiente (`409 SUBSCRIPTION_PENDING_FIRST_PAYMENT`).
- Nueva referencia única de máquina de estados sin renombrar estados existentes.
- Swagger regenerado y tests/coverage en verde.

**Non-Goals:**

- No se toca el webhook de pagos / `applyApprovedInstallment` / `UpdateTier`.
- No se agrega historial paginado ni endpoints de listado.
- No se modifican las validaciones de deuda ni la firma de `ChangeTier`.
- No se relaja el índice único parcial.

## Decisions

### D1. Ruta con `:period` y parsing en el controller

`url_mappings.go`:

```go
r.GET("/api/v1/users/:id/subscriptions/:period", app.tierSubscriptionController.GetCurrentSubscription)

r.DELETE("/api/v1/users/:id/roles/:role_id/subscriptions/pending", app.tierSubscriptionController.CancelPendingSubscription)
```

El handler `GetCurrentSubscription` parsea `c.Param("period")`. Valores válidos: `current` y `next` (nuevas constantes en `domains/constants/subscription_period.go`: `SubscriptionPeriodCurrent`, `SubscriptionPeriodNext` + `IsValidSubscriptionPeriod`). Valor inválido o ausente → `400`. `role_id` sigue siendo query param obligatorio (no cambia). El atributo gin wildcard `:period` matchea cualquier segmento, así `.../current` y `.../next` siguen funcionando sin conflictos (no hay otras rutas bajo `/subscriptions/`).

**Alternativas consideradas:** dos endpoints separados (`/current`, `/next`) — se descarta por pedido explícito del usuario de un solo parámetro; query param `?period=` — se descarta porque la consigna pide URL param.

### D2. Propagar el estado al DAO: `FindActiveByUserRole` parametrizable

Se mantiene el método `FindActiveByUserRole` (pedido explícito en la consigna) pero con varargs de estados:

```go
FindActiveByUserRole(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error)
```

- Con argumentos: `WHERE user_id=? AND role_id=? AND status IN (?...)`.
- Sin argumentos (default): se comporta igual que hoy (`IN ('first_payment_pending','active')`), así `ChangeTier` (línea 97) **no cambia**.

Llamadas en el service:

- `current` → `FindActiveByUserRole(ctx, uid, rid, string(constants.SubscriptionStatusActive))`
- `next` → `FindActiveByUserRole(ctx, uid, rid, string(constants.SubscriptionStatusFirstPaymentPending))`

La interface `TierSubscriptionServiceInterface.GetCurrentSubscription` cambia su firma a `GetCurrentSubscription(ctx, userID, roleID int64, period string)` (el controller pasa el período parseado y validado). Se actualizan mocks y tests de la interface.

### D3. "Mayor hierarchy" queda cubierto por el índice único parcial

El requisito "sub `active` de mayor `hierarchy`" se satisface sin JOIN contra `tiers`: el índice único parcial `uq_sub_ids_user_role_active` (postgres.go) garantiza `status IN ('active','first_payment_pending')` → a lo sumo **un** registro por `(user_id, role_id)`. Con a lo sumo una sub activa, elegir "la de mayor hierarchy" es no ambiguo. Se documenta que si algún día se relaja el índice, habrá que agregar el ordenamiento por jerarquía.

### D4. Respuesta `200 {}` cuando no hay sub en el período

Hoy, sin sub vigente, `GetCurrentSubscription` devuelve el estado `tier`/`rol` del usuario. Con la nueva semántica: si no existe sub del estado pedido, el service devuelve `(nil, nil)` y el controller responde `c.JSON(http.StatusOK, gin.H{})` → body literal `{}`.

`CurrentSubscriptionResponse` no se toca (ni sus tags): cuando hay sub se devuelve igual que hoy. Esto es un **BREAKING de comportamiento** para `current` en roles gratis sin sub activa (antes `tier`/`role`, ahora `{}`) — se coordina con el frontend.

**Alternativas consideradas:** devolver el struct cero — descartado porque `tier`/`role` sin `omitempty` ensucian el body con ceros; tocar los tags del DTO — descartado por cambiador de contrato de baja ganancia.

### D5. Cancelación de pendiente: estados `canceled` y flujo

Nuevos estados en constantes (sin renombrar nada):

- `subscription_status.go`: `SubscriptionStatusCanceled = "canceled"`.
- `installment_status.go`: `InstallmentStatusCanceled = "canceled"`.

Métodos nuevos de DAO:

```go
// tier_subscription_dao.go
FindPendingByUserRoleTier(ctx, userID, roleID, tierID) (*Sub, error) // status = first_payment_pending, para la terna
FindByUserRoleTier(ctx, userID, roleID, tierID) (*Sub, error)        // la última (por id) de la terna, cualquier estado — para distinguir 404/409
SetCanceled(ctx, id) (error)                                          // status = canceled

// installment_dao.go
CancelPendingBySubscription(ctx, subscriptionID) (error)              // status pending → canceled
```

Service `CancelPendingSubscription(ctx, userID, roleID, tierID)`:

1. `sub := FindPendingByUserRoleTier(...)` → error genérico si DB falla.
2. `sub == nil`:
   - `existing := FindByUserRoleTier(...)`. Si `existing == nil` → error `"suscripción no encontrada"` → `404`.
   - Si existe la terna pero no está pendiente → error `"la suscripción no está en primer pago pendiente"` → `409`.
3. `sub != nil`: `SetCanceled(sub.ID)` y `CancelPendingBySubscription(sub.ID)` en una transacción GORM (mismo patrón `apply` + `s.db.Transaction` de `ChangeTier`), log con `customlogger`.
4. Respuesta `200` con el id/estado de la sub cancelada (reusa `CurrentSubscriptionResponse` o un DTO chico `{subscription_id, subscription_status}`).

Controller `CancelPendingSubscription`: self-only (mismo `forbiddenNotSelfSubscription`), parsea `user_id`/`role_id` de path y `tier_id` de query (`?tier_id=`), mapea los errores nuevos en `mapTierSubscriptionError` (agregar casos `"suscripción no encontrada"` → 404, `"la suscripción no está en primer pago pendiente"` → 409).

**Alternativa considerada:** un solo UPDATE condicional `WHERE ... AND status='first_payment_pending'` sin pre-consulta — descartado porque no distingue `404` (terna inexistente) de `409` (terna en otro estado).

### D6. El bloqueo de `ChangeTier` se mantiene y queda documentado

La validación ya existe (líneas 119-121: `sub.Status == first_payment_pending` → `409 SUBSCRIPTION_PENDING_FIRST_PAYMENT`). No se cambia código; se agregan tests de service que cubren el escenario "pendiente bloquea" y "tras cancelar, permite" (con `FindActiveByUserRole` retornando nil tras la cancelación). Esto hace explícita la MODIFICACION DOS de la consigna y la conecta con la cancelación.

### D7. Máquina de estados centralizada y legible

Nuevo documento único `docs/STATE_MACHINES.md` con, por entidad: estados válidos (constantes), transiciones permitidas y disparadores, e invariantes (ej. índice único parcial, qué libera el slot). Entidades a documentar: `subscription` (tier y team en una sola vista), `installment`, `user_role`, `user`, `team`, `team_user`, `invitation`, `join_request`, `seller_connection`. Se mantienen las constantes como fuente de verdad (no se duplican en código); el doc es referencia. Se agrega una referencia cruzada desde `CLAUDE.md` (sección Quirks/Testing) y se actualiza al cerrar el change.

### D8. Swagger y pruebas

- Anotaciones godoc en el controller: `@Router /api/v1/users/{id}/subscriptions/{period} [get]` (con `@Param period path string true "current|next"`) y `@Router /api/v1/users/{id}/roles/{role_id}/subscriptions/pending [delete]`.
- Regenerar con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` (ver `.agentics/WORKFLOW.md`).
- Tests: controller (período válido/inválido/faltante, body vacío, DELETE 200/404/409/403/400), service (`GetCurrentSubscription` con período, `CancelPendingSubscription`, bloqueo de tier), dao (Postgres real vía `testutils.SetupTestDB`, incl. que `canceled` deja de ser "vigente" y libera el índice).
- Coverage: mantener ≥ 80% (`make coverage-with-db`).

## Risks / Trade-offs

- [BREAKING de comportamiento en `current` para roles sin sub activa] → Coordinado con el frontend; la nueva semántica es la pedida (`200 {}`). Los casos con sub siguen idénticos.
- [Cambio de firma de `FindActiveByUserRole` rompe callers/tests] → Varargs con default preserva el call site de `ChangeTier`; los tests de DAO se actualizan.
- [GET con `:period` colisiona con futuras rutas bajo `/subscriptions/`] → Por ahora es la única bajo ese prefijo; si se agregan más, se ordenan las de mayor especificidad primero.
- [`IsValidSubscriptionStatus`/`GetValidSubscriptionStatuses` pasan a aceptar `canceled`] → Se revisan los usos (tiers/subscripciones) para que validaciones de input no acepten `canceled` en cargas; el DAO solo lo escribe transicionalmente.
- [Cancelar cuotas `pending` de la sub incluye potencialmente cuotas con `blocked_date` vencido] → La cancelación es exactamente el escape para esos casos; el dinero no cobrado se pierde por diseño (estado cancelado terminal). Se valida confirmar con negocio si aplica reembolso por webhook — fuera de alcance.
- [Borrar la sub pendiente deja el tier de acceso (`user_roles.tier_id`) sin suscripción] → El acceso sigue reflejado en `user_roles` (tier base actual); se respeta el estado actual del ledger sin modificar `user_roles`.

## Migration Plan

- No hay cambios de schema (solo estados en columnas existentes; el índice único parcial ya excluye `canceled` por su `WHERE`).
- Deploy: feature branch → `develop` (CI verde + coverage ≥ 80%), luego release a `master`. Backward-incompatible solo en la URL del GET (el parámetro `current` conserva el valor análogo); se libera junto con la actualización del frontend.
- Rollback: revert del merge; los estados `canceled` existentes quedan como historial inerte (no rompen queries que filtran por los estados vigentes).

## Open Questions

- ¿Debe `current` seguir devolviendo `tier`/`role` para el caso "rol gratis con tier base y sin sub" (para que el frontend siga mostrando el tier actual) en lugar de body vacío? Asumido `{}` según consigna; pendiente confirmación con negocio/frontend.
- En `DELETE pending`, si el frontend prefiere tier en el body en vez de query param, se adapta (decisión por defecto: query `tier_id` por simplicidad del DELETE).