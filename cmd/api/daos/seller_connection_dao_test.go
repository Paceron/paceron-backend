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
		ClientID:       "app-a",
		MPUserID:       "1234567890",
		AccessToken:    "encrypted-access-token",
		RefreshToken:   "encrypted-refresh-token",
		Status:         string(constants.SellerConnectionStatusAuthorized),
		TokenExpiresAt: nil,
	}

	created, err := dao.Upsert(nil, conn)
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	found, err := dao.FindByUserAndClient(nil, 999001, "app-a")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "app-a", found.ClientID)
	assert.Equal(t, "1234567890", found.MPUserID)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), found.Status)
	assert.Equal(t, "encrypted-access-token", found.AccessToken)
}

func TestSellerConnectionDao_FindByUserAndClient_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	found, err := dao.FindByUserAndClient(nil, 999999, "app-a")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSellerConnectionDao_MultiApp_SameUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	// Misma cuenta, dos apps distintas: cada conexión es independiente y el
	// upsert de una no pisa la de la otra (clave (user_id, client_id)).
	connA := &dbs.SellerConnection{
		UserID:       777111,
		ClientID:     "app-a",
		MPUserID:     "111",
		AccessToken:  "access-a",
		RefreshToken: "refresh-a",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	connB := &dbs.SellerConnection{
		UserID:       777111,
		ClientID:     "app-b",
		MPUserID:     "222",
		AccessToken:  "access-b",
		RefreshToken: "refresh-b",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	_, err := dao.Upsert(nil, connA)
	require.NoError(t, err)
	_, err = dao.Upsert(nil, connB)
	require.NoError(t, err)

	foundA, err := dao.FindByUserAndClient(nil, 777111, "app-a")
	require.NoError(t, err)
	require.NotNil(t, foundA)
	assert.Equal(t, "access-a", foundA.AccessToken)

	foundB, err := dao.FindByUserAndClient(nil, 777111, "app-b")
	require.NoError(t, err)
	require.NotNil(t, foundB)
	assert.Equal(t, "access-b", foundB.AccessToken)

	// Re-connect de app-a actualiza solo su fila, no la de app-b.
	updatedA := &dbs.SellerConnection{
		UserID:       777111,
		ClientID:     "app-a",
		MPUserID:     "333",
		AccessToken:  "access-a-2",
		RefreshToken: "refresh-a-2",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	result, err := dao.Upsert(nil, updatedA)
	require.NoError(t, err)
	assert.Equal(t, "access-a-2", result.AccessToken)

	foundBAfter, err := dao.FindByUserAndClient(nil, 777111, "app-b")
	require.NoError(t, err)
	require.NotNil(t, foundBAfter)
	assert.Equal(t, "access-b", foundBAfter.AccessToken)
}

func TestSellerConnectionDao_Upsert_UpdatesExisting(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999002,
		ClientID:     "app-a",
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
		ClientID:     "app-a",
		MPUserID:     "222",
		AccessToken:  "access-new",
		RefreshToken: "refresh-new",
		PublicKey:    "TEST-pk-new",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	result, err := dao.Upsert(nil, updated)
	require.NoError(t, err)
	assert.Equal(t, "222", result.MPUserID)
	assert.Equal(t, "access-new", result.AccessToken)
	assert.Equal(t, "TEST-pk-new", result.PublicKey)

	found, err := dao.FindByUserAndClient(nil, 999002, "app-a")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "222", found.MPUserID)
	assert.Equal(t, "refresh-new", found.RefreshToken)
	assert.Equal(t, "TEST-pk-new", found.PublicKey)
}

func TestSellerConnectionDao_FindAuthorizedByUserAndClient(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999003,
		ClientID:     "app-a",
		MPUserID:     "333",
		AccessToken:  "access",
		RefreshToken: "refresh",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	found, err := dao.FindAuthorizedByUserAndClient(nil, 999003, "app-a")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), found.Status)
}

func TestSellerConnectionDao_FindAuthorizedByUserAndClient_NotAuthorized(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:      999004,
		ClientID:    "app-a",
		MPUserID:    "444",
		AccessToken: "access",
		Status:      string(constants.SellerConnectionStatusDeauthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	found, err := dao.FindAuthorizedByUserAndClient(nil, 999004, "app-a")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSellerConnectionDao_SetStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999005,
		ClientID:     "app-a",
		MPUserID:     "555",
		AccessToken:  "access",
		RefreshToken: "refresh",
		Status:       string(constants.SellerConnectionStatusAuthorized),
	}
	require.NoError(t, func() error {
		_, err := dao.Upsert(nil, conn)
		return err
	}())

	err := dao.SetStatus(nil, 999005, "app-a", string(constants.SellerConnectionStatusDeauthorized))
	require.NoError(t, err)

	found, err := dao.FindByUserAndClient(nil, 999005, "app-a")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), found.Status)
}

func TestSellerConnectionDao_SetStatusByMPUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	conn := &dbs.SellerConnection{
		UserID:       999006,
		ClientID:     "app-a",
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

	found, err := dao.FindByUserAndClient(nil, 999006, "app-a")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), found.Status)
}

func TestSellerConnectionDao_SetStatus_NoRows(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSellerConnectionDao(db)

	err := dao.SetStatus(nil, 999999, "app-a", string(constants.SellerConnectionStatusDeauthorized))
	require.NoError(t, err)
}
