.PHONY: test coverage coverage-html test-db-up test-db-down test-db-restart test-with-db coverage-with-db

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
