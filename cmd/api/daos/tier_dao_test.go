package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewTierDao(t *testing.T) {
	dao := NewTierDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTierDao_ImplementsInterface(t *testing.T) {
	dao := NewTierDao(&gorm.DB{})
	var iface TierDaoInterface = dao
	_ = iface
}

func TestTierDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTierDao(&gorm.DB{})
	})
}

func testRole(db *gorm.DB, name string) *dbs.Role {
	role := &dbs.Role{Name: name}
	db.Create(role)
	return role
}

func TestTierDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_create")

	tier := &dbs.Tier{Name: "base_test", RoleID: role.ID, RoleName: role.Name}
	err := dao.Create(nil, tier)

	require.NoError(t, err)
	assert.NotZero(t, tier.ID)
}

func TestTierDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_findid")

	tier := &dbs.Tier{Name: "tier_findid_test", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, tier))

	found, err := dao.FindByID(nil, tier.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "tier_findid_test", found.Name)
}

func TestTierDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierDao_FindByNameAndRole_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_findnamerole")

	tier := &dbs.Tier{Name: "premium_test", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, tier))

	found, err := dao.FindByNameAndRole(nil, "premium_test", role.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tier.ID, found.ID)
}

func TestTierDao_FindByNameAndRole_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)

	found, err := dao.FindByNameAndRole(nil, "no_existe", 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierDao_FindByName_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_findname")

	tier := &dbs.Tier{Name: "tier_byname_test", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, tier))

	found, err := dao.FindByName(nil, "tier_byname_test")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tier.ID, found.ID)
}

func TestTierDao_FindByName_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)

	found, err := dao.FindByName(nil, "no_existe")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierDao_FindLowestByRole_OrderByHierarchy(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_lowest")

	premium := &dbs.Tier{Name: "premium_bylowest", RoleID: role.ID, RoleName: role.Name, Hierarchy: 3}
	medium := &dbs.Tier{Name: "medium_bylowest", RoleID: role.ID, RoleName: role.Name, Hierarchy: 2}
	base := &dbs.Tier{Name: "base_bylowest", RoleID: role.ID, RoleName: role.Name, Hierarchy: 1}
	require.NoError(t, dao.Create(nil, premium))
	require.NoError(t, dao.Create(nil, medium))
	require.NoError(t, dao.Create(nil, base))

	lowest, err := dao.FindLowestByRole(nil, role.ID)

	require.NoError(t, err)
	require.NotNil(t, lowest)
	assert.Equal(t, base.ID, lowest.ID)
	assert.Equal(t, 1, lowest.Hierarchy)
}

func TestTierDao_FindLowestByRole_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)

	found, err := dao.FindLowestByRole(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierDao_FindLowestByRole_SkipsPaidLowerHierarchy(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_lowest_paid")

	paid := &dbs.Tier{Name: "paid_h0", RoleID: role.ID, RoleName: role.Name, Hierarchy: 0, PaymentRequired: true}
	free := &dbs.Tier{Name: "free_h5", RoleID: role.ID, RoleName: role.Name, Hierarchy: 5, PaymentRequired: false}
	require.NoError(t, dao.Create(nil, paid))
	require.NoError(t, dao.Create(nil, free))

	lowest, err := dao.FindLowestByRole(nil, role.ID)

	require.NoError(t, err)
	require.NotNil(t, lowest)
	assert.Equal(t, free.ID, lowest.ID)
	assert.False(t, lowest.PaymentRequired)
}

func TestTierDao_FindLowestByRole_NoFreeBase(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_lowest_only_paid")

	require.NoError(t, dao.Create(nil, &dbs.Tier{Name: "paid_only", RoleID: role.ID, RoleName: role.Name, Hierarchy: 1, PaymentRequired: true}))

	found, err := dao.FindLowestByRole(nil, role.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierDao_FindLowestByRoles_SkipsPaidLowerHierarchy(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	roleA := testRole(db, "role_for_lowest_paid_a")
	roleB := testRole(db, "role_for_lowest_paid_b")

	paidA := &dbs.Tier{Name: "paid_a_h0", RoleID: roleA.ID, RoleName: roleA.Name, Hierarchy: 0, PaymentRequired: true}
	freeA := &dbs.Tier{Name: "free_a_h5", RoleID: roleA.ID, RoleName: roleA.Name, Hierarchy: 5, PaymentRequired: false}
	paidB := &dbs.Tier{Name: "paid_b_h1", RoleID: roleB.ID, RoleName: roleB.Name, Hierarchy: 1, PaymentRequired: true}
	for _, tier := range []*dbs.Tier{paidA, freeA, paidB} {
		require.NoError(t, dao.Create(nil, tier))
	}

	found, err := dao.FindLowestByRoles(nil, []int64{roleA.ID, roleB.ID})

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, freeA.ID, found[0].ID)
	assert.Equal(t, roleA.ID, found[0].RoleID)
}

func TestTierDao_GetAll_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_getall")

	active := &dbs.Tier{Name: "tier_activo_test", RoleID: role.ID, RoleName: role.Name}
	deleted := &dbs.Tier{Name: "tier_eliminado_test", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	all, err := dao.GetAll(nil)

	require.NoError(t, err)
	names := make([]string, 0, len(all))
	for _, tr := range all {
		names = append(names, tr.Name)
	}
	assert.Contains(t, names, "tier_activo_test")
	assert.NotContains(t, names, "tier_eliminado_test")
}

func TestTierDao_Update_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_update")

	tier := &dbs.Tier{Name: "tier_original_test", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, tier))

	tier.Description = "actualizado"
	require.NoError(t, dao.Update(nil, tier))

	found, err := dao.FindByID(nil, tier.ID)
	require.NoError(t, err)
	assert.Equal(t, "actualizado", found.Description)
}

func TestTierDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_softdelete")

	tier := &dbs.Tier{Name: "tier_a_borrar_test", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, tier))

	err := dao.SoftDelete(nil, tier.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, tier.ID)
	assert.Nil(t, found)
}

func TestTierDao_FindByIDs_ExcludesDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	role := testRole(db, "role_for_tier_findids")

	first := &dbs.Tier{Name: "tier_findids_a", RoleID: role.ID, RoleName: role.Name}
	second := &dbs.Tier{Name: "tier_findids_b", RoleID: role.ID, RoleName: role.Name}
	require.NoError(t, dao.Create(nil, first))
	require.NoError(t, dao.Create(nil, second))
	require.NoError(t, dao.SoftDelete(nil, first.ID))

	found, err := dao.FindByIDs(nil, []int64{first.ID, second.ID, 999999})

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, second.ID, found[0].ID)

	empty, err := dao.FindByIDs(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestTierDao_FindLowestByRoles_OnePerRole(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierDao(db)
	roleA := testRole(db, "role_for_lowest_tiers_a")
	roleB := testRole(db, "role_for_lowest_tiers_b")

	baseA := &dbs.Tier{Name: "base_a", RoleID: roleA.ID, RoleName: roleA.Name, Hierarchy: 1}
	premiumA := &dbs.Tier{Name: "premium_a", RoleID: roleA.ID, RoleName: roleA.Name, Hierarchy: 3}
	baseB := &dbs.Tier{Name: "base_b", RoleID: roleB.ID, RoleName: roleB.Name, Hierarchy: 2}
	for _, tier := range []*dbs.Tier{premiumA, baseA, baseB} {
		require.NoError(t, dao.Create(nil, tier))
	}

	found, err := dao.FindLowestByRoles(nil, []int64{roleA.ID, roleB.ID})

	require.NoError(t, err)
	require.Len(t, found, 2)
	byRole := map[int64]dbs.Tier{}
	for _, tier := range found {
		byRole[tier.RoleID] = tier
	}
	assert.Equal(t, baseA.ID, byRole[roleA.ID].ID)
	assert.Equal(t, baseB.ID, byRole[roleB.ID].ID)

	empty, err := dao.FindLowestByRoles(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}
