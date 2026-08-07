## 1. Infra

- [x] 1.1 `ci.yml`: `services.postgres` (`postgres:16-alpine`, healthcheck) + env `TEST_DB_*` para el job
- [x] 1.2 `cmd/api/testutils/db.go`: `SetupTestDB(t)` — conexión única por proceso vía `postgresdb.ConfigDB`, transacción aislada por test con rollback en `t.Cleanup`, skip si falta `TEST_DB_HOST`
- [x] 1.3 `Makefile`: `test-db-up`/`test-db-down`/`test-db-restart` (container local :5433) + `test-with-db`/`coverage-with-db`

## 2. Tests de DAO (14 archivos)

- [x] 2.1 `role_dao_test.go` — piloto, validado contra Postgres real antes de replicar al resto
- [x] 2.2 `auth_dao_test.go` (no existía, creado)
- [x] 2.3 `permission_dao_test.go`
- [x] 2.4 `tier_dao_test.go`
- [x] 2.5 `tier_permission_dao_test.go`
- [x] 2.6 `team_dao_test.go` (+ helpers compartidos `persistUser`, `testTeam`)
- [x] 2.7 `team_user_dao_test.go`
- [x] 2.8 `group_dao_test.go` (+ helper `testGroup`)
- [x] 2.9 `group_user_dao_test.go`
- [x] 2.10 `invitation_dao_test.go` (+ helper `testInvitation`)
- [x] 2.11 `password_reset_dao_test.go`
- [x] 2.12 `refresh_token_dao_test.go`
- [x] 2.13 `user_role_dao_test.go`
- [x] 2.14 `user_dao_test.go` (incluye `SearchActive`/`FindByIDs`, agregados en `feature/user-search-endpoint`/`feature/user-batch-lookup`)

## 3. Gate y docs

- [x] 3.1 `.testcoverage.yml`: `threshold.total` de `0` a `80`
- [x] 3.2 `docs/TESTING.md` nuevo
- [x] 3.3 `CLAUDE.md`: sección Testing actualizada (ya no "no hay DB real en CI")
- [x] 3.4 `SETUP.md`: referencia a `docs/TESTING.md`

## 4. Verificación

- [x] 4.1 `make test-db-up` + `make test-with-db` — 161 tests de `daos` en verde contra Postgres real local
- [x] 4.2 `make coverage-with-db` — total del proyecto 78% → 85.1%, `daos` 4.1% → 85.3%
- [x] 4.3 `go test ./...` sin `TEST_DB_HOST` (sin Docker) — los tests de `daos` skipean, resto verde, sin romper el flujo de un dev sin Docker
- [x] 4.4 `go build`/`go vet` verdes
