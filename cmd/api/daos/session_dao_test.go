package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionDao(&gorm.DB{})
	var iface SessionDaoInterface = dao
	_ = iface
}

func TestSessionDao_CreateAndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionDao(db)
	owner := persistUser(db, "session-owner-1@test.com", "61000001")
	s := &dbs.Session{OwnerID: owner.ID, Name: "Sesión base"}

	err := dao.Create(nil, s)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, s.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "Sesión base", found.Name)
}

func TestSessionDao_FindByOwner_ExcludesDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionDao(db)
	owner := persistUser(db, "session-owner-2@test.com", "61000002")
	visible := &dbs.Session{OwnerID: owner.ID, Name: "Visible"}
	require.NoError(t, dao.Create(nil, visible))
	deleted := &dbs.Session{OwnerID: owner.ID, Name: "Borrada"}
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	results, err := dao.FindByOwner(nil, owner.ID)

	require.NoError(t, err)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "Visible")
	assert.NotContains(t, names, "Borrada")
}

func TestSessionDao_SoftDelete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionDao(db)
	owner := persistUser(db, "session-owner-3@test.com", "61000003")
	s := &dbs.Session{OwnerID: owner.ID, Name: "A borrar"}
	require.NoError(t, dao.Create(nil, s))

	err := dao.SoftDelete(nil, s.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, s.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
