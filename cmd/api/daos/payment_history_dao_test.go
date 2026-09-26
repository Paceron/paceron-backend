package daos

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewPaymentHistoryDao(t *testing.T) {
	assert.NotNil(t, NewPaymentHistoryDao(&gorm.DB{}))
}

func TestPaymentHistoryDao_ImplementsInterface(t *testing.T) {
	var iface PaymentHistoryDaoInterface = NewPaymentHistoryDao(&gorm.DB{})
	_ = iface
}

func TestTrimPage(t *testing.T) {
	rows, hasMore, err := trimPage([]int{1, 2, 3}, 2)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, rows)
	assert.True(t, hasMore)

	rows, hasMore, _ = trimPage([]int{1, 2}, 2)
	assert.Equal(t, []int{1, 2}, rows)
	assert.False(t, hasMore)
}

// --- fixtures ---

const webhookRaw = `{"id":1319998877,"status":"approved","transaction_details":{"net_received_amount":14101.5,"total_paid_amount":15000}}`
const processPaymentRaw = `{"ID":1319998877,"Status":"approved","StatusDetail":"accredited","FeeDetailsRaw":null}`

func ptrStr(s string) *string { return &s }

func persistTeamInstallment(db *gorm.DB, teamID, userID int64, number int) *dbs.Installment {
	inst := &dbs.Installment{
		TeamID:            &teamID,
		UserID:            userID,
		InstallmentNumber: number,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            15000,
	}
	db.Create(inst)
	return inst
}

type paymentFixture struct {
	sellerID      *int64
	installmentID int64
	concept       string
	mpPaymentID   string
	status        string
	amount        float64
	raw           *string
	createdAt     time.Time
}

func persistPayment(db *gorm.DB, f paymentFixture) *dbs.Payment {
	p := &dbs.Payment{
		Concept:       f.concept,
		Amount:        f.amount,
		CurrencyID:    "ARS",
		Status:        f.status,
		PaymentID:     f.mpPaymentID,
		SellerUserID:  f.sellerID,
		InstallmentID: &f.installmentID,
		RawResponse:   f.raw,
		CreatedAt:     f.createdAt,
	}
	db.Create(p)
	return p
}

// receivedSeed crea un entrenador con un equipo y un corredor con una cuota.
type receivedSeed struct {
	trainer *dbs.User
	runner  *dbs.User
	team    *dbs.Team
	inst    *dbs.Installment
}

func seedReceived(db *gorm.DB, suffix string) receivedSeed {
	trainer := persistUser(db, "ph-trainer-"+suffix+"@test.com", "41"+suffix)
	runner := persistUser(db, "ph-runner-"+suffix+"@test.com", "42"+suffix)
	team := testTeam(db, "Equipo "+suffix, trainer.ID)
	inst := persistTeamInstallment(db, team.ID, runner.ID, 1)
	return receivedSeed{trainer: trainer, runner: runner, team: team, inst: inst}
}

func teamPayment(s receivedSeed, mpID, status string, raw *string, createdAt time.Time) paymentFixture {
	return paymentFixture{
		sellerID:      &s.trainer.ID,
		installmentID: s.inst.ID,
		concept:       string(constants.PaymentConceptTeamSubscription),
		mpPaymentID:   mpID,
		status:        status,
		amount:        15000,
		raw:           raw,
		createdAt:     createdAt,
	}
}

// --- ListReceived ---

func TestPaymentHistoryDao_ListReceived_FiltersBySellerAndResolvesRefs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	mine := seedReceived(db, "000001")
	other := seedReceived(db, "000002")
	now := time.Now().UTC().Truncate(time.Second)

	persistPayment(db, teamPayment(mine, "mp-1", "approved", ptrStr(webhookRaw), now))
	persistPayment(db, teamPayment(other, "mp-2", "approved", nil, now))

	rows, hasMore, err := dao.ListReceived(nil, mine.trainer.ID, ReceivedPaymentFilters{}, 1, 20)

	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, rows, 1)
	r := rows[0]
	assert.Equal(t, "mp-1", r.MPPaymentID)
	assert.Equal(t, float64(15000), r.GrossAmount)
	assert.Equal(t, mine.team.ID, r.TeamID)
	require.NotNil(t, r.TeamName)
	assert.Equal(t, mine.team.Name, *r.TeamName)
	assert.Equal(t, mine.runner.ID, r.PayerID)
	require.NotNil(t, r.PayerEmail)
	assert.Equal(t, mine.runner.Email, *r.PayerEmail)
	assert.Equal(t, 1, r.InstallmentNumber)
}

func TestPaymentHistoryDao_ListReceived_ExcludesRowsWithoutMPPaymentID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000003")
	now := time.Now().UTC()

	persistPayment(db, teamPayment(s, "", "pending", nil, now))
	persistPayment(db, teamPayment(s, "mp-real", "approved", nil, now))

	rows, _, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{}, 1, 20)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "mp-real", rows[0].MPPaymentID)
}

func TestPaymentHistoryDao_ListReceived_ExcludesOtherConcepts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000004")
	f := teamPayment(s, "mp-order", "approved", nil, time.Now().UTC())
	f.concept = string(constants.PaymentConceptOrder)
	persistPayment(db, f)

	rows, _, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{}, 1, 20)

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestPaymentHistoryDao_ListReceived_NetAmount(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000005")
	base := time.Now().UTC().Add(-time.Hour)

	persistPayment(db, teamPayment(s, "mp-webhook", "approved", ptrStr(webhookRaw), base.Add(4*time.Minute)))
	persistPayment(db, teamPayment(s, "mp-process", "approved", ptrStr(processPaymentRaw), base.Add(3*time.Minute)))
	persistPayment(db, teamPayment(s, "mp-nil-raw", "approved", nil, base.Add(2*time.Minute)))
	persistPayment(db, teamPayment(s, "mp-pending", "pending",
		ptrStr(`{"transaction_details":{"net_received_amount":0}}`), base.Add(time.Minute)))
	persistPayment(db, teamPayment(s, "mp-rejected", "rejected", ptrStr(webhookRaw), base))

	rows, _, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{}, 1, 20)

	require.NoError(t, err)
	require.Len(t, rows, 5)
	byID := map[string]*float64{}
	for _, r := range rows {
		byID[r.MPPaymentID] = r.NetAmount
	}
	require.NotNil(t, byID["mp-webhook"])
	assert.InDelta(t, 14101.5, *byID["mp-webhook"], 0.001)
	assert.Nil(t, byID["mp-process"])
	assert.Nil(t, byID["mp-nil-raw"])
	assert.Nil(t, byID["mp-pending"])
	assert.Nil(t, byID["mp-rejected"])
}

func TestPaymentHistoryDao_ListReceived_Filters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000006")
	otherTeam := testTeam(db, "Otro equipo", s.trainer.ID)
	otherInst := persistTeamInstallment(db, otherTeam.ID, s.runner.ID, 1)
	now := time.Now().UTC()

	persistPayment(db, teamPayment(s, "mp-a", "approved", nil, now))
	persistPayment(db, teamPayment(s, "mp-b", "rejected", nil, now))
	f := teamPayment(s, "mp-c", "cancelled", nil, now)
	f.installmentID = otherInst.ID
	persistPayment(db, f)

	byTeam, _, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{TeamID: &s.team.ID}, 1, 20)
	require.NoError(t, err)
	assert.Len(t, byTeam, 2)

	byStatus, _, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{Statuses: []string{"rejected", "cancelled"}}, 1, 20)
	require.NoError(t, err)
	assert.Len(t, byStatus, 2)

	both, _, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{TeamID: &s.team.ID, Statuses: []string{"rejected", "cancelled"}}, 1, 20)
	require.NoError(t, err)
	require.Len(t, both, 1)
	assert.Equal(t, "mp-b", both[0].MPPaymentID)
}

func TestPaymentHistoryDao_ListReceived_OrderAndPagination(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000007")
	base := time.Now().UTC().Add(-48 * time.Hour)
	for i := 0; i < 21; i++ {
		persistPayment(db, teamPayment(s, fmt.Sprintf("mp-%02d", i), "approved", nil, base.Add(time.Duration(i)*time.Minute)))
	}

	page1, more1, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{}, 1, 20)
	require.NoError(t, err)
	assert.Len(t, page1, 20)
	assert.True(t, more1)
	assert.Equal(t, "mp-20", page1[0].MPPaymentID)

	page2, more2, err := dao.ListReceived(nil, s.trainer.ID, ReceivedPaymentFilters{}, 2, 20)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.False(t, more2)
	assert.Equal(t, "mp-00", page2[0].MPPaymentID)
}

func TestPaymentHistoryDao_ListReceivedSince(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000008")
	cut := time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)

	persistPayment(db, teamPayment(s, "mp-before", "approved", nil, cut.Add(-time.Minute)))
	persistPayment(db, teamPayment(s, "mp-at", "approved", nil, cut))
	persistPayment(db, teamPayment(s, "mp-after", "rejected", nil, cut.Add(time.Hour)))

	rows, err := dao.ListReceivedSince(nil, s.trainer.ID, cut)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "mp-after", rows[0].MPPaymentID)
	assert.Equal(t, "mp-at", rows[1].MPPaymentID)
}

// --- ListMyTierPayments ---

func TestPaymentHistoryDao_ListMyTierPayments(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	user := persistUser(db, "ph-tier-user@test.com", "43000001")
	otherUser := persistUser(db, "ph-tier-other@test.com", "43000002")
	trainerRole := testRole(db, "entrenador_ph")
	runnerRole := testRole(db, "corredor_ph")
	trainerTier := &dbs.Tier{Name: "Premium_entrenador", RoleID: trainerRole.ID, RoleName: "entrenador"}
	db.Create(trainerTier)
	runnerTier := &dbs.Tier{Name: "Premium_corredor", RoleID: runnerRole.ID, RoleName: "corredor"}
	db.Create(runnerTier)

	trainerSub := persistSubscription(db, user.ID, trainerRole.ID, trainerTier.ID, "active")
	runnerSub := persistSubscription(db, user.ID, runnerRole.ID, runnerTier.ID, "active")
	otherSub := persistSubscription(db, otherUser.ID, trainerRole.ID, trainerTier.ID, "active")
	trainerInst := persistTierInstallment(db, trainerSub.ID, user.ID, 2)
	due := time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC)
	db.Model(trainerInst).Update("due_date", due)
	runnerInst := persistTierInstallment(db, runnerSub.ID, user.ID, 1)
	otherInst := persistTierInstallment(db, otherSub.ID, otherUser.ID, 1)

	// Cuota de equipo del mismo usuario: no es un pago de tier.
	owner := persistUser(db, "ph-tier-owner@test.com", "43000003")
	team := testTeam(db, "Equipo tier", owner.ID)
	teamInst := persistTeamInstallment(db, team.ID, user.ID, 1)

	now := time.Now().UTC()
	tierPayment := func(instID int64, mpID string, at time.Time) paymentFixture {
		return paymentFixture{installmentID: instID, concept: string(constants.PaymentConceptOrder),
			mpPaymentID: mpID, status: "approved", amount: 9999, createdAt: at}
	}
	persistPayment(db, tierPayment(trainerInst.ID, "mp-trainer", now))
	persistPayment(db, tierPayment(runnerInst.ID, "mp-runner", now.Add(-time.Minute)))
	persistPayment(db, tierPayment(otherInst.ID, "mp-other", now))
	persistPayment(db, tierPayment(trainerInst.ID, "", now))
	persistPayment(db, tierPayment(teamInst.ID, "mp-team", now))

	all, hasMore, err := dao.ListMyTierPayments(nil, user.ID, "", 1, 20)
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, all, 2)
	assert.Equal(t, "mp-trainer", all[0].MPPaymentID)
	assert.Equal(t, "mp-runner", all[1].MPPaymentID)

	onlyTrainer, _, err := dao.ListMyTierPayments(nil, user.ID, "entrenador", 1, 20)
	require.NoError(t, err)
	require.Len(t, onlyTrainer, 1)
	r := onlyTrainer[0]
	assert.Equal(t, trainerSub.ID, r.SubscriptionID)
	assert.Equal(t, 2, r.InstallmentNumber)
	require.NotNil(t, r.DueDate)
	assert.True(t, due.Equal(*r.DueDate))
	require.NotNil(t, r.TierName)
	assert.Equal(t, "Premium_entrenador", *r.TierName)
	require.NotNil(t, r.TierRoleName)
	assert.Equal(t, "entrenador", *r.TierRoleName)
}

func TestPaymentHistoryDao_ListMyTierPayments_Pagination(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	user := persistUser(db, "ph-tier-page@test.com", "43000004")
	role := testRole(db, "entrenador_ph_page")
	tier := testTier(db, "base_ph_page", role.ID)
	sub := persistSubscription(db, user.ID, role.ID, tier.ID, "active")
	inst := persistTierInstallment(db, sub.ID, user.ID, 1)
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		persistPayment(db, paymentFixture{installmentID: inst.ID, concept: "subscription",
			mpPaymentID: fmt.Sprintf("mp-t%d", i), status: "rejected", amount: 100, createdAt: base.Add(time.Duration(i) * time.Minute)})
	}

	page1, more, err := dao.ListMyTierPayments(nil, user.ID, "", 1, 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.True(t, more)

	page2, more, err := dao.ListMyTierPayments(nil, user.ID, "", 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 1)
	assert.False(t, more)
}

// --- ramas de error de DB ---

func TestPaymentHistoryDao_DBFail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewPaymentHistoryDao(failing)

	_, _, err := dao.ListReceived(nil, 1, ReceivedPaymentFilters{}, 1, 20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing received payments")

	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewPaymentHistoryDao(failing)
	_, err = dao.ListReceivedSince(nil, 1, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing received payments since date")

	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewPaymentHistoryDao(failing)
	_, _, err = dao.ListMyTierPayments(nil, 1, "entrenador", 1, 20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing tier payments")
}
