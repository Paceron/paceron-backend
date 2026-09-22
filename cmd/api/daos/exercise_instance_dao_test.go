package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestExerciseInstanceDao_ImplementsInterface(t *testing.T) {
	dao := NewExerciseInstanceDao(&gorm.DB{})
	var iface ExerciseInstanceDaoInterface = dao
	_ = iface
}

func TestExerciseInstanceDao_Create_AndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	desc := "Descripción congelada"
	inst := &dbs.ExerciseInstance{
		Name:        "Trote suave congelado",
		Description: &desc,
		Kind:        "jogging",
	}

	require.NoError(t, dao.Create(nil, inst))
	require.Greater(t, inst.ID, int64(0))

	found, err := dao.FindByID(nil, inst.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "Trote suave congelado", found.Name)
	assert.Equal(t, "jogging", found.Kind)
	require.NotNil(t, found.Description)
	assert.Equal(t, desc, *found.Description)
}

func TestExerciseInstanceDao_FindByID_NotFound_ReturnsNil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestExerciseInstanceDao_FindByIDs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	inst1 := &dbs.ExerciseInstance{Name: "Trote", Kind: "jogging"}
	require.NoError(t, dao.Create(nil, inst1))
	inst2 := &dbs.ExerciseInstance{Name: "Serie", Kind: "running"}
	require.NoError(t, dao.Create(nil, inst2))

	rows, err := dao.FindByIDs(nil, []int64{inst2.ID, inst1.ID})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, inst1.ID, rows[0].ID)
	assert.Equal(t, inst2.ID, rows[1].ID)

	rows, err = dao.FindByIDs(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestExerciseInstanceDao_Delete_Physical(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	inst := &dbs.ExerciseInstance{Name: "A borrar", Kind: "running"}
	require.NoError(t, dao.Create(nil, inst))

	require.NoError(t, dao.Delete(nil, inst.ID))

	found, err := dao.FindByID(nil, inst.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestExerciseInstanceDao_HasFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	with := &dbs.ExerciseInstance{Name: "Con feedback", Kind: "running"}
	require.NoError(t, dao.Create(nil, with))
	without := &dbs.ExerciseInstance{Name: "Sin feedback", Kind: "running"}
	require.NoError(t, dao.Create(nil, without))
	testFeedback(t, db, 1, 1, 1, with.ID, nil, 0)

	has, err := dao.HasFeedback(nil, with.ID)
	require.NoError(t, err)
	assert.True(t, has)

	has, err = dao.HasFeedback(nil, without.ID)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestExerciseInstanceDao_HasFeedback_IgnoresSoftDeletedFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewExerciseInstanceDao(db)
	inst := &dbs.ExerciseInstance{Name: "Con feedback borrado", Kind: "running"}
	require.NoError(t, dao.Create(nil, inst))
	fb := testFeedback(t, db, 1, 1, 1, inst.ID, nil, 0)

	require.NoError(t, db.Model(&dbs.WorkoutFeedback{}).Where("id = ?", fb.ID).Update("deleted_at", fb.CreatedAt).Error)

	has, err := dao.HasFeedback(nil, inst.ID)
	require.NoError(t, err)
	assert.False(t, has)
}
