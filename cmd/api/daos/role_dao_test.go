package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewRoleDao(t *testing.T) {
	dao := NewRoleDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestRoleDao_ImplementsInterface(t *testing.T) {
	dao := NewRoleDao(&gorm.DB{})
	var iface RoleDaoInterface = dao
	_ = iface
}

func TestRoleDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewRoleDao(&gorm.DB{})
	})
}

func TestRoleDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	role := &dbs.Role{Name: "entrenador_test", Description: "desc"}
	err := dao.Create(nil, role)

	require.NoError(t, err)
	assert.NotZero(t, role.ID)
}

func TestRoleDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	role := &dbs.Role{Name: "corredor_test"}
	require.NoError(t, dao.Create(nil, role))

	found, err := dao.FindByID(nil, role.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "corredor_test", found.Name)
}

func TestRoleDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRoleDao_FindByID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	role := &dbs.Role{Name: "borrado_test"}
	require.NoError(t, dao.Create(nil, role))
	require.NoError(t, dao.SoftDelete(nil, role.ID))

	found, err := dao.FindByID(nil, role.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRoleDao_FindByName_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	role := &dbs.Role{Name: "nombre_unico_test"}
	require.NoError(t, dao.Create(nil, role))

	found, err := dao.FindByName(nil, "nombre_unico_test")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, role.ID, found.ID)
}

func TestRoleDao_FindByName_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	found, err := dao.FindByName(nil, "no_existe")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRoleDao_GetAll_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	active := &dbs.Role{Name: "activo_test"}
	deleted := &dbs.Role{Name: "eliminado_test"}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	all, err := dao.GetAll(nil)

	require.NoError(t, err)
	names := make([]string, 0, len(all))
	for _, r := range all {
		names = append(names, r.Name)
	}
	assert.Contains(t, names, "activo_test")
	assert.NotContains(t, names, "eliminado_test")
}

func TestRoleDao_Update_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	role := &dbs.Role{Name: "original_test"}
	require.NoError(t, dao.Create(nil, role))

	role.Description = "actualizada"
	require.NoError(t, dao.Update(nil, role))

	found, err := dao.FindByID(nil, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "actualizada", found.Description)
}

func TestRoleDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRoleDao(db)

	role := &dbs.Role{Name: "a_borrar_test"}
	require.NoError(t, dao.Create(nil, role))

	err := dao.SoftDelete(nil, role.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, role.ID)
	assert.Nil(t, found)
}
