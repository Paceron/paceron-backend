package testutils

import (
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
)

var (
	sharedTestDB   *gorm.DB
	sharedTestErr  error
	sharedTestOnce sync.Once
)

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
		sharedTestDB, sharedTestErr = postgresdb.ConfigDB(config.DB{
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
