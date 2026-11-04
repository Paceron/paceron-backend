package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func TestFullSeederSeedPaymentsAndMessages_RequiresPreviousPhases(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})

	err := s.seedPaymentsAndMessages()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "seedIdentity")
	assert.Zero(t, fullPaymentCount(t, db, &dbs.Payment{}))
	assert.Zero(t, fullPaymentCount(t, db, &dbs.SessionMessage{}))
	assert.Zero(t, fullPaymentCount(t, db, &dbs.PushToken{}))
}

func TestFullSeederSeedPaymentsAndMessages_SeedsRealServiceContractsIdempotently(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := seedFullPaymentsPrerequisites(t, db)

	require.NoError(t, s.seedPaymentsAndMessages())
	counts := fullPaymentCounts(t, db)
	paymentIDs := copyInt64Map(s.ids.payments)
	installmentIDs := copyInt64Map(s.ids.installments)
	messageIDs := copyInt64Map(s.ids.sessionMessages)
	pushIDs := copyInt64Map(s.ids.pushTokens)
	var subscriptionsBefore []dbs.UserRoleTierSubscription
	require.NoError(t, db.Order("id").Find(&subscriptionsBefore).Error)

	require.NoError(t, s.seedPaymentsAndMessages())
	assert.Equal(t, counts, fullPaymentCounts(t, db))
	assert.Equal(t, paymentIDs, s.ids.payments)
	assert.Equal(t, installmentIDs, s.ids.installments)
	assert.Equal(t, messageIDs, s.ids.sessionMessages)
	assert.Equal(t, pushIDs, s.ids.pushTokens)
	var subscriptionsAfter []dbs.UserRoleTierSubscription
	require.NoError(t, db.Order("id").Find(&subscriptionsAfter).Error)
	assert.Equal(t, subscriptionsBefore, subscriptionsAfter)
	var membershipsBefore []dbs.TeamUser
	require.NoError(t, db.Order("id").Find(&membershipsBefore).Error)

	independent := newFullSeeder(db, options{})
	independent.now = s.now.Add(4 * time.Hour)
	independent.ids = s.ids
	require.NoError(t, independent.seedPaymentsAndMessages())
	assert.Equal(t, counts, fullPaymentCounts(t, db))
	var membershipsAfter []dbs.TeamUser
	require.NoError(t, db.Order("id").Find(&membershipsAfter).Error)
	assert.Equal(t, membershipsBefore, membershipsAfter)

	history := services.NewPaymentHistoryService(daos.NewPaymentHistoryDao(db))
	r1History, err := history.ListHistory(nil, s.ids.users["r1"], string(constants.PaymentHistoryTypeSubscription), "", 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"pending", "approved", "approved"}, fullPaymentStatuses(r1History.Payments))
	assert.Equal(t, []int{3, 2, 1}, fullPaymentInstallmentNumbers(r1History.Payments))

	r3History, err := history.ListHistory(nil, s.ids.users["r3"], string(constants.PaymentHistoryTypeSubscription), "", 1)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"approved", "rejected"}, fullPaymentStatuses(r3History.Payments))
	var r3Pending dbs.UserRoleTierSubscription
	require.NoError(t, db.First(&r3Pending, s.ids.subscriptions["r3_premium_pending"]).Error)
	assert.Equal(t, "first_payment_pending", r3Pending.Status)

	trainerReceipts, err := history.ListReceived(nil, s.ids.users["trainer_1"], int64Ptr(s.ids.teams["team_f"]), "approved", 1)
	require.NoError(t, err)
	require.Len(t, trainerReceipts.Payments, 1)
	receipt := trainerReceipts.Payments[0]
	assert.Equal(t, "approved", receipt.Status)
	assert.Equal(t, "seed-full-payment-team-f-r10-1", receipt.MPPaymentID)
	assert.Equal(t, s.ids.teams["team_f"], receipt.Team.ID)
	assert.Equal(t, s.ids.users["r10"], receipt.Payer.ID)
	require.NotNil(t, receipt.NetAmount)
	assert.Equal(t, 4275.0, *receipt.NetAmount)
	var splitPayment dbs.Payment
	require.NoError(t, db.Where("external_reference = ?", "seed_full:payment:team-f-r10-1").Take(&splitPayment).Error)
	require.NotNil(t, splitPayment.SellerUserID)
	assert.Equal(t, s.ids.users["trainer_1"], *splitPayment.SellerUserID)
	require.NotNil(t, splitPayment.MarketplaceFee)
	assert.Equal(t, 225.0, *splitPayment.MarketplaceFee)
	require.NotNil(t, splitPayment.RawResponse)
	var rawResponse struct {
		TransactionDetails struct {
			NetReceivedAmount float64 `json:"net_received_amount"`
		} `json:"transaction_details"`
	}
	require.NoError(t, json.Unmarshal([]byte(*splitPayment.RawResponse), &rawResponse))
	assert.Equal(t, 4275.0, rawResponse.TransactionDetails.NetReceivedAmount)

	teamService := services.NewTeamService(
		daos.NewTeamDao(db), nil, nil, nil, nil, nil, nil, nil, nil, daos.NewSellerConnectionDao(db),
	)
	teamF, err := teamService.GetByID(nil, s.ids.teams["team_f"])
	require.NoError(t, err)
	assert.True(t, teamF.CanReceivePayments)
	teamB, err := teamService.GetByID(nil, s.ids.teams["team_b"])
	require.NoError(t, err)
	assert.False(t, teamB.CanReceivePayments)

	teamSubscription := services.NewTeamSubscriptionService(
		daos.NewTeamDao(db), daos.NewTeamUserDao(db), daos.NewInstallmentDao(db),
	)
	r2Team, err := teamSubscription.GetTeamSubscription(nil, s.ids.users["r2"], s.ids.teams["team_b"])
	require.NoError(t, err)
	assert.False(t, r2Team.HasDebt)
	assert.Equal(t, "first_payment_pending", r2Team.Membership.SubscriptionStatus)
	require.NotNil(t, r2Team.NextInstallment)
	assert.Equal(t, 1, r2Team.NextInstallment.InstallmentNumber)
	assert.Nil(t, r2Team.NextInstallment.NextDueDate)
	teamBInstallmentID := r2Team.NextInstallment.InstallmentID
	paymentService := services.NewPaymentService(
		daos.NewPaymentDao(db), nil, nil, daos.NewSellerConnectionDao(db), daos.NewTeamDao(db), nil,
		daos.NewPlatformSettingDao(db), daos.NewInstallmentDao(db), nil,
	)
	paymentCountBeforeCheckout := fullPaymentCount(t, db, &dbs.Payment{})
	_, err = paymentService.CreatePreference(config.GetTestContext(), payment.CreatePreferenceRequest{
		Concept: string(constants.PaymentConceptTeamSubscription), InstallmentID: &teamBInstallmentID,
		Items: []payment.PreferenceItem{{Title: "Mensualidad", Quantity: 1, UnitPrice: 5000}},
	})
	require.ErrorIs(t, err, services.ErrPaymentSellerNotConnected)
	assert.Equal(t, paymentCountBeforeCheckout, fullPaymentCount(t, db, &dbs.Payment{}))

	r10TeamF, err := teamSubscription.GetTeamSubscription(nil, s.ids.users["r10"], s.ids.teams["team_f"])
	require.NoError(t, err)
	assert.True(t, r10TeamF.HasDebt)
	assert.Equal(t, "active", r10TeamF.Membership.SubscriptionStatus)
	assert.Equal(t, 1, r10TeamF.Membership.PaidInstallments)
	require.NotNil(t, r10TeamF.NextInstallment)
	assert.Equal(t, 2, r10TeamF.NextInstallment.InstallmentNumber)
	require.NotNil(t, r10TeamF.NextInstallment.NextDueDate)
	assert.True(t, r10TeamF.NextInstallment.NextDueDate.Equal(time.Date(2026, time.August, 13, 9, 0, 0, 0, gametime.Location)))
	require.NotNil(t, r10TeamF.NextInstallment.BlockedDate)
	assert.True(t, r10TeamF.NextInstallment.BlockedDate.Equal(time.Date(2026, time.August, 20, 9, 0, 0, 0, gametime.Location)))
	assert.True(t, r10TeamF.NextInstallment.BlockedDate.Before(s.now))

	teamUserService := services.NewTeamUserService(
		daos.NewTeamUserDao(db), daos.NewTeamDao(db), nil, nil, nil, nil, nil, nil,
		daos.NewInstallmentDao(db), db,
	)
	err = teamUserService.RemoveUser(nil, s.ids.teams["team_f"], s.ids.users["r10"], s.ids.users["r10"])
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deuda pendiente")

	var installments []dbs.Installment
	require.NoError(t, db.Where("id IN ?", fullPaymentIDs(s.ids.installments)).Order("id").Find(&installments).Error)
	assert.NotEmpty(t, installments)
	for _, installment := range installments {
		assert.NotEqual(t, installment.SubscriptionID == nil, installment.TeamID == nil, "cada cuota debe tener exactamente un padre")
	}
	assertSubscriptionPaidCountersMatchLedger(t, db)
	for _, key := range []string{"team_b_r1_1", "team_b_r2_1", "team_b_r6_1", "team_b_r7_1", "team_b_r8_1", "team_b_r11_1"} {
		var row dbs.Installment
		require.NoError(t, db.First(&row, s.ids.installments[key]).Error, key)
		assert.Equal(t, "pending", row.Status)
		assert.Nil(t, row.DueDate)
		assert.Nil(t, row.BlockedDate)
		assert.Nil(t, row.InternalPaymentID)
		assert.Nil(t, row.ExternalPaymentID)
	}
	for _, userKey := range []string{"r1", "r2", "r6", "r7", "r8", "r11"} {
		var member dbs.TeamUser
		require.NoError(t, db.Where("team_id = ? AND user_id = ?", s.ids.teams["team_b"], s.ids.users[userKey]).Take(&member).Error)
		assert.Equal(t, "first_payment_pending", member.SubscriptionStatus)
		assert.Zero(t, member.PaidInstallments)
	}
	for _, key := range []string{"r1_tier_3", "r2_tier_3", "r3_tier_premium_1", "team_f_r10_2"} {
		var row dbs.Installment
		require.NoError(t, db.First(&row, s.ids.installments[key]).Error, key)
		assert.Nil(t, row.InternalPaymentID, key)
		assert.Nil(t, row.ExternalPaymentID, key)
	}
	var teamBPayments int64
	require.NoError(t, db.Table("payments p").Joins("JOIN installments i ON i.id = p.installment_id").
		Where("i.team_id = ?", s.ids.teams["team_b"]).Count(&teamBPayments).Error)
	assert.Zero(t, teamBPayments)
	var teamFPaid dbs.Installment
	require.NoError(t, db.First(&teamFPaid, s.ids.installments["team_f_r10_1"]).Error)
	assert.Equal(t, "paid", teamFPaid.Status)
	assert.Equal(t, 4500.0, teamFPaid.Amount)
	assert.Nil(t, teamFPaid.DueDate)
	assert.Nil(t, teamFPaid.BlockedDate)
	require.NotNil(t, teamFPaid.InternalPaymentID)
	assert.Equal(t, s.ids.payments["team-f-r10-1"], *teamFPaid.InternalPaymentID)
	require.NotNil(t, teamFPaid.ExternalPaymentID)
	assert.Equal(t, "seed-full-payment-team-f-r10-1", *teamFPaid.ExternalPaymentID)
	require.NoError(t, db.SavePoint("full_installment_check").Error)
	invalidArc := dbs.Installment{
		SubscriptionID: int64Ptr(s.ids.subscriptions["r1_premium_active"]), TeamID: int64Ptr(s.ids.teams["team_b"]),
		UserID: s.ids.users["r2"], InstallmentNumber: 99, Status: "pending", Amount: 5000,
		CreatedAt: s.now, UpdatedAt: s.now,
	}
	err = db.Create(&invalidArc).Error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_installments_exclusive_parent")
	require.NoError(t, db.RollbackTo("full_installment_check").Error)
	for key, id := range s.ids.payments {
		var row dbs.Payment
		require.NoError(t, db.First(&row, id).Error, key)
		require.NotEmpty(t, row.PaymentID)
		require.NotNil(t, row.ExternalRef)
		assert.Equal(t, "seed_full:payment:"+key, *row.ExternalRef)
	}
	beforeRun := fullPaymentCounts(t, db)
	err = runFullSeed(db, options{profile: fullProfile, seed: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompleto")
	assert.Equal(t, beforeRun, fullPaymentCounts(t, db))
}

func TestFullSeederPayments_RejectsConflictingExternalReferenceAndRollsBackPhase(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := seedFullPaymentsPrerequisites(t, db)
	externalReference := "seed_full:payment:r1-tier-1"
	conflict := dbs.Payment{
		PaymentID: "foreign-payment", ExternalRef: &externalReference, Concept: "order",
		Amount: 1, CurrencyID: "ARS", Status: "approved",
	}
	require.NoError(t, db.Create(&conflict).Error)
	installmentsBefore := fullPaymentCount(t, db, &dbs.Installment{})

	err := s.seedPaymentsAndMessages()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no coincide con la identidad completa")
	assert.Zero(t, fullPaymentCount(t, db, &dbs.SessionMessage{}))
	assert.Zero(t, fullPaymentCount(t, db, &dbs.PushToken{}))
	assert.Equal(t, installmentsBefore, fullPaymentCount(t, db, &dbs.Installment{}))
	assert.Zero(t, len(s.ids.payments))
}

func TestFullSeederPayments_RejectsLegacyTask8RowsWithoutAdoptingThem(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := seedFullPaymentsPrerequisites(t, db)
	teamID, userID := s.ids.teams["team_a"], s.ids.users["r10"]
	legacyInstallment := dbs.Installment{
		TeamID: &teamID, UserID: userID, InstallmentNumber: 1, Status: "paid", Amount: 4500,
		CreatedAt: s.now, UpdatedAt: s.now,
	}
	require.NoError(t, db.Create(&legacyInstallment).Error)
	externalReference := "seed_full:payment:team-a-r10-1"
	legacyPayment := dbs.Payment{
		UserID: &userID, PaymentID: "seed-full-payment-team-a-r10-1", ExternalRef: &externalReference,
		Concept: string(constants.PaymentConceptTeamSubscription), Amount: 4500, CurrencyID: "ARS",
		Status: "approved", InstallmentID: &legacyInstallment.ID,
	}
	require.NoError(t, db.Create(&legacyPayment).Error)
	paymentsBefore := fullPaymentCount(t, db, &dbs.Payment{})
	installmentsBefore := fullPaymentCount(t, db, &dbs.Installment{})

	err := s.seedPaymentsAndMessages()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "versión anterior de Task 8")
	assert.Equal(t, paymentsBefore, fullPaymentCount(t, db, &dbs.Payment{}))
	assert.Equal(t, installmentsBefore, fullPaymentCount(t, db, &dbs.Installment{}))
	assert.Zero(t, fullPaymentCount(t, db, &dbs.SessionMessage{}))
	assert.Zero(t, len(s.ids.payments))
}

func TestFullSeederPayments_SplitUsesConfiguredMarketplaceFee(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := seedFullPaymentsPrerequisites(t, db)
	require.NoError(t, db.Model(&dbs.PlatformSetting{}).
		Where("key = ?", "marketplace_fee_percent").Update("value", "7.5").Error)

	require.NoError(t, s.seedPaymentsAndMessages())
	var split dbs.Payment
	require.NoError(t, db.Where("external_reference = ?", "seed_full:payment:team-f-r10-1").Take(&split).Error)
	require.NotNil(t, split.MarketplaceFee)
	assert.Equal(t, 337.5, *split.MarketplaceFee)
	require.NotNil(t, split.RawResponse)
	var raw struct {
		TransactionDetails struct {
			NetReceivedAmount float64 `json:"net_received_amount"`
		} `json:"transaction_details"`
	}
	require.NoError(t, json.Unmarshal([]byte(*split.RawResponse), &raw))
	assert.Equal(t, 4162.5, raw.TransactionDetails.NetReceivedAmount)
}

func seedFullPaymentsPrerequisites(t *testing.T, db *gorm.DB) *fullSeeder {
	t.Helper()
	seedFullActivityTestConfig(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.October, 13, 15, 0, 0, 0, gametime.Location)
	oldNow := gametime.Now
	gametime.Now = func() time.Time { return s.now }
	t.Cleanup(func() { gametime.Now = oldNow })
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	require.NoError(t, s.seedCatalogCalendar())
	require.NoError(t, s.seedActivity())
	require.NoError(t, s.seedGamification())
	require.NoError(t, s.seedCompetitions())
	return s
}

func fullPaymentCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Count(&count).Error)
	return count
}

func fullPaymentCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	return map[string]int64{
		"payments":     fullPaymentCount(t, db, &dbs.Payment{}),
		"installments": fullPaymentCount(t, db, &dbs.Installment{}),
		"messages":     fullPaymentCount(t, db, &dbs.SessionMessage{}),
		"recipients":   fullPaymentCount(t, db, &dbs.SessionMessageRecipient{}),
		"push":         fullPaymentCount(t, db, &dbs.PushToken{}),
	}
}

func fullPaymentIDs(ids map[string]int64) []int64 {
	values := make([]int64, 0, len(ids))
	for _, id := range ids {
		values = append(values, id)
	}
	return values
}

func fullPaymentStatuses(payments []payment.HistoryPaymentItem) []string {
	statuses := make([]string, len(payments))
	for i := range payments {
		statuses[i] = payments[i].Status
	}
	return statuses
}

func fullPaymentInstallmentNumbers(payments []payment.HistoryPaymentItem) []int {
	numbers := make([]int, len(payments))
	for i := range payments {
		numbers[i] = payments[i].InstallmentNumber
	}
	return numbers
}

func assertSubscriptionPaidCountersMatchLedger(t *testing.T, db *gorm.DB) {
	t.Helper()
	var subscriptions []dbs.UserRoleTierSubscription
	require.NoError(t, db.Find(&subscriptions).Error)
	for _, subscription := range subscriptions {
		var paid int64
		require.NoError(t, db.Model(&dbs.Installment{}).
			Where("subscription_id = ? AND status = ?", subscription.ID, string(constants.InstallmentStatusPaid)).
			Count(&paid).Error)
		assert.Equal(t, int64(subscription.PaidInstallments), paid, "suscripción %d", subscription.ID)
	}
	var memberships []dbs.TeamUser
	require.NoError(t, db.Find(&memberships).Error)
	for _, membership := range memberships {
		var paid int64
		require.NoError(t, db.Model(&dbs.Installment{}).
			Where("team_id = ? AND user_id = ? AND status = ?", membership.TeamID, membership.UserID, string(constants.InstallmentStatusPaid)).
			Count(&paid).Error)
		assert.Equal(t, int64(membership.PaidInstallments), paid, "membresía %d", membership.ID)
	}
}
