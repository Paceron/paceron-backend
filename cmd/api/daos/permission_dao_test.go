package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewPermissionDao(t *testing.T) {
	dao := NewPermissionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestPermissionDao_ImplementsInterface(t *testing.T) {
	dao := NewPermissionDao(&gorm.DB{})
	var iface PermissionDaoInterface = dao
	_ = iface
}

func TestPermissionDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewPermissionDao(&gorm.DB{})
	})
}

func TestPermissionDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	permission := &dbs.Permission{Name: "crear_venta_test"}
	err := dao.Create(nil, permission)

	require.NoError(t, err)
	assert.NotZero(t, permission.ID)
}

func TestPermissionDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	permission := &dbs.Permission{Name: "permiso_findid_test"}
	require.NoError(t, dao.Create(nil, permission))

	found, err := dao.FindByID(nil, permission.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "permiso_findid_test", found.Name)
}

func TestPermissionDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPermissionDao_FindByID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	permission := &dbs.Permission{Name: "permiso_borrado_test"}
	require.NoError(t, dao.Create(nil, permission))
	require.NoError(t, dao.SoftDelete(nil, permission.ID))

	found, err := dao.FindByID(nil, permission.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPermissionDao_FindByName_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	permission := &dbs.Permission{Name: "permiso_findname_test"}
	require.NoError(t, dao.Create(nil, permission))

	found, err := dao.FindByName(nil, "permiso_findname_test")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, permission.ID, found.ID)
}

func TestPermissionDao_FindByName_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	found, err := dao.FindByName(nil, "no_existe")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPermissionDao_GetAll_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	active := &dbs.Permission{Name: "permiso_activo_test"}
	deleted := &dbs.Permission{Name: "permiso_eliminado_test"}
	require.NoError(t, dao.Create(nil, active))
	require.NoError(t, dao.Create(nil, deleted))
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	all, err := dao.GetAll(nil)

	require.NoError(t, err)
	names := make([]string, 0, len(all))
	for _, p := range all {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, "permiso_activo_test")
	assert.NotContains(t, names, "permiso_eliminado_test")
}

func TestPermissionDao_Update_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	permission := &dbs.Permission{Name: "permiso_original_test"}
	require.NoError(t, dao.Create(nil, permission))

	permission.Description = "actualizado"
	require.NoError(t, dao.Update(nil, permission))

	found, err := dao.FindByID(nil, permission.ID)
	require.NoError(t, err)
	assert.Equal(t, "actualizado", found.Description)
}

func TestPermissionDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPermissionDao(db)

	permission := &dbs.Permission{Name: "permiso_a_borrar_test"}
	require.NoError(t, dao.Create(nil, permission))

	err := dao.SoftDelete(nil, permission.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, permission.ID)
	assert.Nil(t, found)
}
