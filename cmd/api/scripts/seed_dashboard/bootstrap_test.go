package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// El DSN de --db es lo que permite sembrar la base descartable de los tests sin
// tocar el .env.local de la máquina de cada uno.
func TestParseSeedDSN(t *testing.T) {
	db, err := parseSeedDSN("postgres://usuario:clave@localhost:5433/paceron_test")

	require.NoError(t, err)
	assert.Equal(t, "localhost", db.Host)
	assert.Equal(t, "5433", db.Port)
	assert.Equal(t, "usuario", db.Username)
	assert.Equal(t, "clave", db.Password)
	assert.Equal(t, "paceron_test", db.Name)
	assert.Positive(t, db.MaxOpenConnections,
		"sin límites de pool, ConfigDB rechaza la conexión porMaxOpenConnections = 0")
}

func TestParseSeedDSN_PuertoPorDefecto(t *testing.T) {
	db, err := parseSeedDSN("postgresql://postgres:postgres@localhost/paceron")
	require.NoError(t, err)
	assert.Equal(t, "5432", db.Port)
	assert.Equal(t, "paceron", db.Name)
}

func TestParseSeedDSN_Invalido(t *testing.T) {
	for _, raw := range []string{"", "no-es-una-url", "postgres://localhost:5432", "postgres://user@host/"} {
		_, err := parseSeedDSN(raw)
		assert.Error(t, err, "%q debería ser inválido", raw)
	}
}
