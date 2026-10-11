package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// D6: ejercicio de instancia batch — RETURNING rellena por posición.
func TestExerciseInstanceDao_CreateBatch_RellenaIdsPorPosicion(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	rows := []*dbs.ExerciseInstance{
		{Name: "batch E 1", Kind: "running"},
		{Name: "batch E 2", Kind: "rest"},
	}

	require.NoError(t, dao.CreateBatch(nil, rows))

	for i, row := range rows {
		assert.NotZero(t, row.ID, "row %d sin ID", i)
		found, err := dao.FindByID(nil, row.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, row.Name, found.Name)
	}
	assert.NotEqual(t, rows[0].ID, rows[1].ID)
}

func TestExerciseInstanceDao_CreateBatch_LoteVacioNoTocaDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)

	require.NoError(t, dao.CreateBatch(nil, nil))
}

func TestSessionExerciseInstanceDao_CreateBatch_LoteVacioNoTocaDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionExerciseInstanceDao(db)

	require.NoError(t, dao.CreateBatch(nil, nil))
}

func TestSessionExerciseInstanceDao_CreateBatch_GrabaLinks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := NewSessionInstanceDao(db)
	exerciseDao := NewExerciseInstanceDao(db)
	dao := NewSessionExerciseInstanceDao(db)

	inst := &dbs.SessionInstance{Name: "batch link S"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "batch link E", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	rows := []*dbs.SessionExerciseInstance{
		{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main", RepeatCount: 2, RestMinutes: 30},
	}

	require.NoError(t, dao.CreateBatch(nil, rows))

	links, err := dao.FindBySessionInstance(nil, inst.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, ex.ID, links[0].ExerciseInstanceID)
	assert.Equal(t, "main", links[0].Role)
}
