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

func TestNewInstallmentDao(t *testing.T) {
	dao := NewInstallmentDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestInstallmentDao_ImplementsInterface(t *testing.T) {
	dao := NewInstallmentDao(&gorm.DB{})
	var iface InstallmentDaoInterface = dao
	_ = iface
}

func TestInstallmentDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewInstallmentDao(&gorm.DB{})
	})
}

func persistTierInstallment(db *gorm.DB, subscriptionID, userID int64, number int) *dbs.Installment {
	inst := &dbs.Installment{
		SubscriptionID:    &subscriptionID,
		UserID:            userID,
		InstallmentNumber: number,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            1500,
	}
	db.Create(inst)
	return inst
}

func TestInstallmentDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-create@test.com", "32000001")
	role := testRole(db, "role_for_inst_create")
	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusFirstPaymentPending))

	inst := &dbs.Installment{
		SubscriptionID:    &sub.ID,
		UserID:            user.ID,
		InstallmentNumber: 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            1500,
	}
	err := dao.Create(nil, inst)

	require.NoError(t, err)
	assert.NotZero(t, inst.ID)
}

func TestInstallmentDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestInstallmentDao_MarkPaidConditional_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-mark@test.com", "32000002")
	role := testRole(db, "role_for_inst_mark")
	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))
	inst := persistTierInstallment(db, sub.ID, user.ID, 2)

	internalID := int64(99)
	externalID := "MP-123"
	affected, err := dao.MarkPaidConditional(nil, inst.ID, &internalID, &externalID)

	require.NoError(t, err)
	assert.True(t, affected)

	updated, err := dao.FindByID(nil, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, string(constants.InstallmentStatusPaid), updated.Status)
	require.NotNil(t, updated.InternalPaymentID)
	assert.Equal(t, internalID, *updated.InternalPaymentID)
	require.NotNil(t, updated.ExternalPaymentID)
	assert.Equal(t, externalID, *updated.ExternalPaymentID)
}

// TestInstallmentDao_MarkPaidConditional_Idempotent cubre el escenario de doble
// notificación del webhook: el segundo marcado no afecta filas.
func TestInstallmentDao_MarkPaidConditional_Idempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-idem@test.com", "32000003")
	role := testRole(db, "role_for_inst_idem")
	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))
	inst := persistTierInstallment(db, sub.ID, user.ID, 1)

	internalID := int64(99)
	externalID := "MP-123"
	_, err := dao.MarkPaidConditional(nil, inst.ID, &internalID, &externalID)
	require.NoError(t, err)

	affected, err := dao.MarkPaidConditional(nil, inst.ID, &internalID, &externalID)
	require.NoError(t, err)
	assert.False(t, affected)
}

func TestInstallmentDao_FindPendingBySubscription_ExcludesPaid(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-pending@test.com", "32000004")
	role := testRole(db, "role_for_inst_pending")
	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))

	pending := persistTierInstallment(db, sub.ID, user.ID, 2)
	paid := persistTierInstallment(db, sub.ID, user.ID, 1)
	internalID := int64(100)
	externalID := "MP-100"
	_, err := dao.MarkPaidConditional(nil, paid.ID, &internalID, &externalID)
	require.NoError(t, err)

	pendingList, err := dao.FindPendingBySubscription(nil, sub.ID)

	require.NoError(t, err)
	require.Len(t, pendingList, 1)
	assert.Equal(t, pending.ID, pendingList[0].ID)
}

func TestInstallmentDao_FindNext_ReturnsLowestPending(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-next@test.com", "32000005")
	role := testRole(db, "role_for_inst_next")
	sub := persistSubscription(db, user.ID, role.ID, 1, string(constants.SubscriptionStatusActive))

	inst2 := persistTierInstallment(db, sub.ID, user.ID, 2)
	inst1 := persistTierInstallment(db, sub.ID, user.ID, 1)

	next, err := dao.FindNext(nil, sub.ID)

	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Equal(t, inst1.ID, next.ID)
	assert.NotEqual(t, inst2.ID, next.ID)
}

func TestInstallmentDao_FindNext_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)

	next, err := dao.FindNext(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, next)
}

func TestInstallmentDao_FindPendingByUserTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	owner := persistUser(db, "inst-team-owner@test.com", "32000006")
	member := persistUser(db, "inst-team-member@test.com", "32000007")
	team := testTeam(db, "equipo_inst_team", owner.ID)

	inst := &dbs.Installment{
		TeamID:            &team.ID,
		UserID:            member.ID,
		InstallmentNumber: 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            5000,
	}
	require.NoError(t, dao.Create(nil, inst))

	list, err := dao.FindPendingByUserTeam(nil, team.ID, member.ID)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, inst.ID, list[0].ID)
}

// TestInstallmentDao_ExclusiveArc cubre el CHECK de arco exclusivo: una cuota
// debe referenciar exactamente uno de subscription_id o team_id.
func TestInstallmentDao_ExclusiveArc_BothNilFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-arcnil@test.com", "32000008")

	inst := &dbs.Installment{
		UserID:            user.ID,
		InstallmentNumber: 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            1500,
	}
	err := dao.Create(nil, inst)

	require.Error(t, err)
}

func TestInstallmentDao_ExclusiveArc_BothSetFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInstallmentDao(db)
	user := persistUser(db, "inst-arcboth@test.com", "32000009")
	owner := persistUser(db, "inst-arcboth-owner@test.com", "32000010")
	team := testTeam(db, "equipo_inst_arcboth", owner.ID)

	subID := int64(1)
	inst := &dbs.Installment{
		SubscriptionID:    &subID,
		TeamID:            &team.ID,
		UserID:            user.ID,
		InstallmentNumber: 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            1500,
	}
	err := dao.Create(nil, inst)

	require.Error(t, err)
}