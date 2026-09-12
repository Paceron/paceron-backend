package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestExerciseDao_ImplementsInterface(t *testing.T) {
	dao := NewExerciseDao(&gorm.DB{})
	var iface ExerciseDaoInterface = dao
	_ = iface
}

func TestExerciseDao_CreateAndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-1@test.com", "60000001")
	e := &dbs.Exercise{OwnerID: owner.ID, Name: "Sentadillas", Kind: "running"}

	err := dao.Create(nil, e)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, e.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "Sentadillas", found.Name)
}

func TestExerciseDao_FindByID_NotFoundReturnsNilNil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestExerciseDao_FindByOwner_ExcludesDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-2@test.com", "60000002")
	visible := &dbs.Exercise{OwnerID: owner.ID, Name: "Trote suave", Kind: "jogging"}
	require.NoError(t, dao.Create(nil, visible))
	deleted := &dbs.Exercise{OwnerID: owner.ID, Name: "Viejo", Kind: "walking"}
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	results, err := dao.FindByOwner(nil, owner.ID)

	require.NoError(t, err)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "Trote suave")
	assert.NotContains(t, names, "Viejo")
}

func TestExerciseDao_Update_ClearsOptionalFieldsToNull(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-3@test.com", "60000003")
	minutes := 30
	e := &dbs.Exercise{OwnerID: owner.ID, Name: "Con minutos", Kind: "running", Minutes: &minutes}
	require.NoError(t, dao.Create(nil, e))

	e.Minutes = nil
	e.Name = "Sin minutos"
	err := dao.Update(nil, e)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, e.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "Sin minutos", found.Name)
	assert.Nil(t, found.Minutes)
}

func TestExerciseDao_SoftDelete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseDao(db)
	owner := persistUser(db, "exercise-owner-4@test.com", "60000004")
	e := &dbs.Exercise{OwnerID: owner.ID, Name: "A borrar", Kind: "running"}
	require.NoError(t, dao.Create(nil, e))

	err := dao.SoftDelete(nil, e.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, e.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
