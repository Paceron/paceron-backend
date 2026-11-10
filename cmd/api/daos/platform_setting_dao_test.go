package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestPlatformSettingDao_GetMissingDevuelveDefault(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPlatformSettingDao(db)

	value, setting, err := dao.Get(nil, "test_missing_key", 1.25)

	require.NoError(t, err)
	assert.Equal(t, 1.25, value)
	assert.Nil(t, setting)
}

func TestPlatformSettingDao_SetCreaYActualiza(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPlatformSettingDao(db)

	created, err := dao.Set(nil, "test_upsert_key", 2.5, 10)
	require.NoError(t, err)
	assert.Equal(t, "test_upsert_key", created.Key)
	assert.Equal(t, "2.5", created.Value)
	require.NotNil(t, created.UpdatedBy)
	assert.Equal(t, int64(10), *created.UpdatedBy)

	value, found, err := dao.Get(nil, "test_upsert_key", 0)
	require.NoError(t, err)
	assert.Equal(t, 2.5, value)
	require.NotNil(t, found)
	assert.Equal(t, created.Key, found.Key)

	updated, err := dao.Set(nil, "test_upsert_key", 7.5, 11)
	require.NoError(t, err)
	assert.Equal(t, "7.5", updated.Value)
	require.NotNil(t, updated.UpdatedBy)
	assert.Equal(t, int64(11), *updated.UpdatedBy)
}

func TestPlatformSettingDao_GetValorNoNumericoFalla(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPlatformSettingDao(db)

	require.NoError(t, db.Create(&dbs.PlatformSetting{Key: "test_non_numeric", Value: `"no-es-numero"`}).Error)

	_, _, err := dao.Get(nil, "test_non_numeric", 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing platform setting value")
}

func TestPlatformSettingDao_ErroresDeDB(t *testing.T) {
	db := testutils.SetupTestDB(t)

	t.Run("get", func(t *testing.T) {
		failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
		_, _, err := NewPlatformSettingDao(failing).Get(nil, "test_fail_get", 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error finding platform setting")
	})

	t.Run("set al leer", func(t *testing.T) {
		failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
		_, err := NewPlatformSettingDao(failing).Set(nil, "test_fail_set_select", 1, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error finding platform setting")
	})

	t.Run("set al crear", func(t *testing.T) {
		failing := testutils.FailingDB(t, db, func(op string) bool { return op == "insert" })
		_, err := NewPlatformSettingDao(failing).Set(nil, "test_fail_set_insert", 1, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error creating platform setting")
	})

	t.Run("set al actualizar", func(t *testing.T) {
		require.NoError(t, db.Create(&dbs.PlatformSetting{Key: "test_fail_set_update", Value: "1"}).Error)
		failing := testutils.FailingDB(t, db, func(op string) bool { return op == "update" })
		_, err := NewPlatformSettingDao(failing).Set(nil, "test_fail_set_update", 2, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error updating platform setting")
	})
}
