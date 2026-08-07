package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewTeamUserDao(t *testing.T) {
	dao := NewTeamUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTeamUserDao_ImplementsInterface(t *testing.T) {
	dao := NewTeamUserDao(&gorm.DB{})
	var iface TeamUserDaoInterface = dao
	_ = iface
}

func TestTeamUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTeamUserDao(&gorm.DB{})
	})
}

func TestTeamUserDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-create-owner@test.com", "30000001")
	team := testTeam(db, "equipo_tu_create", owner.ID)

	tu := &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}
	err := dao.Create(nil, tu)

	require.NoError(t, err)
	assert.NotZero(t, tu.ID)
}

func TestTeamUserDao_FindByTeamAndUser_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-find-owner@test.com", "30000002")
	team := testTeam(db, "equipo_tu_find", owner.ID)
	tu := &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, tu))

	found, err := dao.FindByTeamAndUser(nil, team.ID, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tu.ID, found.ID)
}

func TestTeamUserDao_FindByTeamAndUser_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)

	found, err := dao.FindByTeamAndUser(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTeamUserDao_FindByTeamID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-byteam-owner@test.com", "30000003")
	member := persistUser(db, "tu-byteam-member@test.com", "30000004")
	team := testTeam(db, "equipo_tu_byteam", owner.ID)

	active := &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}
	deleted := &dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByTeamID(nil, team.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestTeamUserDao_FindByUserID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-byuser-owner@test.com", "30000005")
	team1 := testTeam(db, "equipo_tu_byuser1", owner.ID)
	team2 := testTeam(db, "equipo_tu_byuser2", owner.ID)

	active := &dbs.TeamUser{TeamID: team1.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}
	deleted := &dbs.TeamUser{TeamID: team2.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByUserID(nil, owner.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestTeamUserDao_CountActiveByTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-count-owner@test.com", "30000006")
	member := persistUser(db, "tu-count-member@test.com", "30000007")
	team := testTeam(db, "equipo_tu_count", owner.ID)

	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}))
	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()}))

	count, err := dao.CountActiveByTeam(nil, team.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestTeamUserDao_CountActiveByTeamExcludingUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-countexcl-owner@test.com", "30000008")
	member := persistUser(db, "tu-countexcl-member@test.com", "30000009")
	team := testTeam(db, "equipo_tu_countexcl", owner.ID)

	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}))
	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()}))

	count, err := dao.CountActiveByTeamExcludingUser(nil, team.ID, owner.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestTeamUserDao_HasOwnerByTeam_True(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-hasowner-owner@test.com", "30000010")
	team := testTeam(db, "equipo_tu_hasowner", owner.ID)
	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}))

	hasOwner, err := dao.HasOwnerByTeam(nil, team.ID)

	require.NoError(t, err)
	assert.True(t, hasOwner)
}

func TestTeamUserDao_HasOwnerByTeam_False(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-noowner-owner@test.com", "30000011")
	team := testTeam(db, "equipo_tu_noowner", owner.ID)

	hasOwner, err := dao.HasOwnerByTeam(nil, team.ID)

	require.NoError(t, err)
	assert.False(t, hasOwner)
}

func TestTeamUserDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-softdelete-owner@test.com", "30000012")
	team := testTeam(db, "equipo_tu_softdelete", owner.ID)
	tu := &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, tu))

	err := dao.SoftDelete(nil, tu.ID)

	require.NoError(t, err)
	found, _ := dao.FindByTeamAndUser(nil, team.ID, owner.ID)
	assert.Nil(t, found)
}

func TestTeamUserDao_SoftDeleteByTeamID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamUserDao(db)
	owner := persistUser(db, "tu-softdeleteteam-owner@test.com", "30000013")
	member := persistUser(db, "tu-softdeleteteam-member@test.com", "30000014")
	team := testTeam(db, "equipo_tu_softdeleteteam", owner.ID)
	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: owner.ID, RoleInTeam: "entrenador", AssignmentDate: time.Now()}))
	require.NoError(t, dao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()}))

	err := dao.SoftDeleteByTeamID(nil, team.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByTeamID(nil, team.ID)
	require.NoError(t, findErr)
	assert.Empty(t, found)
}
