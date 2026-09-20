## Why

`GET /api/v1/auth/permissions` arma el tier de cada rol del usuario leyendo `user_roles.tier_id` (`ur.TierID`), un campo denormalizado que se sincroniza recién al activar una suscripción paga. No consulta el ledger `user_role_tier_subscriptions`, que es la fuente de verdad del tier vigente (invariante: a lo sumo una sub vigente por `(user_id, role_id)`). Cuando el ledger diverge del campo denormalizado, el endpoint reporta un tier incorrecto — por ejemplo, devuelve `"tier": "premium_entrenador"` cuando la mejor sub vigente para el rol es `base`.

## Objetivo

Corregir la resolución de tier en `GET /api/v1/auth/permissions` para que use la suscripción vigente del ledger (`user_role_tier_subscriptions.status IN ('active','first_payment_pending')`) como primera fuente de verdad, con `user_roles.tier_id` como fallback — el mismo criterio que ya aplican `GetCurrentSubscription` y `teamConfigurationService.resolveEntrenadorTier`.

## What Changes

- En `GetUserPermissions`, por cada rol se resuelve el tier igual que `GetCurrentSubscription`/`resolveEntrenadorTier`:
  1. Se consulta `TierSubscriptionDao.FindActiveByUserRole(userID, roleID)` (sub en `active` o `first_payment_pending`).
  2. Si existe, `tier = sub.TierID`.
  3. Si no existe, fallback a `ur.TierID`.
- `permissionsQueryService` incorpora `TierSubscriptionDaoInterface` como dependencia nueva; `NewPermissionsQueryService` recibe el DAO extra.
- Se actualiza el wiring de DI en `cmd/api/app/app.go`.
- Se actualizan los tests del servicio: mock del nuevo DAO + casos de sub vigente (tier del ledger gana) y sin sub (fallback a `user_roles.tier_id`).
- Sin cambios en controller, endpoints ni Swagger: la respuesta JSON y el contrato del endpoint no cambian, solo la resolución interna del campo `tier`.
- La validación de datos faltantes se mantiene tal cual (tier no configurado, tier sin permisos asociados, etc.).

## Alcance

- Resolución de tier por rol en el endpoint `GET /api/v1/auth/permissions`.
- Dependencia nueva `TierSubscriptionDaoInterface` en `permissionsQueryService` y su wiring.
- Tests del servicio de permisos.

## No alcance

- Cambiar el contrato JSON del endpoint (forma de la respuesta).
- Modificar otros endpoints que lean permisos o tier, ni la lógica de `GetCurrentSubscription` / `team_configuration_service` (ya resuelven bien).
- Migraciones, cambios de modelo o de constantes de estado.
- Cambiar cómo se sincroniza `user_roles.tier_id` en la activación (línea `first_payment_pending ─► active` del STATE_MACHINES).

## Métrica de éxito

- Para un usuario cuyo ledger `user_role_tier_subscriptions` tiene una sub vigente de tier `X`, el endpoint reporta `tier = X` aunque `user_roles.tier_id` almacene otro valor.
- Todo `go test ./...` en verde.
- La resolución queda alineada con el criterio ya usado en `GetCurrentSubscription` y `resolveEntrenadorTier`.

## Capabilities

### New Capabilities

- `user-permissions`: resolución de permisos y tier efectivo por usuario (`GET /api/v1/auth/permissions`). El spec fija el requerimiento de resolver el tier del rol desde la suscripción vigente del ledger antes que el campo denormalizado.

### Modified Capabilities

Ninguna — no existe un spec previo de permisos (`openspec/specs/` solo tiene `user-bank-alias` y `workout-feedback`).

## Impact

- `cmd/api/services/permissions_query_service.go` — lógica de resolución de tier + nueva dependencia.
- `cmd/api/app/app.go` — wiring de `NewPermissionsQueryService` (1 DAO extra).
- `cmd/api/services/permissions_query_service_test.go` — mock nuevo + casos nuevos; ajuste del constructor en los casos existentes.
- `openspec/specs/user-permissions/spec.md` — spec nuevo.
- El patrón de resolución ya existe en `cmd/api/services/team_configuration_service.go` y `tier_subscription_service.go` (GetCurrentSubscription): esta corrección alinea el endpoint de permisos con ese criterio, no introduce uno nuevo.