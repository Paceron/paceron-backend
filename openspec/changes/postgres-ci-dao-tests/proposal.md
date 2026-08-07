## Why

Auditoría de coverage (2026-08-07): total del proyecto en 78%, con `daos` en 4.1% — el paquete más grande sin testear de verdad, solo conformidad de interfaz (`NewXDao`, `ImplementsInterface`, `NoPanic`), sin ejercitar ninguna query real, porque CI no tenía Postgres. El usuario decidió: agregar Postgres real en CI para poder testear `daos`, y una vez alcanzado el 80% de coverage, forzarlo como piso obligatorio (CI rompe si baja).

## What Changes

- `ci.yml`: nuevo `services.postgres` (`postgres:16-alpine`, container efímero por job, healthcheck) + env `TEST_DB_*` para todo el job.
- `cmd/api/testutils/db.go` (nuevo): `SetupTestDB(t *testing.T) *gorm.DB` — conecta una vez por proceso de test vía `postgresdb.ConfigDB` (mismo `AutoMigrate` que la app real), y devuelve una transacción aislada por test (rollback automático en `t.Cleanup`). Se skipea solo si `TEST_DB_HOST` no está seteada — `go test ./...` sigue funcionando sin Docker en cualquier máquina.
- Los 14 archivos de `daos/*_test.go` pasan de 3 tests de conformidad de interfaz a tests reales contra Postgres: create/find/update/soft-delete/exclusión de soft-deleted, por cada método público.
- `Makefile`: `test-db-up`/`test-db-down`/`test-db-restart` (container local en :5433, mismo default que CI) + `test-with-db`/`coverage-with-db` (variantes de `test`/`coverage` con las env vars seteadas).
- `.testcoverage.yml`: `threshold.total` pasa de `0` a `80` — **ahora bloquea el merge si el coverage total cae por debajo**.
- `docs/TESTING.md` (nuevo): cómo correr los tests con DB real localmente, cómo funciona `SetupTestDB`, y por qué se descartaron sqlmock/SQLite como alternativa (evaluadas explícitamente, ver `design.md`).
- `CLAUDE.md`/`SETUP.md`: actualizados para reflejar que ya no es cierto que "no hay DB real en CI".

## Capabilities

### New Capabilities
- `dao-integration-testing`: tests de `daos` contra Postgres real, aislados por transacción, opcionales en local (skip sin Docker), obligatorios en CI.

## Impact

- **Nuevo**: `cmd/api/testutils/db.go`, `docs/TESTING.md`, 1 archivo de test nuevo (`auth_dao_test.go` no existía)
- **Modificado**: los 13 `daos/*_test.go` restantes (tests reales agregados, boilerplate de interfaz se mantiene), `ci.yml`, `Makefile`, `.testcoverage.yml`, `CLAUDE.md`, `SETUP.md`
- **Coverage real**: `daos` 4.1% → 85.3%, total del proyecto 78% → 85.1%
- **Sin cambios**: ningún archivo de producción (`daos/*.go` sin tocar, solo sus tests) — cero riesgo de regresión funcional
- **Local dev**: opcional, requiere Docker solo si querés correr los tests de `daos` contra DB real; sin Docker, `go test ./...` sigue funcionando igual que antes (esos tests se skipean)
