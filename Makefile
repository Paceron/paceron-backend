.PHONY: test coverage coverage-html test-db-up test-db-down test-db-restart test-with-db coverage-with-db
.PHONY: local-up local-down local-ps local-logs local-dump local-restore local-reset local-shell
.PHONY: demo-baseline demo-baseline-force demo-restore demo-reset demo-verify demo-status

# =============================================================================
# Entorno de desarrollo 100% local (Postgres + storage S3-compatible).
# Distinto de los targets test-db-* de abajo: aquellos son una base de test
# descartable para correr los tests de daos/, este es el entorno de trabajo con
# los datos reales clonados de Supabase. Ver docs/ENTORNO_LOCAL.md.
#
# El compose lee estas variables para armar el entorno. Con los defaults alcanza,
# no hace falta .env.local.
ENV_FILE ?= .env.local
COMPOSE_ENV := $(if $(wildcard $(ENV_FILE)),--env-file $(ENV_FILE),)

local-up:      ## Levanta Postgres :5432 y storage :9000/:9001, con el bucket listo
	docker compose $(COMPOSE_ENV) up -d

local-down:    ## Baja los contenedores, conservando los volúmenes (los datos quedan)
	docker compose $(COMPOSE_ENV) down

local-reset:   ## Baja los contenedores y BORRA los volúmenes (base y bucket vacíos)
	docker compose $(COMPOSE_ENV) down -v

local-ps:      ## Estado de los contenedores
	docker compose $(COMPOSE_ENV) ps -a

local-logs:    ## Logs de db y storage
	docker compose $(COMPOSE_ENV) logs -f --tail=100

local-shell:   ## psql contra la base local
	docker compose $(COMPOSE_ENV) exec db psql -U $${POSTGRES_USER:-postgres} -d $${POSTGRES_DB:-paceron_local}

local-dump:    ## Clona la base de Supabase a backup/paceron-<timestamp>.dump
	./scripts/dump_db.sh

local-restore: ## Restaura el último dump de backup/ en el Postgres local (pide confirmación)
	./scripts/restore_db.sh

# =============================================================================
# Demo: congelar la base como baseline y volver a ella. Para cuando los datos se
# ensucian (una demo con varias rondas, pruebas manuales). El instructivo corto
# está en INSTRUCTIVO-DEMO.html, el detalle en docs/ENTORNO_LOCAL.md.
# =============================================================================
DEMO_FLAGS ?=

demo-baseline: ## Congela el estado actual de la base como baseline (una vez, antes de empezar)
	./scripts/demo_db.sh baseline $(DEMO_FLAGS)

demo-restore:  ## Vuelve al baseline en caliente, rápido — para usar entre rondas
	./scripts/demo_db.sh restore $(DEMO_FLAGS)

demo-reset:    ## Vuelve al baseline en frío: borra volúmenes (base y bucket) y levanta de cero
	./scripts/demo_db.sh reset $(DEMO_FLAGS)

demo-verify:   ## Compara los conteos de la base contra el baseline (sale distinto de 0 si difieren)
	./scripts/demo_db.sh verify $(DEMO_FLAGS)

demo-status:   ## Estado del baseline, de los contenedores y de cuánto difiere la base
	./scripts/demo_db.sh status $(DEMO_FLAGS)

# Reemplazar el baseline es una decisión explícita, no un default: por eso el
# flag va explícito acá y no en DEMO_FLAGS (que arrastraría a los otros targets).
demo-baseline-force:
	./scripts/demo_db.sh baseline --force

# =============================================================================
# Tests — abajo, la base de test descartable para los tests de daos/
# =============================================================================
# Variables usadas por los tests de daos/ (testutils.SetupTestDB) — mismos defaults
# que el container de test-db-up y que el service de ci.yml. Ver docs/TESTING.md.
# Deliberadamente NO exportadas para los targets `test`/`coverage` normales: sin
# TEST_DB_HOST seteada, esos tests se skipean solos (go test ./... sigue andando
# sin Docker). Los targets *-with-db son los que sí las exportan.
TEST_DB_HOST ?= localhost
TEST_DB_PORT ?= 5433
TEST_DB_USER ?= postgres
TEST_DB_PASSWORD ?= postgres
TEST_DB_NAME ?= paceron_test

test:
	go test ./... -v

coverage:
	mkdir -p ci/test_coverage
	go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs go test -coverpkg=./... -coverprofile=ci/test_coverage/coverage.out -covermode=atomic
	go tool cover -func=ci/test_coverage/coverage.out

coverage-html: coverage
	go tool cover -html=ci/test_coverage/coverage.out -o ci/test_coverage/coverage.html

# Postgres local para correr los tests de daos/ contra una DB real, igual que CI.
# Puerto 5433 (no 5432) para no chocar con un Postgres local ya corriendo.
test-db-up:
	docker run -d --name paceron-test-db \
		-e POSTGRES_USER=$(TEST_DB_USER) \
		-e POSTGRES_PASSWORD=$(TEST_DB_PASSWORD) \
		-e POSTGRES_DB=$(TEST_DB_NAME) \
		-p $(TEST_DB_PORT):5432 \
		postgres:16-alpine
	@echo "esperando que Postgres este listo..."
	@until docker exec paceron-test-db pg_isready -U $(TEST_DB_USER) > /dev/null 2>&1; do sleep 1; done
	@echo "listo, TEST_DB_HOST=$(TEST_DB_HOST) TEST_DB_PORT=$(TEST_DB_PORT)"

test-db-down:
	docker rm -f paceron-test-db

test-db-restart: test-db-down test-db-up

# Igual que test/coverage, pero con TEST_DB_* seteadas — corre también los tests de
# daos/ contra Postgres real. Requiere `make test-db-up` corrido antes.
test-with-db:
	TEST_DB_HOST=$(TEST_DB_HOST) TEST_DB_PORT=$(TEST_DB_PORT) TEST_DB_USER=$(TEST_DB_USER) TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) TEST_DB_NAME=$(TEST_DB_NAME) go test ./... -v

coverage-with-db:
	mkdir -p ci/test_coverage
	TEST_DB_HOST=$(TEST_DB_HOST) TEST_DB_PORT=$(TEST_DB_PORT) TEST_DB_USER=$(TEST_DB_USER) TEST_DB_PASSWORD=$(TEST_DB_PASSWORD) TEST_DB_NAME=$(TEST_DB_NAME) \
		bash -c "go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs go test -coverpkg=./... -coverprofile=ci/test_coverage/coverage.out -covermode=atomic"
	go tool cover -func=ci/test_coverage/coverage.out
