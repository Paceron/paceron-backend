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

func TestNewRefreshTokenDao(t *testing.T) {
	dao := NewRefreshTokenDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestRefreshTokenDao_ImplementsInterface(t *testing.T) {
	dao := NewRefreshTokenDao(&gorm.DB{})
	var iface RefreshTokenDaoInterface = dao
	_ = iface
}

func TestRefreshTokenDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewRefreshTokenDao(&gorm.DB{})
	})
}

func TestRefreshTokenDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-create@test.com", "80000001")

	token := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-1", TokenHash: "hash-create", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	err := dao.Create(nil, token)

	require.NoError(t, err)
	assert.NotZero(t, token.ID)
}

func TestRefreshTokenDao_FindActiveByHash_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-find@test.com", "80000002")
	token := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-2", TokenHash: "hash-find", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, token))

	found, err := dao.FindActiveByHash(nil, "hash-find")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, token.ID, found.ID)
}

func TestRefreshTokenDao_FindActiveByHash_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)

	found, err := dao.FindActiveByHash(nil, "no-existe")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRefreshTokenDao_FindActiveByHash_ExcludesExpired(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-expired@test.com", "80000003")
	token := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-3", TokenHash: "hash-expired", ExpiresAt: time.Now().Add(-1 * time.Hour)}
	require.NoError(t, dao.Create(nil, token))

	found, err := dao.FindActiveByHash(nil, "hash-expired")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRefreshTokenDao_FindActiveByHash_ExcludesRevoked(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-revoked@test.com", "80000004")
	token := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-4", TokenHash: "hash-revoked", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, token))
	require.NoError(t, dao.Revoke(nil, token.ID, nil))

	found, err := dao.FindActiveByHash(nil, "hash-revoked")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestRefreshTokenDao_Revoke_WithReplacedBy(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-replace@test.com", "80000005")
	old := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-5", TokenHash: "hash-old", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, old))
	newToken := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-5", TokenHash: "hash-new", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, newToken))

	err := dao.Revoke(nil, old.ID, &newToken.ID)

	require.NoError(t, err)
	var reloaded dbs.RefreshToken
	require.NoError(t, db.First(&reloaded, old.ID).Error)
	require.NotNil(t, reloaded.RevokedAt)
	require.NotNil(t, reloaded.ReplacedBy)
	assert.Equal(t, newToken.ID, *reloaded.ReplacedBy)
}

func TestRefreshTokenDao_RevokeBySessionID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-bysession@test.com", "80000006")
	token1 := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-6", TokenHash: "hash-s6-1", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	token2 := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-6", TokenHash: "hash-s6-2", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, token1))
	require.NoError(t, dao.Create(nil, token2))

	err := dao.RevokeBySessionID(nil, "session-6")

	require.NoError(t, err)
	found1, _ := dao.FindActiveByHash(nil, "hash-s6-1")
	found2, _ := dao.FindActiveByHash(nil, "hash-s6-2")
	assert.Nil(t, found1)
	assert.Nil(t, found2)
}

func TestRefreshTokenDao_RevokeByUserID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-byuser@test.com", "80000007")
	other := persistUser(db, "rt-byuser-other@test.com", "80000008")
	token1 := &dbs.RefreshToken{UserID: user.ID, SessionID: "u-session-7a", TokenHash: "hash-u7-1", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	token2 := &dbs.RefreshToken{UserID: user.ID, SessionID: "u-session-7b", TokenHash: "hash-u7-2", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	otherToken := &dbs.RefreshToken{UserID: other.ID, SessionID: "o-session-8", TokenHash: "hash-8-1", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, token1))
	require.NoError(t, dao.Create(nil, token2))
	require.NoError(t, dao.Create(nil, otherToken))

	err := dao.RevokeByUserID(nil, user.ID)

	require.NoError(t, err)
	found1, _ := dao.FindActiveByHash(nil, "hash-u7-1")
	found2, _ := dao.FindActiveByHash(nil, "hash-u7-2")
	assert.Nil(t, found1)
	assert.Nil(t, found2)
	foundOther, _ := dao.FindActiveByHash(nil, "hash-8-1")
	require.NotNil(t, foundOther)
	assert.Equal(t, otherToken.ID, foundOther.ID)
}

func TestRefreshTokenDao_RevokeByUserID_Idempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRefreshTokenDao(db)
	user := persistUser(db, "rt-byuser2@test.com", "80000009")
	token := &dbs.RefreshToken{UserID: user.ID, SessionID: "session-9", TokenHash: "hash-9", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	require.NoError(t, dao.Create(nil, token))
	require.NoError(t, dao.RevokeByUserID(nil, user.ID))

	err := dao.RevokeByUserID(nil, user.ID)

	require.NoError(t, err)
	found, _ := dao.FindActiveByHash(nil, "hash-9")
	assert.Nil(t, found)
	var reloaded dbs.RefreshToken
	require.NoError(t, db.First(&reloaded, token.ID).Error)
	require.NotNil(t, reloaded.RevokedAt)
}
