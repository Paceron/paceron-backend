# Tasks

## Global Constraints

- Rama actual: `feature/gaps-backend` (rama general de gaps).
- No romper el endpoint self-only `GET /api/v1/users/:id/teams/:team_id/subscription`.
- Tests con Postgres real: `paceron-test-db:5433` y `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Coverage gate 85; no bajar `.testcoverage.yml`.
- Comentarios mínimos; stagear solo rutas explícitas; no push/merge.
- No tocar base local/demo (`local-*`, `demo-*`, `fullseed-*`).

## Task 1: DAO batch de cuotas pendientes

- [x] **1.1** Agregar `FindPendingByTeam(ctx, teamID)` a `InstallmentDaoInterface` y su implementación en `cmd/api/daos/installment_dao.go` (`WHERE team_id = ? AND status = 'pending' ORDER BY user_id, installment_number ASC`).
- [x] **1.2** Agregar test DAO contra Postgres real en `cmd/api/daos/installment_dao_test.go`: cuotas de dos corredores del mismo equipo, una cuota de otro equipo, orden por usuario/número.
- [x] **1.3** Actualizar mocks de `InstallmentDaoInterface` que compilean en tests (`mockInstallmentDao`) con el nuevo método.
- [x] **1.4** `go test` focalizado de DAO y build verde.

## Task 2: Service batch y autorización

- [x] **2.1** Agregar DTOs `TeamSubscriptionsResponse` y `TeamMemberSubscriptionInfo` en `cmd/api/domains/teambio/team_subscriptions.go`.
- [x] **2.2** Agregar `GetTeamSubscriptions(ctx, callerID, teamID)` a `TeamSubscriptionServiceInterface` e implementarlo reutilizando `HasPendingDebt` y `membershipOverdue`.
- [x] **2.3** Autorización: owner del equipo o `team_user` activo con rol `entrenador`; el resto devuelve `ErrTeamSubscriptionForbidden`.
- [x] **2.4** Derivar `status` (`active`/`first_payment_pending`/`overdue`), `subscription_status`, `has_debt`, `init_amount` y `paid_installments`; filtrar solo `role_in_team='corredor'`.
- [x] **2.5** Tests service con mocks: owner, entrenador, corredor 403, equipo inexistente, gratis, primer pago pendiente, deuda y agrupación batch.
- [x] **2.6** `go test` focalizado de services y build verde.

## Task 3: Controller, ruta y Swagger

- [x] **3.1** Agregar `GetTeamSubscriptions` a `TeamSubscriptionControllerInterface` y handler que lea `:id`, valide `>0`, llame al service y mapee 401/400/403/404/500.
- [x] **3.2** Agregar ruta `GET /api/v1/teams/:id/subscriptions` en `cmd/api/app/url_mappings.go`.
- [x] **3.3** Agregar tests de controller: 200, 401, 400, 403, 404.
- [x] **3.4** Anotar Swagger y regenerar con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- [x] **3.5** `go build`, `go vet` y suite de controllers verde.

## Task 4: Documentación y verificación final

- [x] **4.1** Documentar el endpoint y el shape en `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (sección nueva) y agregar la ruta al README si corresponde.
- [x] **4.2** Explicar el mapeo de estados para el frontend (`active`→activo, `overdue`→vencido, `first_payment_pending`→en prueba) y que el endpoint es owner/entrenador-only.
- [x] **4.3** `openspec validate gap-41-team-subscriptions-batch --strict`.
- [x] **4.4** gofmt, `go build ./...`, `go vet ./...`, suite completa con Postgres real y coverage.
- [x] **4.5** Tildar tasks y commitear con Conventional Commits.
