package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// El helper debe fallar por TIPO de operación y delegar el resto al pool real.
func TestFailingDB_FailsByOperationType(t *testing.T) {
	db := SetupTestDB(t)
	seed := &dbs.SessionInstance{Name: "seed vivo"}
	require.NoError(t, db.Create(seed).Error, "el pool original no debe verse afectado")

	failingFind := FailingDB(t, db, func(op string) bool { return op == "select" })

	var rows []dbs.SessionInstance
	err := failingFind.Where("id > ?", 0).Find(&rows).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FailingDB")

	// El mismo modelo por el pool original sigue funcionando.
	var okRows []dbs.SessionInstance
	require.NoError(t, db.Where("id > ?", 0).Find(&okRows).Error)

	failingInsert := FailingDB(t, db, func(op string) bool { return op == "insert" })
	require.Error(t, failingInsert.Create(&dbs.SessionInstance{Name: "no entra"}).Error)
	require.NoError(t, db.Create(&dbs.SessionInstance{Name: "entra por el original"}).Error)

	failingUpdate := FailingDB(t, db, func(op string) bool { return op == "update" })
	require.Error(t, failingUpdate.Model(&dbs.SessionInstance{}).Where("id = ?", seed.ID).Update("name", "x").Error)
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Where("id = ?", seed.ID).Update("name", "y").Error)

	failingDelete := FailingDB(t, db, func(op string) bool { return op == "delete" })
	require.Error(t, failingDelete.Delete(seed).Error)
}

// El fallback de QueryRowContext no rompe al pool original y el handle original
// sigue usable después de construir FailingDB (Statement clonado, no mutado).
func TestFailingDB_OriginalHandleUnaffected(t *testing.T) {
	db := SetupTestDB(t)
	inst := &dbs.SessionInstance{Name: "original"}
	require.NoError(t, db.Create(inst).Error)

	for range [3]struct{}{} {
		FailingDB(t, db, func(op string) bool { return op == "select" })
	}

	var found dbs.SessionInstance
	require.NoError(t, db.Where("id = ?", inst.ID).First(&found).Error)
	assert.Equal(t, inst.Name, found.Name)

	row := FailingDB(t, db, func(op string) bool { return op == "select" }).Raw("SELECT name FROM session_instances WHERE id = ?", inst.ID).Row()
	require.NotNil(t, row)
}
