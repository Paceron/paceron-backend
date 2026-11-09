# Diseño — Gap 42

## D1. Tier base interno

`corredor` conserva un tier de `hierarchy=1` y `payment_required=false` en `tiers`. No se expone en el catálogo ni se puede modificar por API. Es el tier que resuelven `AssignRole` (cuando no se envía `tier_id`) y `GET /auth/permissions` (cuando no hay suscripción activa), por lo que no se toca esa cadena.

## D2. Catálogo

- `tierService.GetAll(ctx, roleID)`:
  - Si `roleID` corresponde a `corredor`, devuelve `[]` sin error.
  - Si no viene `roleID`, excluye todos los tiers cuyo `role_name` sea `corredor`.
  - Si `roleID` corresponde a otro rol, conserva el comportamiento actual.
- `tierService.GetByID`/`GetByName` no se usan para pintar el catálogo del corredor; no se cambian para no romper herramientas internas.
- `tierService.Create` rechaza cualquier tier para `corredor` con un error de validación.
- `tierService.Update` y `tierService.Delete` rechazan tiers de `corredor` para proteger el base interno. La purga de tiers pagos se hace con el script manual, no por API.

## D3. Asignación y cambio

- `userRoleService.AssignRole`: si el rol es `corredor` y `req.TierID != 0`, devuelve error de validación (“el rol corredor no admite tiers”). Con `tier_id` ausente sigue resolviendo el base interno.
- `tierSubscriptionService.ChangeTier`: si el rol es `corredor`, devuelve error de validación (“el rol corredor no admite cambios de tier”).
- `GET /auth/permissions` no cambia: sigue leyendo el base interno y sus `tier_permissions`.

## D4. Purga manual

Nuevo `scripts/purge_runner_tiers.sql`, transaccional e idempotente:

1. Resolver `role_id` de `corredor` y el `base_tier_id` de menor jerarquía **con `payment_required = false`**; si no hay tier base gratuito, abortar sin conservar un tier pago como base.
2. Reasignar a `base_tier_id` cualquier `user_roles` de corredor que apunte a otro tier.
3. Borrar `tier_permissions` de los tiers de corredor distintos del base.
4. Borrar `installments` de **todas** las suscripciones de tier del rol corredor (el base es gratuito y no debe tener suscripciones).
5. Borrar **todas** las `user_role_tier_subscriptions` del rol corredor.
6. Borrar físicamente los tiers de corredor distintos del base.
7. Verificar que quede exactamente un tier de corredor, que sea gratuito, y que no queden asignaciones fuera del base ni suscripciones del rol; si falla, revertir la transacción.
8. No tocar el tier base, los roles, ni los pagos ya conciliados. En bases migradas por la app, la FK `payments.installment_id ON DELETE SET NULL` deja los pagos sin vínculo a la cuota purgada (no borra ni cambia su monto/estado); sin esa FK, el campo conserva el valor viejo.

El script no se ejecuta automáticamente ni desde migraciones. Se documenta en `docs/STATE_MACHINES.md` y en el RUNBOOK local.

## D5. Tests

- `tier_service_test`: `GetAll` con rol corredor devuelve `[]`; sin filtro oculta corredor; rol entrenador mantiene sus tiers; `Create`/`Update`/`Delete` rechazan corredor.
- `user_role_service_test`: asignar corredor sin tier usa el base; con tier explícito devuelve error; entrenador con tier pago sigue funcionando.
- `tier_subscription_service_test`: `ChangeTier` de corredor devuelve error; entrenador no cambia.
- `permissions_query_service_test`: un corredor con base interno sigue devolviendo permisos y tier base.
- `scripts/purge_runner_tiers.sql`: revisión manual y prueba en una base de test descartable con fixtures; solo bases descartables de test, nunca local/demo/producción.

## D6. Verificación

`openspec validate --strict`, `gofmt`, `go build ./...`, `go vet ./...`, suite completa con `paceron-test-db:5433`/`TEST_DB_*`, coverage gate 85. Sin push ni merge.
