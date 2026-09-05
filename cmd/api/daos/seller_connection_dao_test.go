package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewSellerConnectionDao(t *testing.T) {
	dao := NewSellerConnectionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestSellerConnectionDao_ImplementsInterface(t *testing.T) {
	dao := NewSellerConnectionDao(&gorm.DB{})
	var iface SellerConnectionDaoInterface = dao
	_ = iface
}

func TestSellerConnectionDao_Create_FindByUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:         999001,
		MPUserID:       "1234567890",
		AccessToken:    "encrypted-access-token",
		RefreshToken:   "encrypted-refresh-token",
		Status:         string(constants.SellerConnectionStatusAuthorized),
		TokenExpiresAt: nil,
	}

	created, err := dao.Upsert(nil, conn)
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	found, err := dao.FindByUser(nil, 999001)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "1234567890", found.MPUserID)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), found.Status)
	assert.Equal(t, "encrypted-access-token", found.AccessToken)
}

func TestSellerConnectionDao_FindByUser_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	found, err := dao.FindByUser(nil, 999999)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSellerConnectionDao_Upsert_UpdatesExisting(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999002,
		MPUserID:     "111",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	updated := &dbs.SellerConnection{
		UserID:       999002,
		MPUserID:     "222",
		AccessToken:  "access-new",
		RefreshToken: "refresh-new",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	result, err := dao.Upsert(nil, updated)
	require.NoError(t, err)
	assert.Equal(t, "222", result.MPUserID)
	assert.Equal(t, "access-new", result.AccessToken)

	found, err := dao.FindByUser(nil, 999002)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "222", found.MPUserID)
	assert.Equal(t, "refresh-new", found.RefreshToken)
}

func TestSellerConnectionDao_FindAuthorizedByUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999003,
		MPUserID:     "333",
		AccessToken:  "access",
		RefreshToken: "refresh",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	found, err := dao.FindAuthorizedByUser(nil, 999003)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), found.Status)
}

func TestSellerConnectionDao_FindAuthorizedByUser_NotAuthorized(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:      999004,
		MPUserID:    "444",
		AccessToken: "access",
		Status:      string(constants.SellerConnectionStatusDeauthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	found, err := dao.FindAuthorizedByUser(nil, 999004)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSellerConnectionDao_SetStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999005,
		MPUserID:     "555",
		AccessToken:  "access",
		RefreshToken: "refresh",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	err := dao.SetStatus(nil, 999005, string(constants.SellerConnectionStatusDeauthorized))
	require.NoError(t, err)

	found, err := dao.FindByUser(nil, 999005)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), found.Status)
}

func TestSellerConnectionDao_SetStatusByMPUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999006,
		MPUserID:     "123456",
		AccessToken:  "access",
		RefreshToken: "refresh",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	err := dao.SetStatusByMPUser(nil, int64(123456), string(constants.SellerConnectionStatusDeauthorized))
	require.NoError(t, err)

	found, err := dao.FindByUser(nil, 999006)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), found.Status)
}

func TestSellerConnectionDao_SetStatus_NoRows(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	err := dao.SetStatus(nil, 999999, string(constants.SellerConnectionStatusDeauthorized))
	require.NoError(t, err)
}
