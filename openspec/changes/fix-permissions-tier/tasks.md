## 1. Servicio de permisos

- [x] 1.1 Agregar `tierSubDao daos.TierSubscriptionDaoInterface` como campo de `permissionsQueryService` y como parámetro de `NewPermissionsQueryService` en `cmd/api/services/permissions_query_service.go`.
- [x] 1.2 En `GetUserPermissions`, por cada rol resolver el tier: consultar `tierSubDao.FindActiveByUserRole(ctx, userID, ur.RoleID)`; si hay sub vigente usar `sub.TierID`, si no fallback a `ur.TierID`; el resto del flujo (perms, missingData, respuesta) queda igual. Verificar con `go build ./...`.

## 2. Wiring de DI

- [x] 2.1 En `cmd/api/app/app.go`, mover la creación de `tierSubscriptionDao := daos.NewTierSubscriptionDao(db)` (hoy en la línea 265) a antes del bloque "Permissions Query flow" y pasar el DAO a `services.NewPermissionsQueryService(...)`.
- [x] 2.2 Ajustar la línea 266 para reusar el mismo `tierSubscriptionDao` (sin crear una segunda instancia). Verificar con `go build ./...`.

## 3. Tests del servicio de permisos

- [x] 3.1 Crear `mockTierSubscriptionDaoForQuery` (implementando `FindActiveByUserRole` y los métodos restantes de la interfaz) en `cmd/api/services/permissions_query_service_test.go` y actualizar las 12 llamadas a `NewPermissionsQueryService` con el nuevo argumento.
- [x] 3.2 Agregar caso: sub vigente `active` con tier base mientras `user_roles.tier_id` apunta a tier pago → responde `tier = base` (regresión del bug reportado).
- [x] 3.3 Agregar caso: sub `first_payment_pending` hacia tier pago → responde el tier pago de la sub.
- [x] 3.4 Agregar caso: sin sub vigente → fallback a `user_roles.tier_id` (comportamiento previo).
- [x] 3.5 Agregar caso: tier de la sub vigente no configurado (`FindByID` devuelve nil) → error "datos faltantes" con el `tier_id` indicado.

## 4. Verificación

- [x] 4.1 Correr `go test ./...` en verde.
- [x] 4.2 Con los datos reportados (user_id con ledger vigente divergente de `user_roles.tier_id`), verificar manualmente que `GET /api/v1/auth/permissions?user_id=X` responde el tier de la sub vigente.
- [x] 4.3 Actualizar `docs/STATE_MACHINES.md` o el README si corresponde para reflejar que el endpoint de permisos resuelve el tier por sub vigente (solo si aplica; el STATE_MACHINES ya documenta el criterio general de resolución).