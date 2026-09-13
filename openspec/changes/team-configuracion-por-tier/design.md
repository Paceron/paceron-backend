## Context

El tier de un usuario se modela por rol: `user_roles` tiene `tier_id` por (user, role) y el pago se registra en `user_role_tier_subscriptions` (ledger por user/rol con status `first_payment_pending`/`active`/`ended`). El rol del dueño de equipo se resuelve por nombre (`roles.name = "entrenador"`, const `teamOwnerRoleName` en `cmd/api/services/team_service.go`). `GET /users/:id/subscriptions/current` ya resuelve el tier vigente para un rol: sub activa si existe, si no `user_roles.tier_id`.

El frontend necesita, al crear/editar un equipo, los topes que su plan le permite. Esa config es **código** (no tabla): un hashmap tier → `{max_members, minimum_fee}`, editable en un solo archivo. Como el endpoint consulta el tier del propio entrenador, la identidad sale del **access token** (no hace falta `user_id` ni `team_id`).

## Goals / Non-Goals

**Goals:**
- `GET /api/v1/team-configuration` (autenticado) devuelve `{"max_members": M, "minimum_fee": F}` para el usuario logueado.
- El tier del entrenador (rol "entrenador") se resuelve con el mismo criterio que la suscripción vigente; si no hay tier resuelto, se devuelve el default.
- Tier que no está en el hashmap → default (`10` / `20000`).
- Sin params: identidad 100% del access token; no se valida ningún equipo.

**Non-Goals:**
- **No** exponer el mapa ni el criterio de resolución al frontend (solo los dos valores).
- **No** validar propiedad de ningún equipo (`team_id` no existe en el contrato).
- **No** modificar tiers, suscripciones, ni el CRUD de equipos.
- **No** cambios de schema.

## Decisions

### D1. Repuesta plana, dos campos

`{"max_members": 10, "minimum_fee": 20000}` — campos ingleses según pidió el frontend. El struct del hashmap y el de respuesta son el mismo DTO (`TeamConfiguration` con tags JSON), así no hay mapeo.

### D2. Hashmap en `cmd/api/domains/teamconfiguration`

```go
var teamTierConfigurations = map[string]TeamConfiguration{
    "base":    {MaxMembers: 10, MinimumFee: 20000},
    "medium":  {MaxMembers: 25, MinimumFee: 20000},
    "premium": {MaxMembers: 50, MinimumFee: 20000},
}
func ForTier(tierName string) TeamConfiguration { ... default 10/20000 si falta }
```

Tier desconocido (o usuario sin tier) → default `{10, 20000}` (mismo default que pidió el usuario). Los valores por tier son placeholder acordados (escalonada: fee mínimo 20000 fijo, más integrantes por tier) y se ajustan en este único archivo.

### D3. Resolución del tier del entrenador

Mismo criterio que `GetCurrentSubscription` (`tier_subscription_service.go`), fijo al rol "entrenador":
1. `roleDao.FindByName("entrenador")` → si no existe el rol, default (no hay de dónde sacar tier).
2. `tierSubDao.FindActiveByUserRole(user_id, role.ID)` → si hay sub vigente, `sub.TierID`.
3. Si no hay sub, `userRoleDao.FindByUserAndRole(user_id, role.ID)` → `ur.TierID`.
4. `tierDao.FindByID(tierID)` → `tier.Name` → `ForTier(name)`.
Cualquier eslabón que no resuelva (rol/ur/tier nil) → default.

### D4. Identidad del access token, sin params

El controller lee `utils.GetAuthUserID(c)` (seteado por `AuthMiddleware`); sin identidad → `401`. No hay `user_id` ni `team_id` en el contrato: el endpoint es self-only por construcción. Como no valida un equipo, el service no depende de `TeamDao`, y el único error no-éxito es interno → `500`.

### D5. Ruta y swagger

Ruta nueva `GET /api/v1/team-configuration` (no cuelga de `/teams/:id` porque no es un subrecurso del equipo). Godoc swagger sin params (solo el bearer + response). Swagger regenerado y verificado.

## Risks / Trade-offs

- **Config en código y no en DB**: cualquier cambio de topes por tier requiere deploy. Es lo pedido explícitamente ("file golang donde haya un hashmap"); si más adelante se quiere administrar por plataforma, se migra a `platform_settings`/tabla propia.
- **Valores placeholder**: base/medium/premium con default {10, 20000}. Si el negocio define otros números, se cambian solo en `teamconfiguration`.
- **Sin `team_id`**: la config es por entrenador, no por equipo. Si algún día la config dependiera del equipo (p.ej. un equipo "de pago para grandes"), se agrega el param en un cambio futuro.