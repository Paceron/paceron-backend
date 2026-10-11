package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// D6: el INSERT batch con RETURNING rellena los IDs por posición de row —
// el caller mapea fila↔instancia por índice sin depender del orden de DB.
func TestSessionInstanceDao_CreateBatch_RellenaIdsPorPosicion(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	rows := []*dbs.SessionInstance{
		{Name: "batch S 1"},
		{Name: "batch S 2"},
		{Name: "batch S 3"},
	}

	require.NoError(t, dao.CreateBatch(nil, rows))

	for i, row := range rows {
		assert.NotZero(t, row.ID, "row %d sin ID", i)
		found, err := dao.FindByID(nil, row.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, row.Name, found.Name)
	}
	for i := 1; i < len(rows); i++ {
		assert.NotEqual(t, rows[i-1].ID, rows[i].ID)
	}
}

func TestSessionInstanceDao_CreateBatch_LoteVacioNoTocaDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)

	require.NoError(t, dao.CreateBatch(nil, nil))
}
