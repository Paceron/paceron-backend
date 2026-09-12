## Context

El tier de un usuario se modela por rol: `user_roles` tiene `tier_id` por (user, role) y el pago se registra en `user_role_tier_subscriptions` (ledger por user/rol con status `first_payment_pending`/`active`/`ended`). El rol del dueño de equipo se resuelve por nombre (`roles.name = "entrenador"`, const `teamOwnerRoleName` en `cmd/api/services/team_service.go`). `GET /users/:id/subscriptions/current` ya resuelve el tier vigente para un rol: sub activa si existe, si no `user_roles.tier_id`.

El frontend necesita, al crear/editar un equipo, los topes que su plan le permite. Esa config es **código** (no tabla): un hashmap tier → `{max_members, minimum_fee}`, editable en un solo archivo.

## Goals / Non-Goals

**Goals:**
- `GET /api/v1/team-configuration?user_id=X&team_id=Y` devuelve `{"max_members": M, "minimum_fee": F}`.
- El tier del entrenador (rol "entrenador") se resuelve con el mismo criterio que la suscripción vigente; si no hay tier resuelto, se devuelve el default.
- Tier que no está en el hashmap → default (`10` / `20000`).
- El entrenador debe ser dueño del equipo consultado.

**Non-Goals:**
- **No** exponer el mapa ni el criterio de resolución al frontend (solo los dos valores).
- **No** modificar tiers, suscripciones, ni el CRUD de equipos.
- **No** cambios de schema.
- **No** recalcular/validar nada contra `teams.max_members` ya seteado: es solo consulta de la config permitida.

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

### D4. Validación del equipo

`teamDao.FindByID(team_id)`: nil → `404` "equipo no encontrado". `team.OwnerID != user_id` → `403` "solo el dueño del equipo puede consultar su configuración". Es la puerta que impide leer la config de tier de cualquier usuario logueado usando id ajeno.

### D5. Self-only sobre el path

El endpoint exige auth; además, quien pide debe estar pidiendo **su propio** `user_id` (mismo criterio self-only que `GET /users/:id/subscriptions/current`): `authUserID != user_id` → `403`. El front siempre pasa el id del entrenador logueado.

### D6. Ruta y swagger

Ruta nueva `GET /api/v1/team-configuration` (no cuelga de `/teams/:id` porque no es un subrecurso del equipo). Godoc swagger con `@Param user_id` y `@Param team_id` (query, required). Swagger regenerado y verificado.

## Risks / Trade-offs

- **Config en código y no en DB**: cualquier cambio de topes por tier requiere deploy. Es lo pedido explícitamente ("file golang donde haya un hashmap"); si más adelante se quiere administrar por plataforma, se migra a `platform_settings`/tabla propia.
- **Valores placeholder**: base/medium/premium con deee default {10, 20000}. Si el negocio define otros números, se cambian solo en `teamconfiguration`.