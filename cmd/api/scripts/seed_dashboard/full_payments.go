package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type fullPaymentInstallmentSpec struct {
	key, subscriptionKey, teamKey, userKey string
	number                                 int
	status                                 string
	amount                                 float64
	dueDate, blockedDate                   *time.Time
}

type fullPaymentSpec struct {
	key, installmentKey, userKey  string
	status, statusDetail, concept string
	createdAt                     time.Time
	sellerKey                     string
	marketplaceFee                *float64
	netReceived                   *float64
}

func (s *fullSeeder) seedPaymentsAndMessages() error {
	if err := s.requireFullPaymentsAndMessagesPrerequisites(); err != nil {
		return err
	}
	paymentIDs := make(map[string]int64)
	installmentIDs := make(map[string]int64)
	messageIDs := make(map[string]int64)
	pushTokenIDs := make(map[string]int64)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		paymentIDs, installmentIDs, err = s.seedFullPayments(tx)
		if err != nil {
			return err
		}
		messageIDs, pushTokenIDs, err = s.seedFullMessages(tx)
		return err
	})
	if err != nil {
		return fmt.Errorf("sembrar pagos, mensajería y push del perfil full: %w", err)
	}
	mergeFullActivityIDs(s.ids.payments, paymentIDs)
	mergeFullActivityIDs(s.ids.installments, installmentIDs)
	mergeFullActivityIDs(s.ids.sessionMessages, messageIDs)
	mergeFullActivityIDs(s.ids.pushTokens, pushTokenIDs)
	return nil
}

func (s *fullSeeder) requireFullPaymentsAndMessagesPrerequisites() error {
	for _, key := range []string{"trainer_1", "trainer_2", "r1", "r2", "r3", "r4", "r5", "r10"} {
		if s.ids.users[key] <= 0 {
			return fmt.Errorf("seedPaymentsAndMessages requiere seedIdentity: falta %s", key)
		}
	}
	for _, key := range []string{"team_a", "team_b"} {
		if s.ids.teams[key] <= 0 {
			return fmt.Errorf("seedPaymentsAndMessages requiere seedTeams: falta %s", key)
		}
	}
	for _, key := range []string{"r1_premium_active", "r2_premium_debt", "r3_medium_active", "r3_premium_pending"} {
		if s.ids.subscriptions[key] <= 0 {
			return fmt.Errorf("seedPaymentsAndMessages requiere seedIdentity: falta suscripción %s", key)
		}
	}
	if s.ids.sessionInstances["today_a_main"] <= 0 || s.ids.sessionInstances["activity_a_secondary_minus7"] <= 0 {
		return fmt.Errorf("seedPaymentsAndMessages requiere seedActivity y seedCatalogCalendar: faltan instancias de calendario")
	}
	var teams []dbs.Team
	if err := s.db.Where("id IN ?", []int64{s.ids.teams["team_a"], s.ids.teams["team_b"]}).Find(&teams).Error; err != nil {
		return fmt.Errorf("validar equipos para pagos full: %w", err)
	}
	if len(teams) != 2 {
		return fmt.Errorf("seedPaymentsAndMessages requiere los equipos A y B de seedTeams")
	}
	for _, team := range teams {
		if !containsFullSeedMarker(team.Description) {
			return fmt.Errorf("seedPaymentsAndMessages rechaza el equipo %d sin marker full", team.ID)
		}
	}
	if config.MyMP.OAuthClientID == "" {
		return fmt.Errorf("seedPaymentsAndMessages requiere MP_OAUTH_CLIENT_ID")
	}
	var connection dbs.SellerConnection
	if err := s.db.Where("user_id = ? AND client_id = ?", s.ids.users["trainer_1"], config.MyMP.OAuthClientID).Take(&connection).Error; err != nil {
		return fmt.Errorf("seedPaymentsAndMessages requiere la seller connection de trainer_1 de seedTeams: %w", err)
	}
	if connection.Status != string(constants.SellerConnectionStatusAuthorized) || connection.PublicKey == "" ||
		connection.AccessToken == "" || connection.RefreshToken == "" {
		return fmt.Errorf("seller connection existente de trainer_1 no está autorizada o incompleta")
	}
	return nil
}

func (s *fullSeeder) seedFullPayments(tx *gorm.DB) (map[string]int64, map[string]int64, error) {
	installments := make(map[string]dbs.Installment)
	installmentIDs := make(map[string]int64)
	installmentSpecs, err := s.fullPaymentInstallmentSpecs(tx)
	if err != nil {
		return nil, nil, err
	}
	for _, spec := range installmentSpecs {
		row, err := s.ensureFullPaymentInstallment(tx, spec)
		if err != nil {
			return nil, nil, fmt.Errorf("cuota %s: %w", spec.key, err)
		}
		installments[spec.key] = row
		installmentIDs[spec.key] = row.ID
	}

	paymentIDs := make(map[string]int64)
	paymentSpecs, err := s.fullPaymentSpecs(tx)
	if err != nil {
		return nil, nil, err
	}
	for _, spec := range paymentSpecs {
		installment, ok := installments[spec.installmentKey]
		if !ok {
			return nil, nil, fmt.Errorf("pago %s refiere cuota desconocida %q", spec.key, spec.installmentKey)
		}
		payment, err := s.ensureFullPayment(tx, spec, installment)
		if err != nil {
			return nil, nil, fmt.Errorf("pago %s: %w", spec.key, err)
		}
		paymentIDs[spec.key] = payment.ID
		paymentID := payment.PaymentID
		if installment.InternalPaymentID != nil && *installment.InternalPaymentID != payment.ID {
			return nil, nil, fmt.Errorf("cuota %s ya apunta a otro pago interno", spec.installmentKey)
		}
		if installment.ExternalPaymentID != nil && *installment.ExternalPaymentID != paymentID {
			return nil, nil, fmt.Errorf("cuota %s ya apunta a otro payment_id", spec.installmentKey)
		}
		if err := tx.Model(&dbs.Installment{}).Where("id = ?", installment.ID).Updates(map[string]any{
			"internal_payment_id": payment.ID, "external_payment_id": paymentID, "updated_at": s.now,
		}).Error; err != nil {
			return nil, nil, fmt.Errorf("enlazar pago a cuota %s: %w", spec.installmentKey, err)
		}
		internalPaymentID := payment.ID
		installment.InternalPaymentID = &internalPaymentID
		installment.ExternalPaymentID = &paymentID
	}
	return paymentIDs, installmentIDs, nil
}

func (s *fullSeeder) fullPaymentInstallmentSpecs(tx *gorm.DB) ([]fullPaymentInstallmentSpec, error) {
	subscriptions := make(map[string]dbs.UserRoleTierSubscription, 4)
	for _, key := range []string{"r1_premium_active", "r2_premium_debt", "r3_medium_active", "r3_premium_pending"} {
		var subscription dbs.UserRoleTierSubscription
		if err := tx.First(&subscription, s.ids.subscriptions[key]).Error; err != nil {
			return nil, fmt.Errorf("leer suscripción %s: %w", key, err)
		}
		subscriptions[key] = subscription
	}
	r1, r2 := subscriptions["r1_premium_active"], subscriptions["r2_premium_debt"]
	r3Medium, r3Premium := subscriptions["r3_medium_active"], subscriptions["r3_premium_pending"]
	return []fullPaymentInstallmentSpec{
		{key: "r1_tier_1", subscriptionKey: "r1_premium_active", userKey: "r1", number: 1, status: "paid", amount: r1.InitAmount},
		{key: "r1_tier_2", subscriptionKey: "r1_premium_active", userKey: "r1", number: 2, status: "paid", amount: r1.InitAmount},
		{key: "r1_tier_3", subscriptionKey: "r1_premium_active", userKey: "r1", number: 3, status: "pending", amount: r1.InitAmount},
		{key: "r2_tier_1", subscriptionKey: "r2_premium_debt", userKey: "r2", number: 1, status: "paid", amount: r2.InitAmount},
		{key: "r2_tier_2", subscriptionKey: "r2_premium_debt", userKey: "r2", number: 2, status: "paid", amount: r2.InitAmount},
		{key: "r2_tier_3", subscriptionKey: "r2_premium_debt", userKey: "r2", number: 3, status: "pending", amount: r2.InitAmount},
		{key: "r3_tier_medium_1", subscriptionKey: "r3_medium_active", userKey: "r3", number: 1, status: "paid", amount: r3Medium.InitAmount},
		{key: "r3_tier_premium_1", subscriptionKey: "r3_premium_pending", userKey: "r3", number: 1, status: "pending", amount: r3Premium.InitAmount},
		{key: "team_b_r1_1", teamKey: "team_b", userKey: "r1", number: 1, status: "pending", amount: 5000},
		{key: "team_b_r2_1", teamKey: "team_b", userKey: "r2", number: 1, status: "pending", amount: 5000},
		{key: "team_b_r2_2", teamKey: "team_b", userKey: "r2", number: 2, status: "pending", amount: 5000,
			dueDate: timePointer(s.now.AddDate(0, 0, -40)), blockedDate: timePointer(s.now.AddDate(0, 0, -33))},
		{key: "team_a_r10_history_1", teamKey: "team_a", userKey: "r10", number: 1, status: "paid", amount: 4500},
	}, nil
}

func (s *fullSeeder) fullPaymentSpecs(tx *gorm.DB) ([]fullPaymentSpec, error) {
	subscriptions := make(map[string]dbs.UserRoleTierSubscription, 3)
	for _, key := range []string{"r1_premium_active", "r2_premium_debt", "r3_medium_active"} {
		var subscription dbs.UserRoleTierSubscription
		if err := tx.First(&subscription, s.ids.subscriptions[key]).Error; err != nil {
			return nil, fmt.Errorf("leer suscripción %s: %w", key, err)
		}
		subscriptions[key] = subscription
	}
	r1, r2, r3Medium := subscriptions["r1_premium_active"], subscriptions["r2_premium_debt"], subscriptions["r3_medium_active"]
	r1SecondDue, err := installmentDueDate(tx, s.ids.subscriptions["r1_premium_active"], 2)
	if err != nil {
		return nil, err
	}
	r2SecondDue, err := installmentDueDate(tx, s.ids.subscriptions["r2_premium_debt"], 2)
	if err != nil {
		return nil, err
	}
	fee, net := 225.0, 4275.0
	return []fullPaymentSpec{
		{key: "r1-tier-1", installmentKey: "r1_tier_1", userKey: "r1", concept: string(constants.PaymentConceptSubscription), status: "approved", createdAt: r1.StartDate.Add(time.Hour)},
		{key: "r1-tier-2", installmentKey: "r1_tier_2", userKey: "r1", concept: string(constants.PaymentConceptSubscription), status: "approved", createdAt: r1SecondDue.Add(-time.Hour)},
		{key: "r1-tier-3", installmentKey: "r1_tier_3", userKey: "r1", concept: string(constants.PaymentConceptSubscription), status: "pending", statusDetail: "pending_waiting_transfer", createdAt: s.now},
		{key: "r2-tier-1", installmentKey: "r2_tier_1", userKey: "r2", concept: string(constants.PaymentConceptSubscription), status: "approved", createdAt: r2.StartDate.Add(time.Hour)},
		{key: "r2-tier-2", installmentKey: "r2_tier_2", userKey: "r2", concept: string(constants.PaymentConceptSubscription), status: "approved", createdAt: r2SecondDue.Add(-time.Hour)},
		{key: "r2-tier-3", installmentKey: "r2_tier_3", userKey: "r2", concept: string(constants.PaymentConceptSubscription), status: "rejected", statusDetail: "cc_rejected_insufficient_amount", createdAt: s.now.AddDate(0, 0, -31)},
		{key: "r3-tier-medium-1", installmentKey: "r3_tier_medium_1", userKey: "r3", concept: string(constants.PaymentConceptSubscription), status: "approved", createdAt: r3Medium.StartDate.Add(time.Hour)},
		{key: "r3-tier-premium-1", installmentKey: "r3_tier_premium_1", userKey: "r3", concept: string(constants.PaymentConceptSubscription), status: "rejected", statusDetail: "cc_rejected_other_reason", createdAt: s.now},
		{key: "team-a-r10-1", installmentKey: "team_a_r10_history_1", userKey: "r10", concept: string(constants.PaymentConceptTeamSubscription), status: "approved", createdAt: s.now.AddDate(0, -2, 0),
			sellerKey: "trainer_1", marketplaceFee: &fee, netReceived: &net},
		{key: "team-b-r1-1", installmentKey: "team_b_r1_1", userKey: "r1", concept: string(constants.PaymentConceptTeamSubscription), status: "rejected", statusDetail: "cc_rejected_insufficient_amount", createdAt: s.now,
			sellerKey: "trainer_2"},
		{key: "team-b-r2-1", installmentKey: "team_b_r2_1", userKey: "r2", concept: string(constants.PaymentConceptTeamSubscription), status: "rejected", statusDetail: "cc_rejected_other_reason", createdAt: s.now,
			sellerKey: "trainer_2"},
		{key: "team-b-r2-2", installmentKey: "team_b_r2_2", userKey: "r2", concept: string(constants.PaymentConceptTeamSubscription), status: "rejected", statusDetail: "cc_rejected_insufficient_amount", createdAt: s.now.AddDate(0, 0, -39),
			sellerKey: "trainer_2"},
	}, nil
}

func (s *fullSeeder) ensureFullPaymentInstallment(tx *gorm.DB, spec fullPaymentInstallmentSpec) (dbs.Installment, error) {
	userID := s.ids.users[spec.userKey]
	if spec.subscriptionKey != "" {
		id := s.ids.subscriptions[spec.subscriptionKey]
		var rows []dbs.Installment
		if err := tx.Where("subscription_id = ? AND installment_number = ?", id, spec.number).Find(&rows).Error; err != nil {
			return dbs.Installment{}, err
		}
		if len(rows) != 1 {
			return dbs.Installment{}, fmt.Errorf("ledger previo tiene %d cuotas; se esperaba exactamente una", len(rows))
		}
		row := rows[0]
		if row.TeamID != nil || row.UserID != userID || row.Status != spec.status || row.Amount != spec.amount {
			return dbs.Installment{}, fmt.Errorf("la cuota existente no coincide con el ledger de T2")
		}
		return row, nil
	}
	teamIDValue := s.ids.teams[spec.teamKey]
	teamID := &teamIDValue
	var rows []dbs.Installment
	if err := tx.Where("team_id = ? AND user_id = ? AND installment_number = ?", teamIDValue, userID, spec.number).Find(&rows).Error; err != nil {
		return dbs.Installment{}, err
	}
	if len(rows) > 1 {
		return dbs.Installment{}, fmt.Errorf("hay cuotas duplicadas para la clave equipo/usuario/número")
	}
	expected := dbs.Installment{
		TeamID: teamID, UserID: userID, InstallmentNumber: spec.number, Status: spec.status,
		Amount: spec.amount, DueDate: spec.dueDate, BlockedDate: spec.blockedDate,
		CreatedAt: s.now, UpdatedAt: s.now,
	}
	if len(rows) == 1 {
		row := rows[0]
		if row.SubscriptionID != nil || row.Status != expected.Status || row.Amount != expected.Amount ||
			!sameTimePointer(row.DueDate, expected.DueDate) || !sameTimePointer(row.BlockedDate, expected.BlockedDate) {
			return dbs.Installment{}, fmt.Errorf("cuota existente en la clave equipo/usuario/número distinta al perfil full")
		}
		return row, nil
	}
	if err := tx.Create(&expected).Error; err != nil {
		return dbs.Installment{}, err
	}
	return expected, nil
}

func (s *fullSeeder) ensureFullPayment(tx *gorm.DB, spec fullPaymentSpec, installment dbs.Installment) (*dbs.Payment, error) {
	userID := s.ids.users[spec.userKey]
	installmentID := installment.ID
	paymentID := "seed-full-payment-" + spec.key
	externalReference := "seed_full:payment:" + spec.key
	preferenceID := "seed-full-preference-" + spec.key
	method := "master"
	payerEmail := ""
	var user dbs.User
	if err := tx.Select("email").First(&user, userID).Error; err != nil {
		return nil, err
	}
	payerEmail = user.Email
	expected := dbs.Payment{
		UserID: &userID, PreferenceID: preferenceID, PaymentID: paymentID, ExternalRef: &externalReference,
		Concept: spec.concept, Description: fullSeedMarker + " pago sintético " + spec.key,
		Amount: installment.Amount, CurrencyID: "ARS", Status: spec.status, StatusDetail: spec.statusDetail,
		PaymentMethodID: method, Installments: 1, PayerEmail: payerEmail, MarketplaceFee: spec.marketplaceFee,
		InstallmentID: &installmentID, CreatedAt: spec.createdAt, UpdatedAt: s.now,
	}
	if spec.sellerKey != "" {
		sellerID := s.ids.users[spec.sellerKey]
		expected.SellerUserID = &sellerID
	}
	if spec.netReceived != nil {
		raw := fmt.Sprintf(`{"id":"%s","status":"%s","transaction_details":{"net_received_amount":%.2f}}`, paymentID, spec.status, *spec.netReceived)
		expected.RawResponse = &raw
	} else {
		raw := fmt.Sprintf(`{"id":"%s","status":"%s"}`, paymentID, spec.status)
		expected.RawResponse = &raw
	}
	var rows []dbs.Payment
	if err := tx.Where("external_reference = ?", externalReference).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("external_reference full duplicado")
	}
	var byPaymentID []dbs.Payment
	if err := tx.Where("payment_id = ?", paymentID).Find(&byPaymentID).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if len(byPaymentID) != 0 {
			return nil, fmt.Errorf("payment_id sintético ya pertenece a otra fila")
		}
		if err := tx.Create(&expected).Error; err != nil {
			return nil, err
		}
		return &expected, nil
	}
	row := rows[0]
	if len(byPaymentID) != 1 || byPaymentID[0].ID != row.ID || !sameFullPaymentIdentity(row, expected) {
		return nil, fmt.Errorf("la fila de external_reference no coincide con la identidad completa full")
	}
	if err := tx.Model(&dbs.Payment{}).Where("id = ?", row.ID).Updates(map[string]any{
		"created_at": expected.CreatedAt, "updated_at": s.now,
	}).Error; err != nil {
		return nil, err
	}
	row.CreatedAt, row.UpdatedAt = expected.CreatedAt, s.now
	return &row, nil
}

func sameFullPaymentIdentity(actual, expected dbs.Payment) bool {
	return actual.UserID != nil && *actual.UserID == *expected.UserID &&
		actual.PreferenceID == expected.PreferenceID && actual.PaymentID == expected.PaymentID &&
		actual.ExternalRef != nil && *actual.ExternalRef == *expected.ExternalRef &&
		actual.Concept == expected.Concept && actual.Description == expected.Description &&
		actual.Amount == expected.Amount && actual.CurrencyID == expected.CurrencyID &&
		actual.Status == expected.Status && actual.StatusDetail == expected.StatusDetail &&
		actual.PaymentMethodID == expected.PaymentMethodID && actual.Installments == expected.Installments &&
		actual.PayerEmail == expected.PayerEmail && sameFloat64Pointer(actual.MarketplaceFee, expected.MarketplaceFee) &&
		sameInt64Pointer(actual.SellerUserID, expected.SellerUserID) &&
		sameInt64Pointer(actual.InstallmentID, expected.InstallmentID) &&
		sameFullPaymentRawResponse(actual.RawResponse, expected.RawResponse)
}

func sameFullPaymentRawResponse(actual, expected *string) bool {
	if actual == nil || expected == nil {
		return actual == nil && expected == nil
	}
	var actualJSON, expectedJSON any
	if json.Unmarshal([]byte(*actual), &actualJSON) != nil || json.Unmarshal([]byte(*expected), &expectedJSON) != nil {
		return false
	}
	return reflect.DeepEqual(actualJSON, expectedJSON)
}

func installmentDueDate(tx *gorm.DB, subscriptionID int64, number int) (time.Time, error) {
	var row dbs.Installment
	if err := tx.Where("subscription_id = ? AND installment_number = ?", subscriptionID, number).Take(&row).Error; err != nil {
		return time.Time{}, err
	}
	if row.DueDate == nil {
		return time.Time{}, fmt.Errorf("suscripción %d cuota %d sin fecha de vencimiento", subscriptionID, number)
	}
	return *row.DueDate, nil
}
