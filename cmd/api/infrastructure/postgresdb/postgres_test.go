package postgresdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
)

func TestConnString_IncluyeTodosLosCampos(t *testing.T) {
	dsn := ConnString(config.DB{
		Host: "db.local", Port: "5433", Username: "user", Password: "pass", Name: "paceron",
	})

	assert.Contains(t, dsn, "host=db.local")
	assert.Contains(t, dsn, "port=5433")
	assert.Contains(t, dsn, "user=user")
	assert.Contains(t, dsn, "password=pass")
	assert.Contains(t, dsn, "dbname=paceron")
	assert.Contains(t, dsn, "sslmode=disable")
	assert.Contains(t, dsn, "TimeZone=UTC")
}

func TestConfigDB_ErrorDeConexionDevuelveError(t *testing.T) {
	_, err := ConfigDB(config.DB{
		Host: "127.0.0.1", Port: "1", Username: "postgres", Password: "postgres",
		Name: "paceron_test", ConnMaxLifetime: time.Second,
	})

	require.Error(t, err)
}
