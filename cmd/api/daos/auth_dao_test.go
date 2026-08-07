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

func TestNewAuthDao(t *testing.T) {
	dao := NewAuthDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestAuthDao_ImplementsInterface(t *testing.T) {
	dao := NewAuthDao(&gorm.DB{})
	var iface AuthDaoInterface = dao
	_ = iface
}

func TestAuthDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewAuthDao(&gorm.DB{})
	})
}

func testUser(email, dni string) *dbs.User {
	return &dbs.User{
		Name:      "Test",
		Surname:   "User",
		Email:     email,
		DNI:       dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  "hashed",
	}
}

func TestAuthDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	user := testUser("auth-create@test.com", "10000001")
	created, err := dao.Create(nil, user)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotZero(t, created.ID)
}

func TestAuthDao_FindByEmail_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	user := testUser("auth-findemail@test.com", "10000002")
	_, err := dao.Create(nil, user)
	require.NoError(t, err)

	found, err := dao.FindByEmail(nil, "auth-findemail@test.com")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, user.ID, found.ID)
}

func TestAuthDao_FindByEmail_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	found, err := dao.FindByEmail(nil, "no-existe@test.com")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAuthDao_FindByDNI_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	user := testUser("auth-finddni@test.com", "10000003")
	_, err := dao.Create(nil, user)
	require.NoError(t, err)

	found, err := dao.FindByDNI(nil, "10000003")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, user.ID, found.ID)
}

func TestAuthDao_FindByDNI_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	found, err := dao.FindByDNI(nil, "00000000")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestAuthDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	user := testUser("auth-findid@test.com", "10000004")
	_, err := dao.Create(nil, user)
	require.NoError(t, err)

	found, err := dao.FindByID(nil, user.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "auth-findid@test.com", found.Email)
}

func TestAuthDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAuthDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}
