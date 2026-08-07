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

func TestNewGroupUserDao(t *testing.T) {
	dao := NewGroupUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestGroupUserDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupUserDao(&gorm.DB{})
	var iface GroupUserDaoInterface = dao
	_ = iface
}

func TestGroupUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewGroupUserDao(&gorm.DB{})
	})
}

func TestGroupUserDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-create-owner@test.com", "50000001")
	team := testTeam(db, "equipo_gu_create", owner.ID)
	group := testGroup(db, "grupo_gu_create", team.ID)

	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	err := dao.Create(nil, gu)

	require.NoError(t, err)
	assert.NotZero(t, gu.ID)
}

func TestGroupUserDao_FindByGroupAndUser_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-find-owner@test.com", "50000002")
	team := testTeam(db, "equipo_gu_find", owner.ID)
	group := testGroup(db, "grupo_gu_find", team.ID)
	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, gu))

	found, err := dao.FindByGroupAndUser(nil, group.ID, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, gu.ID, found.ID)
}

func TestGroupUserDao_FindByGroupAndUser_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)

	found, err := dao.FindByGroupAndUser(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGroupUserDao_FindByGroupID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-bygroup-owner@test.com", "50000003")
	member := persistUser(db, "gu-bygroup-member@test.com", "50000004")
	team := testTeam(db, "equipo_gu_bygroup", owner.ID)
	group := testGroup(db, "grupo_gu_bygroup", team.ID)

	active := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	deleted := &dbs.GroupUser{GroupID: group.ID, UserID: member.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByGroupID(nil, group.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestGroupUserDao_FindByUserID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-byuser-owner@test.com", "50000005")
	team := testTeam(db, "equipo_gu_byuser", owner.ID)
	group1 := testGroup(db, "grupo_gu_byuser1", team.ID)
	group2 := testGroup(db, "grupo_gu_byuser2", team.ID)

	active := &dbs.GroupUser{GroupID: group1.ID, UserID: owner.ID, DateStart: time.Now()}
	deleted := &dbs.GroupUser{GroupID: group2.ID, UserID: owner.ID, DateStart: time.Now()}
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

func TestGroupUserDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-softdelete-owner@test.com", "50000006")
	team := testTeam(db, "equipo_gu_softdelete", owner.ID)
	group := testGroup(db, "grupo_gu_softdelete", team.ID)
	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, gu))

	err := dao.SoftDelete(nil, gu.ID)

	require.NoError(t, err)
	found, _ := dao.FindByGroupAndUser(nil, group.ID, owner.ID)
	assert.Nil(t, found)
}

func TestGroupUserDao_SoftDeleteByTeamID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupUserDao(db)
	owner := persistUser(db, "gu-softdeleteteam-owner@test.com", "50000007")
	team := testTeam(db, "equipo_gu_softdeleteteam", owner.ID)
	group := testGroup(db, "grupo_gu_softdeleteteam", team.ID)
	gu := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, dao.Create(nil, gu))

	err := dao.SoftDeleteByTeamID(nil, team.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByGroupID(nil, group.ID)
	require.NoError(t, findErr)
	assert.Empty(t, found)
}
