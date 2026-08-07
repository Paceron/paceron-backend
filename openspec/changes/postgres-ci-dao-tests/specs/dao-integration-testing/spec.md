## ADDED Requirements

### Requirement: Tests de DAO contra Postgres real en CI
El sistema SHALL ejecutar los tests de `cmd/api/daos` contra una instancia real de Postgres en cada corrida de CI, sin depender de mocks o dobles de base de datos.

#### Scenario: CI corre con Postgres disponible
- **WHEN** `ci.yml` ejecuta el job de test
- **THEN** un container `postgres:16-alpine` está disponible en `localhost:5432` con las credenciales de `TEST_DB_*`, y los tests de `daos` se ejecutan contra él (no se skipean)

### Requirement: Tests de DAO opcionales en desarrollo local sin Docker
El sistema SHALL permitir que `go test ./...` corra exitosamente en cualquier máquina sin Docker instalado, skipeando automáticamente los tests que requieren Postgres real.

#### Scenario: Desarrollador sin Docker corre la suite completa
- **WHEN** se ejecuta `go test ./...` sin la variable `TEST_DB_HOST` seteada
- **THEN** los tests de `daos` que llaman `testutils.SetupTestDB` se skipean (no fallan), y el resto de la suite corre normalmente

#### Scenario: Desarrollador con Docker quiere correr los tests de DAO localmente
- **WHEN** el desarrollador corre `make test-db-up` y luego `make test-with-db`
- **THEN** los tests de `daos` corren contra un Postgres local descartable, con el mismo resultado que en CI

### Requirement: Aislamiento entre tests de DAO
El sistema SHALL garantizar que cada test de DAO parta de un estado limpio, sin ver datos insertados por otros tests, sin necesidad de truncar tablas entre ejecuciones.

#### Scenario: Dos tests usan el mismo valor único (ej. mismo email)
- **WHEN** dos tests de DAO distintos insertan una fila con el mismo valor de un campo con restricción única (ej. `email`)
- **THEN** ambos tests pasan sin colisión, porque cada uno corre en su propia transacción no confirmada, revertida automáticamente al terminar

### Requirement: Piso de coverage obligatorio
El sistema SHALL bloquear el merge de un cambio si el coverage total del proyecto cae por debajo del 80%.

#### Scenario: Un PR baja el coverage total por debajo de 80%
- **WHEN** el coverage medido en CI (`go tool cover -func`) es menor a 80% del total no excluido
- **THEN** el check `Check test coverage` (`vladopajic/go-test-coverage`) falla, bloqueando el merge

#### Scenario: Un PR mantiene el coverage en 80% o más
- **WHEN** el coverage medido es 80% o superior
- **THEN** el check pasa, sin bloquear el merge
