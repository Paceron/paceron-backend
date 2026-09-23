package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/testutils"
)

// Ramas de error de DB de group_dao / group_user_dao / team_dao usadas por el
// área de calendario (findPresencialCollisions, banners, calendarios
// agregados), inyectando la falla con FailingDB por tipo de operación.

func TestGroupDao_DBFail_Operaciones(t *testing.T) {
	db := testutils.SetupTestDB(t)

	// FindByID.
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewGroupDao(failing)
	_, err := dao.FindByID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding group")

	// FindByIDAndTeamID (handle nuevo: el consumido de FindByID ya gastó el match).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupDao(failing)
	_, err = dao.FindByIDAndTeamID(nil, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding group")

	// GetAll.
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupDao(failing)
	_, err = dao.GetAll(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding groups")

	// GetByTeamID (handle nuevo).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupDao(failing)
	_, err = dao.GetByTeamID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding groups by team")

	// FindByOwnerID.
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupDao(failing)
	_, err = dao.FindByOwnerID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding groups by owner")

	// FindByIDs.
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupDao(failing)
	_, err = dao.FindByIDs(nil, []int64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding groups by ids")

	// FindByIDs sin ids → vacío sin query.
	dao = NewGroupDao(db)
	groups, err := dao.FindByIDs(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, groups)
}

func TestGroupUserDao_DBFail_Operaciones(t *testing.T) {
	db := testutils.SetupTestDB(t)

	// FindByGroupAndUser.
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewGroupUserDao(failing)
	_, err := dao.FindByGroupAndUser(nil, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding group user")

	// FindByGroupID (handle nuevo).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupUserDao(failing)
	_, err = dao.FindByGroupID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding group users")

	// FindByUserID (banner del home / calendario agregado).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupUserDao(failing)
	_, err = dao.FindByUserID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding user groups")
}

func TestTeamDao_DBFail_Operaciones(t *testing.T) {
	db := testutils.SetupTestDB(t)

	// FindByID.
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewTeamDao(failing)
	_, err := dao.FindByID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding team")

	// GetAll.
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewTeamDao(failing)
	_, err = dao.GetAll(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding teams")

	// GetAllByOwnerID (banner del entrenador / colisiones presenciales; handle nuevo).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewTeamDao(failing)
	_, err = dao.GetAllByOwnerID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding teams by owner")

	// GetAllByMemberID (join con team_users).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewTeamDao(failing)
	_, err = dao.GetAllByMemberID(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding teams by member")

	// FindByIDs (batch de nombres para banners).
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewTeamDao(failing)
	_, err = dao.FindByIDs(nil, []int64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding teams by ids")

	// FindByIDs sin ids → vacío sin query.
	dao = NewTeamDao(db)
	teams, err := dao.FindByIDs(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, teams)

	// UpdateIcon / ClearIcon.
	failing = testutils.FailingDB(t, db, nthFail("update", 1))
	dao = NewTeamDao(failing)
	d1 := time.Now().UTC()
	err = dao.UpdateIcon(nil, 1, "teams/x.png", d1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error updating team icon")

	failing = testutils.FailingDB(t, db, nthFail("update", 1))
	dao = NewTeamDao(failing)
	err = dao.ClearIcon(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error clearing team icon")

	// SearchPublic con select fallida.
	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewTeamDao(failing)
	_, _, err = dao.SearchPublic(nil, TeamSearchFilters{}, 1, 1, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error searching teams")
}
