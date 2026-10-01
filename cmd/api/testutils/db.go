package testutils

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
)

var (
	sharedTestDB   *gorm.DB
	sharedTestErr  error
	sharedTestOnce sync.Once
)

// migrationAdvisoryLockKey serializa el AutoMigrate de ConfigDB entre los
// binarios de test que `go test ./...` corre en paralelo contra la misma DB
// (sin esto, CREATE TYPE concurrentes violan pg_type_typname_nsp_index,
// SQLSTATE 23505). Valor arbitrario fijo; el lock es de sesión y muere con la
// conexión, así que un proceso crasheado nunca lo deja colgado.
const migrationAdvisoryLockKey = 47219

// SetupTestDB conecta a una base de test Postgres real (una sola vez por proceso de
// test, vía postgresdb.ConfigDB — mismo AutoMigrate que usa la app) y devuelve una
// transacción aislada para el test actual, revertida automáticamente al terminar
// (t.Cleanup). Cada test parte de un estado limpio sin necesidad de truncar tablas.
//
// Requiere TEST_DB_HOST (con defaults para el resto de TEST_DB_* pensados para el
// container de CI/desarrollo local, ver docs/TESTING.md). Si TEST_DB_HOST no está
// seteada, el test se skipea — así `go test ./...` sigue funcionando sin Docker.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		t.Skip("TEST_DB_HOST no seteada, saltando test de integración con Postgres (ver docs/TESTING.md)")
	}

	sharedTestOnce.Do(func() {
		sharedTestDB, sharedTestErr = connectSerialized(config.DB{
			Host:               host,
			Port:               getEnvOrDefault("TEST_DB_PORT", "5432"),
			Username:           getEnvOrDefault("TEST_DB_USER", "postgres"),
			Password:           getEnvOrDefault("TEST_DB_PASSWORD", "postgres"),
			Name:               getEnvOrDefault("TEST_DB_NAME", "paceron_test"),
			MaxIdleConnections: 5,
			MaxOpenConnections: 15,
			ConnMaxLifetime:    time.Hour,
		})
	})
	if sharedTestErr != nil {
		t.Fatalf("error conectando a la DB de test: %v", sharedTestErr)
	}

	tx := sharedTestDB.Begin()
	t.Cleanup(func() {
		tx.Rollback()
	})
	return tx
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// connectSerialized toma un advisory lock de sesión con una conexión dedicada,
// corre ConfigDB (su AutoMigrate) mientras lo mantiene y libera todo al final.
// El lock bloquea al resto de binarios hasta que el migrador actual termina.
func connectSerialized(cfg config.DB) (*gorm.DB, error) {
	lockDB, err := gorm.Open(postgres.Open(postgresdb.ConnString(cfg)), &gorm.Config{
		Logger: gl.Default.LogMode(gl.Silent),
	})
	if err != nil {
		return nil, err
	}

	rawDB, err := lockDB.DB()
	if err != nil {
		return nil, err
	}
	defer rawDB.Close()

	ctx := context.Background()
	conn, err := rawDB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); err != nil {
		conn.Close()
		return nil, err
	}
	// El unlock DEBE correr en la misma sesión (conn) que tomó el lock;
	// rawDB.Close() de más arriba cierra la sesión física y suelta el lock
	// aunque esta conn falle a mitad de camino.
	defer conn.Close()
	defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockKey)

	return postgresdb.ConfigDB(cfg)
}
