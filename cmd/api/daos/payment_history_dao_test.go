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

func TestPaymentHistoryDao_ListReceivedBetween(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000008")
	from := time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)

	persistPayment(db, teamPayment(s, "mp-before", "approved", nil, from.Add(-time.Minute)))
	persistPayment(db, teamPayment(s, "mp-at-from", "approved", nil, from))
	persistPayment(db, teamPayment(s, "mp-inside", "rejected", nil, from.Add(time.Hour)))
	// El fin del rango es exclusivo: el 1/6 00:00 en Argentina ya es el mes siguiente.
	persistPayment(db, teamPayment(s, "mp-at-to", "approved", nil, to))

	rows, err := dao.ListReceivedBetween(nil, s.trainer.ID, from, to)

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "mp-inside", rows[0].MPPaymentID)
	assert.Equal(t, "mp-at-from", rows[1].MPPaymentID)
}

func TestPaymentHistoryDao_EarliestReceivedAt(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	s := seedReceived(db, "000009")
	other := seedReceived(db, "000010")

	earliest, err := dao.EarliestReceivedAt(nil, s.trainer.ID)
	require.NoError(t, err)
	assert.Nil(t, earliest)

	oldest := time.Date(2026, 2, 10, 15, 0, 0, 0, time.UTC)
	persistPayment(db, teamPayment(s, "mp-old", "rejected", nil, oldest))
	persistPayment(db, teamPayment(s, "mp-new", "approved", nil, oldest.AddDate(0, 3, 0)))
	// Ni la fila fantasma (sin payment_id) ni los cobros de otro vendedor cuentan.
	persistPayment(db, teamPayment(s, "", "pending", nil, oldest.AddDate(-1, 0, 0)))
	persistPayment(db, teamPayment(other, "mp-other", "approved", nil, oldest.AddDate(-2, 0, 0)))

	earliest, err = dao.EarliestReceivedAt(nil, s.trainer.ID)

	require.NoError(t, err)
	require.NotNil(t, earliest)
	assert.True(t, earliest.Equal(oldest), earliest)
}

// --- ListHistory ---

// historySeed arma un usuario con una suscripción de tier y una membresía de
// equipo, más un tercero con sus propios pagos que no tienen que aparecer.
type historySeed struct {
	user        *dbs.User
	trainer     *dbs.User
	team        *dbs.Team
	tierInst    *dbs.Installment
	teamInst    *dbs.Installment
	otherInst   *dbs.Installment
	trainerTier *dbs.Tier
	sub         *dbs.UserRoleTierSubscription
}

func seedHistory(db *gorm.DB) historySeed {
	user := persistUser(db, "ph-hist-user@test.com", "44000001")
	other := persistUser(db, "ph-hist-other@test.com", "44000002")
	trainer := persistUser(db, "ph-hist-trainer@test.com", "44000003")
	role := testRole(db, "entrenador_ph_hist")
	tier := &dbs.Tier{Name: "Premium_entrenador", RoleID: role.ID, RoleName: "entrenador"}
	db.Create(tier)
	sub := persistSubscription(db, user.ID, role.ID, tier.ID, "active")
	tierInst := persistTierInstallment(db, sub.ID, user.ID, 2)
	db.Model(tierInst).Update("due_date", time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC))
	team := testTeam(db, "Runners del Parque", trainer.ID)
	teamInst := persistTeamInstallment(db, team.ID, user.ID, 1)
	otherSub := persistSubscription(db, other.ID, role.ID, tier.ID, "active")
	otherInst := persistTierInstallment(db, otherSub.ID, other.ID, 1)
	return historySeed{user: user, trainer: trainer, team: team, tierInst: tierInst, teamInst: teamInst, otherInst: otherInst, trainerTier: tier, sub: sub}
}

func historyPayment(instID int64, concept, mpID, status string, seller *int64, at time.Time) paymentFixture {
	return paymentFixture{installmentID: instID, concept: concept, mpPaymentID: mpID, status: status, amount: 9999, sellerID: seller, createdAt: at}
}

func TestPaymentHistoryDao_ListHistory_BothTypes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	h := seedHistory(db)
	now := time.Now().UTC()

	// Pago de tier guardado como "order": tiene que aparecer igual.
	persistPayment(db, historyPayment(h.tierInst.ID, string(constants.PaymentConceptOrder), "mp-tier", "approved", nil, now.Add(-time.Hour)))
	persistPayment(db, historyPayment(h.teamInst.ID, string(constants.PaymentConceptTeamSubscription), "mp-team", "approved", &h.trainer.ID, now))
	persistPayment(db, historyPayment(h.tierInst.ID, "subscription", "", "pending", nil, now))
	persistPayment(db, historyPayment(h.otherInst.ID, "subscription", "mp-other", "approved", nil, now))

	rows, hasMore, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{}, 1, 20)

	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, rows, 2)
	team, tier := rows[0], rows[1]

	assert.Equal(t, "mp-team", team.MPPaymentID)
	assert.Nil(t, team.SubscriptionID)
	require.NotNil(t, team.TeamID)
	assert.Equal(t, h.team.ID, *team.TeamID)
	require.NotNil(t, team.TeamName)
	assert.Equal(t, "Runners del Parque", *team.TeamName)
	require.NotNil(t, team.TrainerID)
	assert.Equal(t, h.trainer.ID, *team.TrainerID)
	assert.Nil(t, team.TierID)

	assert.Equal(t, "mp-tier", tier.MPPaymentID)
	require.NotNil(t, tier.SubscriptionID)
	assert.Equal(t, h.sub.ID, *tier.SubscriptionID)
	require.NotNil(t, tier.TierName)
	assert.Equal(t, "Premium_entrenador", *tier.TierName)
	require.NotNil(t, tier.DueDate)
	assert.Nil(t, tier.TeamID)
	assert.Nil(t, tier.TrainerID)
}

func TestPaymentHistoryDao_ListHistory_TrainerFallsBackToTeamOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	h := seedHistory(db)

	// Sin seller_user_id (pago viejo), el entrenador sale del dueño del equipo.
	persistPayment(db, historyPayment(h.teamInst.ID, string(constants.PaymentConceptTeamSubscription), "mp-team", "approved", nil, time.Now().UTC()))

	rows, _, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{Type: "trainer_payment"}, 1, 20)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].TrainerID)
	assert.Equal(t, h.trainer.ID, *rows[0].TrainerID)
}

func TestPaymentHistoryDao_ListHistory_Filters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	h := seedHistory(db)
	now := time.Now().UTC()
	persistPayment(db, historyPayment(h.tierInst.ID, "subscription", "mp-tier-ok", "approved", nil, now))
	persistPayment(db, historyPayment(h.tierInst.ID, "subscription", "mp-tier-rej", "rejected", nil, now.Add(-time.Minute)))
	persistPayment(db, historyPayment(h.teamInst.ID, "team_subscription", "mp-team-ok", "approved", &h.trainer.ID, now))

	subs, _, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{Type: "subscription"}, 1, 20)
	require.NoError(t, err)
	assert.Len(t, subs, 2)

	teams, _, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{Type: "trainer_payment"}, 1, 20)
	require.NoError(t, err)
	require.Len(t, teams, 1)
	assert.Equal(t, "mp-team-ok", teams[0].MPPaymentID)

	rejected, _, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{Type: "subscription", Statuses: []string{"rejected", "cancelled"}}, 1, 20)
	require.NoError(t, err)
	require.Len(t, rejected, 1)
	assert.Equal(t, "mp-tier-rej", rejected[0].MPPaymentID)
}

func TestPaymentHistoryDao_ListHistory_Pagination(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentHistoryDao(db)
	h := seedHistory(db)
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		persistPayment(db, historyPayment(h.tierInst.ID, "subscription", fmt.Sprintf("mp-h%d", i), "approved", nil, base.Add(time.Duration(i)*time.Minute)))
	}

	page1, more, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{}, 1, 2)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.True(t, more)
	assert.Equal(t, "mp-h2", page1[0].MPPaymentID)

	page2, more, err := dao.ListHistory(nil, h.user.ID, HistoryPaymentFilters{}, 2, 2)
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
	_, err = dao.ListReceivedBetween(nil, 1, time.Now(), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing received payments between dates")

	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewPaymentHistoryDao(failing)
	_, err = dao.EarliestReceivedAt(nil, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error getting earliest received payment")

	failing = testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewPaymentHistoryDao(failing)
	_, _, err = dao.ListHistory(nil, 1, HistoryPaymentFilters{}, 1, 20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing payment history")
}
