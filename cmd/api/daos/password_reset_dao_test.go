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

func TestNewPasswordResetDao(t *testing.T) {
	dao := NewPasswordResetDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestPasswordResetDao_ImplementsInterface(t *testing.T) {
	dao := NewPasswordResetDao(&gorm.DB{})
	var iface PasswordResetDaoInterface = dao
	_ = iface
}

func TestPasswordResetDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewPasswordResetDao(&gorm.DB{})
	})
}

func TestPasswordResetDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)
	user := persistUser(db, "pr-create@test.com", "70000001")

	token := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "hash", ExpiresAt: time.Now().Add(15 * time.Minute)}
	err := dao.Create(nil, token)

	require.NoError(t, err)
	assert.NotZero(t, token.ID)
}

func TestPasswordResetDao_FindActiveByUserID_ReturnsMostRecent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)
	user := persistUser(db, "pr-active@test.com", "70000002")

	older := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "old", ExpiresAt: time.Now().Add(15 * time.Minute)}
	require.NoError(t, dao.Create(nil, older))
	newer := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "new", ExpiresAt: time.Now().Add(15 * time.Minute)}
	require.NoError(t, dao.Create(nil, newer))

	found, err := dao.FindActiveByUserID(nil, user.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, newer.ID, found.ID)
}

func TestPasswordResetDao_FindActiveByUserID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)

	found, err := dao.FindActiveByUserID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPasswordResetDao_FindActiveByUserID_ExcludesUsed(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)
	user := persistUser(db, "pr-used@test.com", "70000003")

	token := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "used", ExpiresAt: time.Now().Add(15 * time.Minute)}
	require.NoError(t, dao.Create(nil, token))
	require.NoError(t, dao.MarkUsed(nil, token.ID))

	found, err := dao.FindActiveByUserID(nil, user.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPasswordResetDao_IncrementAttempts_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)
	user := persistUser(db, "pr-attempts@test.com", "70000004")
	token := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "hash", ExpiresAt: time.Now().Add(15 * time.Minute)}
	require.NoError(t, dao.Create(nil, token))

	require.NoError(t, dao.IncrementAttempts(nil, token.ID))
	require.NoError(t, dao.IncrementAttempts(nil, token.ID))

	found, err := dao.FindActiveByUserID(nil, user.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, 2, found.Attempts)
}

func TestPasswordResetDao_MarkUsed_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)
	user := persistUser(db, "pr-markused@test.com", "70000005")
	token := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "hash", ExpiresAt: time.Now().Add(15 * time.Minute)}
	require.NoError(t, dao.Create(nil, token))

	err := dao.MarkUsed(nil, token.ID)

	require.NoError(t, err)
	found, findErr := dao.FindActiveByUserID(nil, user.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}

func TestPasswordResetDao_SoftDeleteByUserID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPasswordResetDao(db)
	user := persistUser(db, "pr-softdelete@test.com", "70000006")
	token := &dbs.PasswordResetToken{UserID: user.ID, CodeHash: "hash", ExpiresAt: time.Now().Add(15 * time.Minute)}
	require.NoError(t, dao.Create(nil, token))

	err := dao.SoftDeleteByUserID(nil, user.ID)

	require.NoError(t, err)
	found, findErr := dao.FindActiveByUserID(nil, user.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
