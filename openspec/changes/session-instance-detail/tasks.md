# Tasks — session-instance-detail

> Rama: `feature/session-instance-detail` (creada). Ejecución inline (change chico).
> Postgres real: `docker start paceron-test-db` (:5433) con `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
> Gate de coverage: 85% (verificar con `go clean -cache` + `make coverage-with-db` + analyzer go-test-coverage; no tocar `.testcoverage.yml`).
> Stagear solo rutas explícitas (nunca `git add -A`, nada de `.superpowers/`). No push/merge (lo hace el usuario). Módulo `simple-arq-golang`.

- [x] 1. DAO de acceso
  - [ ] 1.1 `SessionInstanceDaoInterface` agrega `HasInstanceAccess(ctx, instanceID, callerID) (bool, error)` (o 2 métodos: `HasDayMembershipAccess` + `HasFeedbackAccess` según D2)
  - [ ] 1.2 Implementación con las existence-queries de D2 (membresía activa / owner de equipo del día; feedback activo: atleta, reportante u owner)
  - [ ] 1.3 Tests DAO contra Postgres real: matriz completa del spec (día propio, feedback huérfana propia/ajena/owner, ex-miembro, feedback soft-deleted, equipo ajeno)
  - [ ] 1.4 `go build ./...` + `go test ./cmd/api/daos` verde

- [x] 2. Service + controller
  - [ ] 2.1 `SessionInstanceDetail(ctx, id, callerID)` en `CalendarServiceInterface`: FindByID → 404, acceso → 403, build shape completo (D1/D3)
  - [ ] 2.2 Handler en calendar controller + sentinels/mapeo de errores (404/403/422/500 vía `mapCalendarError` si aplica)
  - [ ] 2.3 Ruta `r.GET("/api/v1/session-instances/:id", ...)` en `url_mappings.go`
  - [ ] 2.4 Tests service (matriz 404/403/200 + shape completo, Postgres real)
  - [ ] 2.5 Tests controller (mock: parsing, 200/401/403/404)

- [x] 3. Swagger + docs
  - [ ] 3.1 Anotaciones Swagger del handler + `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal`
  - [ ] 3.2 `docs/CATALOGO_Y_CALENDARIO.md` (sección del endpoint con shape y regla dual)
  - [ ] 3.3 `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (nota aditiva: Gap 14 cerrado)

- [x] 4. Verificación final
  - [ ] 4.1 `openspec validate session-instance-detail --strict` → valid
  - [ ] 4.2 gofmt (solo archivos tocados) + `go build ./...` + `go vet ./...`
  - [ ] 4.3 Suite completa `go test ./...` con Postgres real → 0 FAIL
  - [ ] 4.4 `go clean -cache` + `make coverage-with-db` → analyzer ≥ 85%, gate PASS, `.testcoverage.yml` intacto
  - [ ] 4.5 Commits convencionales con rutas explícitas (el tilde final de tasks puede ir en el último commit)
