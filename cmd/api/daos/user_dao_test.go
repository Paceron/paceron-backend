package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/testutils"
)

func TestNewUserDao(t *testing.T) {
	dao := NewUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestUserDao_ImplementsInterface(t *testing.T) {
	dao := NewUserDao(&gorm.DB{})
	var iface UserDaoInterface = dao
	_ = iface
}

func TestUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewUserDao(&gorm.DB{})
	})
}

func TestUserDao_GetByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	user := persistUser(db, "ud-getbyid@test.com", "11100001")

	found, err := dao.GetByID(nil, user.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "ud-getbyid@test.com", found.Email)
}

func TestUserDao_GetByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)

	found, err := dao.GetByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUserDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	user := persistUser(db, "ud-findid@test.com", "11100002")

	found, err := dao.FindByID(nil, user.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, user.ID, found.ID)
}

func TestUserDao_FindByEmail_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	persistUser(db, "ud-findemail@test.com", "11100003")

	found, err := dao.FindByEmail(nil, "ud-findemail@test.com")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "11100003", found.DNI)
}

func TestUserDao_FindByEmail_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)

	found, err := dao.FindByEmail(nil, "no-existe@test.com")

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestUserDao_Update_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	user := persistUser(db, "ud-update@test.com", "11100004")

	user.Name = "Actualizado"
	err := dao.Update(nil, user)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, user.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "Actualizado", found.Name)
}

func TestUserDao_UpdateStatus_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	user := persistUser(db, "ud-updatestatus@test.com", "11100005")

	err := dao.UpdateStatus(nil, user.ID, "inactive")

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, user.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "inactive", found.Status)
}

func TestUserDao_SearchActive_MatchesNameSurnameEmail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	match := persistUser(db, "search-target@test.com", "11100006")
	match.Name = "Buscable"
	require.NoError(t, dao.Update(nil, match))
	persistUser(db, "no-match@test.com", "11100007")

	results, err := dao.SearchActive(nil, "buscable", 5)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, match.ID, results[0].ID)
}

func TestUserDao_SearchActive_ExcludesInactive(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	inactive := persistUser(db, "search-inactive@test.com", "11100008")
	inactive.Name = "Inactivobuscable"
	inactive.Status = "inactive"
	require.NoError(t, dao.Update(nil, inactive))

	results, err := dao.SearchActive(nil, "inactivobuscable", 5)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUserDao_SearchActive_RespectsLimit(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	for i := 0; i < 3; i++ {
		u := persistUser(db, "search-limit-"+string(rune('a'+i))+"@test.com", "1120000"+string(rune('0'+i)))
		u.Name = "Limitable"
		require.NoError(t, dao.Update(nil, u))
	}

	results, err := dao.SearchActive(nil, "limitable", 2)

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestUserDao_FindByIDs_ReturnsMatchingOnly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)
	user1 := persistUser(db, "batch1@test.com", "11100010")
	user2 := persistUser(db, "batch2@test.com", "11100011")
	persistUser(db, "batch3@test.com", "11100012")

	results, err := dao.FindByIDs(nil, []int64{user1.ID, user2.ID, 999999})

	require.NoError(t, err)
	require.Len(t, results, 2)
	ids := []int64{results[0].ID, results[1].ID}
	assert.Contains(t, ids, user1.ID)
	assert.Contains(t, ids, user2.ID)
}

func TestUserDao_FindByIDs_EmptyResult(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewUserDao(db)

	results, err := dao.FindByIDs(nil, []int64{999999})

	require.NoError(t, err)
	assert.Empty(t, results)
}
