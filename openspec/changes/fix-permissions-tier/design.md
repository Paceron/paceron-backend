## Context

`GET /api/v1/auth/permissions` (`permissionsQueryController` → `permissionsQueryService.GetUserPermissions`) devuelve, por cada rol del usuario, el tier y sus permisos. Hoy resuelve el tier leyendo `user_roles.tier_id` (`ur.TierID`), un campo denormalizado que se sincroniza recién al activar una suscripción paga (`first_payment_pending ──► active`). No consulta el ledger `user_role_tier_subscriptions`, que es la fuente de verdad del tier vigente (invariante: a lo sumo una sub vigente por `(user_id, role_id)`). Cuando el ledger diverge del campo denormalizado, el endpoint reporta un tier incorrecto (bug reportado: `premium_entrenador` cuando la sub vigente es `base`).

El patrón canónico de resolución ya existe en el código y no varía según la entidad:

1. Sub vigente (`FindActiveByUserRole`, status `active` + `first_payment_pending`).
2. Si existe → `sub.TierID`.
3. Si no existe → `user_roles.tier_id`.

Lo usan `teamConfigurationService.resolveEntrenadorTier` (`team_configuration_service.go:62`) y es el mismo criterio que documenta `GetCurrentSubscription` (`tier_subscription_service.go`). Este change alinea `GetUserPermissions` con ese patrón.

## Goals / Non-Goals

**Goals:**
- Que `GetUserPermissions` reporte el tier de la suscripción vigente del ledger cuando existe, con `user_roles.tier_id` como fallback.
- Mantener intacto el contrato JSON del endpoint y el manejo de datos faltantes.
- Mantener la suite de tests en verde (`go test ./...`).

**Non-Goals:**
- No sincronizar/actualizar `user_roles.tier_id` en ningún flujo (la sync en activación queda como está).
- No modificar otros endpoints que lean tier, ni `GetCurrentSubscription`, ni `team_configuration_service`.
- No cambiar estados, constantes, migraciones ni schema.

## Decisions

### D1. Resolver el tier por rol: sub vigente primero, fallback a `user_roles.tier_id`

En el loop de `GetUserPermissions`, por cada `ur`:

```go
tierID := ur.TierID
sub, err := s.tierSubDao.FindActiveByUserRole(ctx, userID, ur.RoleID)
if err != nil {
    // error genérico "error al obtener permisos", igual que el resto
}
if sub != nil {
    tierID = sub.TierID
}
tier, err := s.tierDao.FindByID(ctx, tierID)
```

- `FindActiveByUserRole` sin statuses explícitos usa el default `['active', 'first_payment_pending']`, idéntico a `resolveEntrenadorTier`.
- El resto del flujo (missingData si el tier no existe, permisos del tier, etc.) queda inalterado.

**Alternativas descartadas:**
- **Sincronizar `user_roles.tier_id` en cada punto de divergencia**: más superficie de cambio, frágil, y contradice "el ledger es la fuente de verdad". El campo denormalizado es un caché del ledger, no al revés.
- **Resolver el "mejor" tier entre sub y `user_roles.tier_id` por jerarquía**: no tiene sentido: el invariante garantiza una sola sub vigente por `(user_id, role_id)`, no hay comparación que hacer. El ledger manda.

### D2. Nueva dependencia `TierSubscriptionDaoInterface` en el servicio

`permissionsQueryService` incorpora `tierSubDao daos.TierSubscriptionDaoInterface`. `NewPermissionsQueryService` recibe el DAO como parámetro nuevo y se propaga en el wiring de `app.go:182`. Es una inyección de DAO (capa permitida por CONVENTIONS), sin service-to-service imports.

### D3. Sin helper compartido (por ahora)

Existe duplicación del patrón en 3 lugares (`GetCurrentSubscription`, `resolveEntrenadorTier`, este fix). Se evaluó extraer un resolver común, pero tocar `team_configuration_service` y `tier_subscription_service` excede el alcance de este bug fix. Se replica el patrón literal y se dejan los 3 puntos alineados; unificar en un resolver reutilizable queda como refactor futuro si el equipo lo quiere.

### D4. Semántica de `first_payment_pending`

Una sub `first_payment_pending` cuenta como "vigente": su `TierID` (tier pago) se reporta aunque los permisos pagos no estén aun habilitados en la práctica. Esto replica exactamente lo que ya hace `resolveEntrenadorTier` y respeta la definición de "sub vigente" del STATE_MACHINES (índice único parcial cubre `active` y `first_payment_pending`). No se introduce una semántica nueva.

## Risks / Trade-offs

- [Una sub `first_payment_pending` reporta tier/permissions pago mientras el acceso pago no está efectivo] → Coherente con `resolveEntrenadorTier` presente; si el equipo quiere semántica estricta "solo `active`", es una decisión de negocio a documentar en otro change, tocaría varios servicios a la vez.
- [Consulta extra por rol (N+1)] → El endpoint ya hace queries por rol (role, tier, tier_permissions, permission); una más no cambia la complejidad. Aceptable para un endpoint de consulta del login/perfil.
- [Dependencia nueva rompe tests existentes del servicio] → Se actualizan con un mock `TierSubscriptionDaoInterface` y casos nuevos; el constructor se ajusta en todos los tests de `permissions_query_service_test.go`.

## Migration Plan

- Sin migración de datos ni de schema.
- Deploy normal de la rama; rolback = revertir el diff (la respuesta vuelve al comportamiento anterior, sin riesgo de datos).

## Open Questions

- Ninguna para este change. Si más adelante se decide que "tier efectivo" = solo `active` (descartando `first_payment_pending`), afecta a `GetCurrentSubscription`, `resolveEntrenadorTier` y este endpoint — evaluar en un change dedicado.