# Tasks — Gap 42

## Global Constraints

- Rama `feature/gap-42-y-performance`; nunca cambiar de rama ni pushear.
- No tocar bases locales/demo; tests solo contra `paceron-test-db:5433` con `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Sin cambios de contrato HTTP fuera de los comportamientos pedidos (catálogo vacío y errores de validación).
- Comentarios de código mínimos. Stagear solo rutas explícitas; nunca `git add -A` ni artefactos `.superpowers/`.
- Conventional Commits. No push/merge.

### Task 1: Guards de catálogo, creación y asignación de tiers del corredor

**Files:**
- Modify: `cmd/api/services/tier_service.go`
- Modify: `cmd/api/services/user_role_service.go`
- Modify: `cmd/api/services/tier_subscription_service.go`
- Test: `cmd/api/services/tier_service_test.go`
- Test: `cmd/api/services/user_role_service_test.go`
- Test: `cmd/api/services/tier_subscription_service_test.go`
- Test: `cmd/api/services/permissions_query_service_test.go`

**Interfaces:**
- Produce: rechazos de validación en `TierService.Create/Update/Delete`, catálogo filtrado en `TierService.GetAll`, guard en `UserRoleService.AssignRole` y `TierSubscriptionService.ChangeTier`.
- Consume: `protectedRoleName` existente en el paquete `services`.

- [x] 1.1 En `tierService.GetAll`, si `roleID` pertenece a `corredor`, devolver `[]` sin error; sin `roleID`, excluir tiers con `RoleName == corredor`.
- [x] 1.2 En `tierService.Create`, rechazar `role.Name == corredor` con `"el rol corredor no admite tiers"`.
- [x] 1.3 En `tierService.Update` y `tierService.Delete`, rechazar tiers cuyo `RoleName` sea `corredor` con el mismo error, antes de escribir.
- [x] 1.4 En `userRoleService.AssignRole`, si `role.Name == corredor && req.TierID != 0`, devolver `"el rol corredor no admite tiers"` antes de crear filas.
- [x] 1.5 En `tierSubscriptionService.ChangeTier`, si `role.Name == corredor`, devolver `"el rol corredor no admite cambios de tier"` antes de consultar suscripciones.
- [x] 1.6 Tests: catálogo corredor `[]`, catálogo global oculta corredor, entrenador intacto, create/update/delete rechazados, assign sin tier usa base, assign con tier rechazado, ChangeTier corredor rechazado, permisos de corredor siguen resolviendo base.
- [x] 1.7 `go build ./...`, `go vet ./...`, suite de `services` con Postgres real y commit `feat(tiers): ocultar y proteger tiers del rol corredor (Gap 42)`.

### Task 2: Purga manual y documentación

**Files:**
- Create: `scripts/purge_runner_tiers.sql`
- Modify: `docs/STATE_MACHINES.md`
- Modify: `cmd/api/scripts/seed_dashboard/RUNBOOK.md`
- Modify: `docs/FRONTEND_IMPACTO_INSTANCIACION.md`

- [x] 2.1 Script SQL transaccional e idempotente: resolver rol `corredor` y su base; reasignar `user_roles.tier_id` al base; borrar `tier_permissions`, `installments`, `user_role_tier_subscriptions` y tiers pagos del corredor; no tocar el base.
- [x] 2.2 Documentar que el script es manual, no una migración automática, y que el tier base interno permanece.
- [x] 2.3 Documentar el contrato frontend: el corredor no tiene catálogo de tiers ni cambio de tier; el entrenador no cambia.
- [x] 2.4 Verificar el script contra una base de test descartable con fixtures; no ejecutarlo en local/demo. Commit `docs(tiers): purga manual y contrato del corredor (Gap 42)`.

### Task 3: Verificación final

- [x] 3.1 `openspec validate gap-42-tiers-corredor --strict` válido.
- [x] 3.2 `gofmt`, `go build ./...`, `go vet ./...`, suite completa `TEST_DB_* go test -count=1 ./...` en verde.
- [x] 3.3 `go clean -cache`, `make coverage-with-db` y analyzer con gate 85.
- [x] 3.4 Tildar checkboxes y commit final si quedó documentación pendiente.
