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

func TestNewUserRoleDao(t *testing.T) {
	dao := NewUserRoleDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestUserRoleDao_ImplementsInterface(t *testing.T) {
	dao := NewUserRoleDao(&gorm.DB{})
	var iface UserRoleDaoInterface = dao
	_ = iface
}

func TestUserRoleDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewUserRoleDao(&gorm.DB{})
	})
}

func TestUserRoleDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserRoleDao(db)
	user := persistUser(db, "ur-create@test.com", "90000001")
	role := testRole(db, "role_for_ur_create")

	ur := &dbs.UserRole{UserID: user.ID, RoleID: role.ID, TierID: 1, AssignmentDate: time.Now()}
	err := dao.Create(nil, ur)

	require.NoError(t, err)
	assert.NotZero(t, ur.ID)
}

func TestUserRoleDao_FindByUserAndRole_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserRoleDao(db)
	user := persistUser(db, "ur-find@test.com", "90000002")
	role := testRole(db, "role_for_ur_find")
	ur := &dbs.UserRole{UserID: user.ID, RoleID: role.ID, TierID: 1, AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, ur))

	found, err := dao.FindByUserAndRole(nil, user.ID, role.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, ur.ID, found.ID)
}

func TestUserRoleDao_FindByUserAndRole_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserRoleDao(db)

	found, err := dao.FindByUserAndRole(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUserRoleDao_FindByUserAndRole_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserRoleDao(db)
	user := persistUser(db, "ur-deleted@test.com", "90000003")
	role := testRole(db, "role_for_ur_deleted")
	ur := &dbs.UserRole{UserID: user.ID, RoleID: role.ID, TierID: 1, AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, ur))
	require.NoError(t, dao.SoftDelete(nil, ur.ID))

	found, err := dao.FindByUserAndRole(nil, user.ID, role.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUserRoleDao_FindByUserID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserRoleDao(db)
	user := persistUser(db, "ur-byuser@test.com", "90000004")
	role1 := testRole(db, "role_for_ur_byuser1")
	role2 := testRole(db, "role_for_ur_byuser2")

	active := &dbs.UserRole{UserID: user.ID, RoleID: role1.ID, TierID: 1, AssignmentDate: time.Now()}
	deleted := &dbs.UserRole{UserID: user.ID, RoleID: role2.ID, TierID: 1, AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByUserID(nil, user.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestUserRoleDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserRoleDao(db)
	user := persistUser(db, "ur-softdelete@test.com", "90000005")
	role := testRole(db, "role_for_ur_softdelete")
	ur := &dbs.UserRole{UserID: user.ID, RoleID: role.ID, TierID: 1, AssignmentDate: time.Now()}
	require.NoError(t, dao.Create(nil, ur))

	err := dao.SoftDelete(nil, ur.ID)

	require.NoError(t, err)
	found, _ := dao.FindByUserAndRole(nil, user.ID, role.ID)
	assert.Nil(t, found)
}
