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

func TestTierPermissionDao_ListPermissionNamesByTier_JoinedNamesOrdered(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_lp_join")
	tier := testTier(db, "tier_for_lp_join", role.ID)
	permLow := testPermission(db, "perm_lp_a")
	permHigh := testPermission(db, "perm_lp_b")
	otherTier := testTier(db, "other_tier_for_lp", role.ID)
	permOther := testPermission(db, "perm_lp_c")

	tpHigh := &dbs.TierPermission{TierID: tier.ID, PermissionID: permHigh.ID, AsignationDate: time.Now()}
	tpLow := &dbs.TierPermission{TierID: tier.ID, PermissionID: permLow.ID, AsignationDate: time.Now()}
	tpOther := &dbs.TierPermission{TierID: otherTier.ID, PermissionID: permOther.ID, AsignationDate: time.Now()}
	require.NoError(t, dao.Create(nil, tpHigh))
	require.NoError(t, dao.Create(nil, tpLow))
	require.NoError(t, dao.Create(nil, tpOther))

	found, err := dao.ListPermissionNamesByTier(nil, tier.ID)

	require.NoError(t, err)
	require.Len(t, found, 2)
	assert.Equal(t, permLow.ID, found[0].PermissionID)
	assert.Equal(t, "perm_lp_a", found[0].PermissionName)
	assert.Equal(t, permHigh.ID, found[1].PermissionID)
	assert.Equal(t, "perm_lp_b", found[1].PermissionName)
}

func TestTierPermissionDao_ListPermissionNamesByTier_ExcludesDeletedRowsAndPermissions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_lp_deleted")
	tier := testTier(db, "tier_for_lp_deleted", role.ID)
	permActive := testPermission(db, "perm_lp_active")
	permDeleted := testPermission(db, "perm_lp_perm_deleted")

	active := &dbs.TierPermission{TierID: tier.ID, PermissionID: permActive.ID, AsignationDate: time.Now()}
	deletedTp := &dbs.TierPermission{TierID: tier.ID, PermissionID: permDeleted.ID, AsignationDate: time.Now()}
	stale := &dbs.TierPermission{TierID: tier.ID, PermissionID: permDeleted.ID, AsignationDate: time.Now()}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deletedTp))
	require.NoError(t, dao.Create(nil, stale))
	require.NoError(t, dao.SoftDelete(nil, deletedTp.ID))
	require.NoError(t, db.Delete(permDeleted).Error)

	found, err := dao.ListPermissionNamesByTier(nil, tier.ID)

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, permActive.ID, found[0].PermissionID)
	assert.Equal(t, "perm_lp_active", found[0].PermissionName)
}

func TestTierPermissionDao_ListPermissionNamesByTier_Empty(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)

	found, err := dao.ListPermissionNamesByTier(nil, 999999)

	require.NoError(t, err)
	assert.Empty(t, found)
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

func TestTierPermissionDao_ListPermissionNamesByTiers_OrderAndMissingPermissions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierPermissionDao(db)
	role := testRole(db, "role_for_tp_batch")

	tier1 := testTier(db, "tier_tp_batch_1", role.ID)
	tier2 := testTier(db, "tier_tp_batch_2", role.ID)
	permA := testPermission(db, "perm_tp_batch_a")
	permB := testPermission(db, "perm_tp_batch_b")
	permMissing := testPermission(db, "perm_tp_batch_missing")

	// Se insertan en orden invertido a propósito: la salida debe ordenar por p.id.
	require.NoError(t, dao.Create(nil, &dbs.TierPermission{TierID: tier1.ID, PermissionID: permB.ID, AsignationDate: time.Now()}))
	require.NoError(t, dao.Create(nil, &dbs.TierPermission{TierID: tier1.ID, PermissionID: permA.ID, AsignationDate: time.Now()}))
	require.NoError(t, dao.Create(nil, &dbs.TierPermission{TierID: tier2.ID, PermissionID: permMissing.ID, AsignationDate: time.Now()}))

	require.NoError(t, db.Model(&dbs.Permission{}).Where("id = ?", permMissing.ID).Update("deleted_at", time.Now()).Error)

	rows, err := dao.ListPermissionNamesByTiers(nil, []int64{tier1.ID, tier2.ID})

	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, tier1.ID, rows[0].TierID)
	assert.Equal(t, permA.ID, rows[0].PermissionID)
	require.NotNil(t, rows[0].PermissionName)
	assert.Equal(t, permA.Name, *rows[0].PermissionName)
	assert.Equal(t, permB.ID, rows[1].PermissionID)
	require.NotNil(t, rows[1].PermissionName)
	assert.Equal(t, permB.Name, *rows[1].PermissionName)
	assert.Equal(t, tier2.ID, rows[2].TierID)
	assert.Nil(t, rows[2].PermissionName, "el permiso borrado se conserva para reportar missingData")

	empty, err := dao.ListPermissionNamesByTiers(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}
