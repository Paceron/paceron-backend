package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionInstanceDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionInstanceDao(&gorm.DB{})
	var iface SessionInstanceDaoInterface = dao
	_ = iface
}

func TestSessionInstanceDao_Create_AndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	desc := "Fartlek congelado"
	inst := &dbs.SessionInstance{Name: "Fartlek 5K", Description: &desc}

	require.NoError(t, dao.Create(nil, inst))
	require.Greater(t, inst.ID, int64(0))

	found, err := dao.FindByID(nil, inst.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "Fartlek 5K", found.Name)
	require.NotNil(t, found.Description)
	assert.Equal(t, desc, *found.Description)
}

func TestSessionInstanceDao_FindByID_NotFound_ReturnsNil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionInstanceDao_Delete_Physical(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "A borrar"}
	require.NoError(t, dao.Create(nil, inst))

	require.NoError(t, dao.Delete(nil, inst.ID))

	found, err := dao.FindByID(nil, inst.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionInstanceDao_HasFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	with := &dbs.SessionInstance{Name: "Con feedback"}
	require.NoError(t, dao.Create(nil, with))
	without := &dbs.SessionInstance{Name: "Sin feedback"}
	require.NoError(t, dao.Create(nil, without))
	testFeedback(t, db, 1, 1, with.ID, 1, nil, 0)

	has, err := dao.HasFeedback(nil, with.ID)
	require.NoError(t, err)
	assert.True(t, has)

	has, err = dao.HasFeedback(nil, without.ID)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestSessionInstanceDao_HasFeedback_IgnoresSoftDeletedFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "Con feedback borrado"}
	require.NoError(t, dao.Create(nil, inst))
	fb := testFeedback(t, db, 1, 1, inst.ID, 1, nil, 0)

	require.NoError(t, db.Model(&dbs.WorkoutFeedback{}).Where("id = ?", fb.ID).Update("deleted_at", fb.CreatedAt).Error)

	has, err := dao.HasFeedback(nil, inst.ID)
	require.NoError(t, err)
	assert.False(t, has)
}
