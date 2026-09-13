package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewTierSubscriptionDao(t *testing.T) {
	dao := NewTierSubscriptionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTierSubscriptionDao_ImplementsInterface(t *testing.T) {
	dao := NewTierSubscriptionDao(&gorm.DB{})
	var iface TierSubscriptionDaoInterface = dao
	_ = iface
}

func TestTierSubscriptionDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTierSubscriptionDao(&gorm.DB{})
	})
}

func persistSubscription(db *gorm.DB, userID, roleID, tierID int64, status string) *dbs.UserRoleTierSubscription {
	sub := &dbs.UserRoleTierSubscription{
		UserID:     userID,
		RoleID:     roleID,
		TierID:     tierID,
		Status:     status,
		InitAmount: 1000,
		StartDate:  time.Now(),
	}
	db.Create(sub)
	return sub
}

func TestTierSubscriptionDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-create@test.com", "31000001")
	role := testRole(db, "role_for_ts_create")

	sub := &dbs.UserRoleTierSubscription{
		UserID:     user.ID,
		RoleID:     role.ID,
		TierID:     1,
		Status:     string(constants.SubscriptionStatusFirstPaymentPending),
		InitAmount: 1500,
		StartDate:  time.Now(),
	}
	err := dao.Create(nil, sub)

	require.NoError(t, err)
	assert.NotZero(t, sub.ID)
}

func TestTierSubscriptionDao_FindActiveByUserRole_OnlyActive(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-active@test.com", "31000002")
	role := testRole(db, "role_for_ts_active")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusFirstPaymentPending))
	require.NoError(t, dao.Create(nil, &dbs.UserRoleTierSubscription{
		UserID:     user.ID,
		RoleID:     role.ID,
		TierID:     2,
		Status:     string(constants.SubscriptionStatusEnded),
		InitAmount: 1500,
		StartDate:  time.Now(),
		EndedDate:  &time.Time{},
	}))

	found, err := dao.FindActiveByUserRole(nil, user.ID, role.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, sub.ID, found.ID)
	_ = dao
}

func TestTierSubscriptionDao_FindActiveByUserRole_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)

	found, err := dao.FindActiveByUserRole(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierSubscriptionDao_SetEnded_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-ended@test.com", "31000003")
	role := testRole(db, "role_for_ts_ended")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))
	require.NoError(t, dao.SetEnded(nil, sub.ID))

	latest, err := dao.FindLatestByUserRole(nil, user.ID, role.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, string(constants.SubscriptionStatusEnded), latest.Status)
	assert.NotNil(t, latest.EndedDate)
}

func TestTierSubscriptionDao_IncrementPaidInstallments(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-incr@test.com", "31000004")
	role := testRole(db, "role_for_ts_incr")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))
	require.NoError(t, dao.IncrementPaidInstallments(nil, sub.ID))

	latest, err := dao.FindLatestByUserRole(nil, user.ID, role.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, 1, latest.PaidInstallments)
}

func TestTierSubscriptionDao_FindLatestByUserRole_Ordering(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-latest@test.com", "31000005")
	role := testRole(db, "role_for_ts_latest")

	first := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusEnded))
	first.EndedDate = &time.Time{}
	require.NoError(t, db.Save(first).Error)
	_ = first
	second := persistSubscription(db, user.ID, role.ID, 2, string(constants.SubscriptionStatusFirstPaymentPending))

	latest, err := dao.FindLatestByUserRole(nil, user.ID, role.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, second.ID, latest.ID)
}

// TestTierSubscriptionDao_PartialUniqueIndex cubre el escenario "una sola
// suscripción vigente por (user_id, role_id)": el índice único parcial creado
// con SQL crudo en la migración rechaza una segunda sub vigente.
func TestTierSubscriptionDao_PartialUniqueIndex(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-unique@test.com", "31000006")
	role := testRole(db, "role_for_ts_unique")

	persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))

	duplicate := &dbs.UserRoleTierSubscription{
		UserID:     user.ID,
		RoleID:     role.ID,
		TierID:     2,
		Status:     string(constants.SubscriptionStatusActive),
		InitAmount: 1500,
		StartDate:  time.Now(),
	}
	err := dao.Create(nil, duplicate)

	require.Error(t, err)
}

func TestTierSubscriptionDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-findid@test.com", "31000007")
	role := testRole(db, "role_for_ts_findid")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))

	found, err := dao.FindByID(nil, sub.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, sub.ID, found.ID)
	assert.Equal(t, user.ID, found.UserID)
}

func TestTierSubscriptionDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTierSubscriptionDao_Activate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-activate@test.com", "31000008")
	role := testRole(db, "role_for_ts_activate")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusFirstPaymentPending))
	require.NoError(t, dao.Activate(nil, sub.ID))

	found, err := dao.FindByID(nil, sub.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SubscriptionStatusActive), found.Status)
}

func TestTierSubscriptionDao_FindActiveByUserRole_FilterByStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-filter@test.com", "31000009")
	role := testRole(db, "role_for_ts_filter")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))

	foundActive, err := dao.FindActiveByUserRole(nil, user.ID, role.ID, string(constants.SubscriptionStatusActive))
	require.NoError(t, err)
	require.NotNil(t, foundActive)
	assert.Equal(t, sub.ID, foundActive.ID)

	foundPending, err := dao.FindActiveByUserRole(nil, user.ID, role.ID, string(constants.SubscriptionStatusFirstPaymentPending))
	require.NoError(t, err)
	assert.Nil(t, foundPending)
}

func TestTierSubscriptionDao_SetCanceled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-cancel@test.com", "31000010")
	role := testRole(db, "role_for_ts_cancel")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusFirstPaymentPending))
	require.NoError(t, dao.SetCanceled(nil, sub.ID))

	found, err := dao.FindByID(nil, sub.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, string(constants.SubscriptionStatusCanceled), found.Status)
}

// TestTierSubscriptionDao_CanceledFreesUniqueSlot: cancelar una sub pendiente la
// saca del conjunto vigente (el índice único parcial solo cubre active y
// first_payment_pending), habilitando crear una suscripción vigente nueva.
func TestTierSubscriptionDao_CanceledFreesUniqueSlot(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-cancelfree@test.com", "31000011")
	role := testRole(db, "role_for_ts_cancelfree")

	pending := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusFirstPaymentPending))
	require.NoError(t, dao.SetCanceled(nil, pending.ID))

	refound, err := dao.FindActiveByUserRole(nil, user.ID, role.ID)
	require.NoError(t, err)
	assert.Nil(t, refound)

	// Una nueva sub vigente para el mismo (user_id, role_id) ya no viola el índice.
	newSub := persistSubscription(db, user.ID, role.ID, 2, string(constants.SubscriptionStatusActive))
	assert.NotZero(t, newSub.ID)
}

func TestTierSubscriptionDao_FindPendingByUserRoleTier(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-pendtier@test.com", "31000012")
	role := testRole(db, "role_for_ts_pendtier")

	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusFirstPaymentPending))

	found, err := dao.FindPendingByUserRoleTier(nil, user.ID, role.ID, sub.TierID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, sub.ID, found.ID)

	missing, err := dao.FindPendingByUserRoleTier(nil, user.ID, role.ID, 999)
	require.NoError(t, err)
	assert.Nil(t, missing)

	require.NoError(t, dao.SetEnded(nil, sub.ID))
	gone, err := dao.FindPendingByUserRoleTier(nil, user.ID, role.ID, sub.TierID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestTierSubscriptionDao_FindByUserRoleTier(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTierSubscriptionDao(db)
	user := persistUser(db, "ts-byurt@test.com", "31000013")
	role := testRole(db, "role_for_ts_byurt")

	oldSub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusEnded))
	oldSub.EndedDate = &time.Time{}
	require.NoError(t, db.Save(oldSub).Error)
	pending := persistSubscription(db, user.ID, role.ID, 2, string(constants.SubscriptionStatusFirstPaymentPending))

	foundOld, err := dao.FindByUserRoleTier(nil, user.ID, role.ID, 1)
	require.NoError(t, err)
	require.NotNil(t, foundOld)
	assert.Equal(t, oldSub.ID, foundOld.ID)

	foundPending, err := dao.FindByUserRoleTier(nil, user.ID, role.ID, 2)
	require.NoError(t, err)
	require.NotNil(t, foundPending)
	assert.Equal(t, pending.ID, foundPending.ID)

	missing, err := dao.FindByUserRoleTier(nil, user.ID, role.ID, 999)
	require.NoError(t, err)
	assert.Nil(t, missing)
}