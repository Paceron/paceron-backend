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

func TestNewTierPermissionDao(t *testing.T) {
	dao := NewTierPermissionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTierPermissionDao_ImplementsInterface(t *testing.T) {
	dao := NewTierPermissionDao(&gorm.DB{})
	var iface TierPermissionDaoInterface = dao
	_ = iface
}

func TestTierPermissionDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTierPermissionDao(&gorm.DB{})
	})
}

func testTier(db *gorm.DB, name string, roleID int64) *dbs.Tier {
	tier := &dbs.Tier{Name: name, RoleID: roleID, RoleName: "role"}
	db.Create(tier)
	return tier
}

func testPermission(db *gorm.DB, name string) *dbs.Permission {
	permission := &dbs.Permission{Name: name}
	db.Create(permission)
	return permission
}

func TestTierPermissionDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_tp_create")
	tier := testTier(db, "tier_for_tp_create", role.ID)
	permission := testPermission(db, "perm_for_tp_create")

	tp := &dbs.TierPermission{TierID: tier.ID, PermissionID: permission.ID, AsignationDate: time.Now()}
	err := dao.Create(nil, tp)

	require.NoError(t, err)
	assert.NotZero(t, tp.ID)
}

func TestTierPermissionDao_FindByTierAndPermission_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_tp_find")
	tier := testTier(db, "tier_for_tp_find", role.ID)
	permission := testPermission(db, "perm_for_tp_find")

	tp := &dbs.TierPermission{TierID: tier.ID, PermissionID: permission.ID, AsignationDate: time.Now()}
	require.NoError(t, dao.Create(nil, tp))

	found, err := dao.FindByTierAndPermission(nil, tier.ID, permission.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tp.ID, found.ID)
}

func TestTierPermissionDao_FindByTierAndPermission_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)

	found, err := dao.FindByTierAndPermission(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierPermissionDao_FindByTierID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_tp_bytier")
	tier := testTier(db, "tier_for_tp_bytier", role.ID)
	permActive := testPermission(db, "perm_active_for_tp")
	permDeleted := testPermission(db, "perm_deleted_for_tp")

	active := &dbs.TierPermission{TierID: tier.ID, PermissionID: permActive.ID, AsignationDate: time.Now()}
	deleted := &dbs.TierPermission{TierID: tier.ID, PermissionID: permDeleted.ID, AsignationDate: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	found, err := dao.FindByTierID(nil, tier.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestTierPermissionDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_tp_softdelete")
	tier := testTier(db, "tier_for_tp_softdelete", role.ID)
	permission := testPermission(db, "perm_for_tp_softdelete")

	tp := &dbs.TierPermission{TierID: tier.ID, PermissionID: permission.ID, AsignationDate: time.Now()}
	require.NoError(t, dao.Create(nil, tp))

	err := dao.SoftDelete(nil, tp.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByTierAndPermission(nil, tier.ID, permission.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
