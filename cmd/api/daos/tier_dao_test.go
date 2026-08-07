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
