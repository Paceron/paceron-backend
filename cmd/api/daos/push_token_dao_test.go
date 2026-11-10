package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/testutils"
)

func TestNewPushTokenDao(t *testing.T) {
	dao := NewPushTokenDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestPushTokenDao_ImplementsInterface(t *testing.T) {
	dao := NewPushTokenDao(&gorm.DB{})
	var iface PushTokenDaoInterface = dao
	_ = iface
}

func TestPushTokenDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewPushTokenDao(&gorm.DB{})
	})
}

func TestPushTokenDao_Upsert_CreatesNew(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)
	user := persistUser(db, "push-create@test.com", "91000001")

	err := dao.Upsert(nil, user.ID, "ExponentPushToken[new-1]", "android")

	require.NoError(t, err)
	tokens, err := dao.FindByUserID(nil, user.ID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, "ExponentPushToken[new-1]", tokens[0].Token)
	assert.Equal(t, "android", tokens[0].Platform)
}

// TestPushTokenDao_Upsert_ReassignsOwner cubre el requerimiento central: si el mismo
// token (dispositivo) se registra con otro user_id, el dueño se reescribe solo.
func TestPushTokenDao_Upsert_ReassignsOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)
	firstUser := persistUser(db, "push-owner-1@test.com", "91000002")
	secondUser := persistUser(db, "push-owner-2@test.com", "91000003")

	require.NoError(t, dao.Upsert(nil, firstUser.ID, "ExponentPushToken[shared]", "android"))
	require.NoError(t, dao.Upsert(nil, secondUser.ID, "ExponentPushToken[shared]", "android"))

	firstUserTokens, err := dao.FindByUserID(nil, firstUser.ID)
	require.NoError(t, err)
	assert.Empty(t, firstUserTokens, "el primer usuario ya no debe tener el token")

	secondUserTokens, err := dao.FindByUserID(nil, secondUser.ID)
	require.NoError(t, err)
	require.Len(t, secondUserTokens, 1)
	assert.Equal(t, "ExponentPushToken[shared]", secondUserTokens[0].Token)
}

func TestPushTokenDao_Upsert_UpdatesPlatform(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)
	user := persistUser(db, "push-platform@test.com", "91000004")

	require.NoError(t, dao.Upsert(nil, user.ID, "ExponentPushToken[platform-change]", "android"))
	require.NoError(t, dao.Upsert(nil, user.ID, "ExponentPushToken[platform-change]", "web"))

	tokens, err := dao.FindByUserID(nil, user.ID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, "web", tokens[0].Platform)
}

func TestPushTokenDao_FindByUserID_MultipleDevices(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)
	user := persistUser(db, "push-multi@test.com", "91000005")

	require.NoError(t, dao.Upsert(nil, user.ID, "ExponentPushToken[device-1]", "android"))
	require.NoError(t, dao.Upsert(nil, user.ID, "ExponentPushToken[device-2]", "web"))

	tokens, err := dao.FindByUserID(nil, user.ID)

	require.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestPushTokenDao_FindByUserID_NoTokens(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)
	user := persistUser(db, "push-none@test.com", "91000006")

	tokens, err := dao.FindByUserID(nil, user.ID)

	require.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestPushTokenDao_FindByUserIDs_MultipleUsers(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)
	first := persistUser(db, "push-batch-1@test.com", "91000007")
	second := persistUser(db, "push-batch-2@test.com", "91000008")
	third := persistUser(db, "push-batch-3@test.com", "91000009")

	require.NoError(t, dao.Upsert(nil, first.ID, "ExponentPushToken[batch-1]", "android"))
	require.NoError(t, dao.Upsert(nil, second.ID, "ExponentPushToken[batch-2]", "web"))

	tokens, err := dao.FindByUserIDs(nil, []int64{first.ID, second.ID, third.ID})

	require.NoError(t, err)
	require.Len(t, tokens, 2)
	assert.ElementsMatch(t, []string{"ExponentPushToken[batch-1]", "ExponentPushToken[batch-2]"},
		[]string{tokens[0].Token, tokens[1].Token})
}

func TestPushTokenDao_FindByUserIDs_Empty(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPushTokenDao(db)

	tokens, err := dao.FindByUserIDs(nil, nil)

	require.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestPushTokenDao_FindByUserIDs_BatchesAt500(t *testing.T) {
	db := testutils.SetupTestDB(t)
	counting, counter := countingDB(db)
	dao := NewPushTokenDao(counting)

	ids := make([]int64, 1201)
	for i := range ids {
		ids[i] = int64(i + 1)
	}

	_, err := dao.FindByUserIDs(nil, ids)
	require.NoError(t, err)
	assert.Equal(t, int64(3), *counter, "1201 IDs = lotes de 500/500/201")

	*counter = 0
	_, err = dao.FindByUserIDs(nil, ids[:500])
	require.NoError(t, err)
	assert.Equal(t, int64(1), *counter, "hasta 500 IDs entra en una query")

	*counter = 0
	_, err = dao.FindByUserIDs(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), *counter, "sin IDs no consulta")
}
