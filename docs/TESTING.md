# Testing: unit vs. integración con Postgres real

## Qué corre sin Docker

`go test ./...` (o `make test`) funciona siempre, sin depender de nada externo. Los tests de `services`/`controllers`/`delegates` usan mocks (mismo patrón de siempre). Los tests de `daos` que necesitan una base real (los que llaman `testutils.SetupTestDB(t)`) se **skipean automáticamente** si no hay `TEST_DB_HOST` seteada — no rompen `go test ./...` en una máquina sin Docker.

## Correr los tests de `daos` contra Postgres real (local)

```bash
make test-db-up        # levanta un container postgres:16-alpine en el puerto 5433
make test-with-db       # go test ./... -v con TEST_DB_* seteadas
make coverage-with-db   # mismo coverage que corre CI
make test-db-down       # baja y borra el container cuando termines
```

`test-db-up` espera a que Postgres esté listo (`pg_isready`) antes de devolver el control. El container es descartable: no persiste datos entre `test-db-down`/`test-db-up`, y cada test corre en su propia transacción que se revierte al terminar (`t.Cleanup`), así que un test nunca ve datos de otro.

## Cómo funciona `testutils.SetupTestDB`

Vive en `cmd/api/testutils/db.go`. Por test:

1. Lee `TEST_DB_HOST` — si está vacía, `t.Skip(...)`.
2. Conecta una sola vez por proceso de test (`sync.Once`) vía `postgresdb.ConfigDB` — el mismo `AutoMigrate` que usa la app en arranque real.

   **Ojo: `AutoMigrate` NO crea foreign keys en este proyecto.** Los modelos GORM de `cmd/api/domains/dbs/` no declaran asociaciones `constraint:` (se puede verificar con `grep "constraint:" cmd/api/domains/dbs/*.go` → 0 resultados), así que el AutoMigrate genera tablas, columnas e índices, pero ninguna FK. Consecuencia práctica: **una base creada por `AutoMigrate` —CI, `make test-db-up`, la máquina de un compañero— no tiene las FK que sí tiene una base migrada a mano.** Las migraciones que agregan FKs viven como SQL manual versionado en `scripts/` (ej. `scripts/migrate_attendance_source_provenance.sql`) y hay que correrlas a mano en cada base nueva.

   Por eso ningún test de comportamiento puede asumir que una FK está aplicada en la base de test. Lo que sí se automatiza es que el script de migración no se pierda: `cmd/api/daos/attendance_migration_test.go` verifica que el SQL siga declarando la constraint, el nombre correcto, sin `ON DELETE CASCADE`, y con el backfill en `qr`. Si algún día se agrega `constraint:` al modelo, ese test falla a propósito para marcar la migración manual como obsoleta.
3. Por test individual: abre una transacción (`db.Begin()`), la pasa al DAO, y la revierte al final (`t.Cleanup`). Esto da aislamiento total entre tests sin necesidad de truncar tablas ni preocuparse por colisiones de índices únicos entre tests distintos (cada transacción no ve lo que insertó otra, al no haber commit).

Variables (con default para el container de `test-db-up`, mismos valores que usa `ci.yml`):

| Variable | Default |
|---|---|
| `TEST_DB_HOST` | *(sin default — vacía = skip)* |
| `TEST_DB_PORT` | `5432` |
| `TEST_DB_USER` | `postgres` |
| `TEST_DB_PASSWORD` | `postgres` |
| `TEST_DB_NAME` | `paceron_test` |

## CI

`ci.yml` define un `services.postgres` (container efímero por job, no persiste entre corridas) y setea `TEST_DB_*` para todo el job — así `go test ./...` en CI SIEMPRE ejercita los tests de `daos` contra Postgres real, sin steps extra.

## Por qué este approach y no sqlmock/SQLite

Evaluado y descartado explícitamente (ver `openspec/changes/postgres-ci-dao-tests/design.md`):

- **sqlmock**: termina testeando "¿armé el SQL que yo mismo predije?", no si el comportamiento es correcto. Frágil ante cambios de query, no detecta problemas reales de comportamiento en Postgres.
- **SQLite en memoria**: diverge de Postgres en funciones que usamos activamente (`ILIKE` en `user_dao.SearchActive`, por ejemplo, no existe en SQLite) — falsa confianza, el test pasaría aunque el comportamiento real en producción fuera distinto.

Postgres real es el único de los tres que da información verdadera sobre el comportamiento del DAO, y el costo (un container efímero, ~10-20s de arranque por job) es bajo comparado con ese beneficio.

## Coverage gate

`.testcoverage.yml` tiene `threshold.total: 85` — **bloquea el merge si el coverage total cae por debajo de 85%**. No bajar este número para pasar el gate: si el número real cae, es porque se agregó/dejó código con lógica real sin testear — agregar el test correspondiente, no ajustar el umbral.
