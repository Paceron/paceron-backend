package services

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mpsdk "github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/restclients/mercadopagoclient"
	"simple-arq-golang/cmd/api/testutils"
)

func TestMain(m *testing.M) {
	config.MyMP = config.MercadoPago{
		AccessToken:   "test-access-token",
		PublicKey:     "test-public-key",
		WebhookSecret: "test-secret",
		WebhookURL:    "https://test.com/webhook",
		CurrencyID:    "ARS",
	}
	os.Exit(m.Run())
}

type mockPaymentDao struct {
	mock.Mock
}

func (m *mockPaymentDao) Create(ctx *gin.Context, p *dbs.Payment) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockPaymentDao) UpdateStatus(ctx *gin.Context, paymentID int64, status, statusDetail string) error {
	args := m.Called(ctx, paymentID, status, statusDetail)
	return args.Error(0)
}

func (m *mockPaymentDao) UpdateRawResponse(ctx *gin.Context, paymentID int64, rawResponse string) error {
	args := m.Called(ctx, paymentID, rawResponse)
	return args.Error(0)
}

func (m *mockPaymentDao) UpdateExternalRef(ctx *gin.Context, paymentID int64, externalRef string) error {
	args := m.Called(ctx, paymentID, externalRef)
	return args.Error(0)
}

func (m *mockPaymentDao) FindByID(ctx *gin.Context, id int64) (*dbs.Payment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dbs.Payment), args.Error(1)
}

func (m *mockPaymentDao) FindByPaymentID(ctx *gin.Context, mpPaymentID string) (*dbs.Payment, error) {
	args := m.Called(ctx, mpPaymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dbs.Payment), args.Error(1)
}

func (m *mockPaymentDao) FindByExternalReference(ctx *gin.Context, externalRef string) (*dbs.Payment, error) {
	args := m.Called(ctx, externalRef)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dbs.Payment), args.Error(1)
}

type mockMercadoPagoClient struct {
	mock.Mock
}

func (m *mockMercadoPagoClient) CreatePreference(ctx context.Context, accessToken string, items []mercadopagoclient.PreferenceItem, externalRef, notificationURL, marketplaceFee string, currencyID string) (string, error) {
	args := m.Called(ctx, accessToken, items, externalRef, notificationURL, marketplaceFee, currencyID)
	return args.String(0), args.Error(1)
}

func (m *mockMercadoPagoClient) CreatePayment(ctx context.Context, accessToken string, req mercadopagoclient.CreatePaymentRequest) (*mercadopagoclient.PaymentResult, error) {
	args := m.Called(ctx, accessToken, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mercadopagoclient.PaymentResult), args.Error(1)
}

func (m *mockMercadoPagoClient) GetPayment(ctx context.Context, accessToken string, paymentID int) (*mpsdk.Response, error) {
	args := m.Called(ctx, accessToken, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mpsdk.Response), args.Error(1)
}

func (m *mockMercadoPagoClient) ValidateWebhookSignature(xSignature, xRequestID, dataID, secret string) error {
	args := m.Called(xSignature, xRequestID, dataID, secret)
	return args.Error(0)
}

func (m *mockMercadoPagoClient) GenerateCardToken(ctx context.Context, accessToken string, cardNumber, expirationMonth, expirationYear, cvv, cardholderName, identificationType, identificationNumber, siteID string) (string, error) {
	args := m.Called(ctx, accessToken, cardNumber, expirationMonth, expirationYear, cvv, cardholderName, identificationType, identificationNumber, siteID)
	return args.String(0), args.Error(1)
}

func (m *mockMercadoPagoClient) GetAuthURL(redirectURI string, state string) string {
	args := m.Called(redirectURI, state)
	return args.String(0)
}

func (m *mockMercadoPagoClient) ExchangeCodeForToken(ctx context.Context, clientID, clientSecret, redirectURI, code string) (*mercadopagoclient.OAuthTokenResponse, error) {
	args := m.Called(ctx, clientID, clientSecret, redirectURI, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mercadopagoclient.OAuthTokenResponse), args.Error(1)
}

func (m *mockMercadoPagoClient) GetUserInfo(ctx context.Context, accessToken string) (*mercadopagoclient.UserInfoResponse, error) {
	args := m.Called(ctx, accessToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mercadopagoclient.UserInfoResponse), args.Error(1)
}

func TestNewPaymentService(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, svc)
}

func TestPaymentService_ImplementsInterface(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)
	var iface PaymentServiceInterface = svc
	_ = iface
}

func TestCreatePreference_Success(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	req := payment.CreatePreferenceRequest{
		Concept:     "order",
		Description: "Test preference",
		Items: []payment.PreferenceItem{
			{Title: "Item 1", Quantity: 1, UnitPrice: 100},
		},
	}

	dao.On("Create", ctx, mock.AnythingOfType("*dbs.Payment")).Return(nil)
	dao.On("UpdateExternalRef", ctx, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil)
	dao.On("UpdateRawResponse", ctx, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil)
	client.On("CreatePreference", ctx, "test-access-token", mock.AnythingOfType("[]mercadopagoclient.PreferenceItem"), mock.AnythingOfType("string"), "https://test.com/webhook", "0.00", "ARS").Return("pref-123", nil)

	resp, err := svc.CreatePreference(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "pref-123", resp.PreferenceID)
	assert.Equal(t, "test-public-key", resp.PublicKey)
	dao.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestCreatePreference_DaoError(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	req := payment.CreatePreferenceRequest{
		Concept:     "order",
		Description: "Test",
		Items: []payment.PreferenceItem{
			{Title: "Item", Quantity: 1, UnitPrice: 50},
		},
	}

	dao.On("Create", ctx, mock.Anything).Return(fmt.Errorf("db error"))

	resp, err := svc.CreatePreference(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error creating payment record")
}

func TestCreatePreference_MPClientError(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	req := payment.CreatePreferenceRequest{
		Concept:     "order",
		Description: "Test",
		Items: []payment.PreferenceItem{
			{Title: "Item", Quantity: 1, UnitPrice: 75},
		},
	}

	dao.On("Create", ctx, mock.Anything).Return(nil)
	dao.On("UpdateExternalRef", ctx, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil)
	dao.On("UpdateRawResponse", ctx, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil)
	client.On("CreatePreference", ctx, "test-access-token", mock.AnythingOfType("[]mercadopagoclient.PreferenceItem"), mock.AnythingOfType("string"), "https://test.com/webhook", "0.00", "ARS").Return("", fmt.Errorf("mp error"))

	resp, err := svc.CreatePreference(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error creating MP preference")
}

func TestProcessPayment_Success(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	req := payment.ProcessPaymentRequest{
		Token:             "tok_visa",
		TransactionAmount: 1500,
		PaymentMethodID:   "visa",
		Installments:      1,
		PayerEmail:        "payer@example.com",
		PreferenceID:      "pref-999",
	}

	dao.On("FindByExternalReference", ctx, "pref-999").Return(nil, nil)
	dao.On("Create", ctx, mock.Anything).Return(nil)
	client.On("CreatePayment", ctx, "test-access-token", mock.AnythingOfType("mercadopagoclient.CreatePaymentRequest")).Return(&mercadopagoclient.PaymentResult{
		ID:           12345,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)
	dao.On("UpdateStatus", ctx, mock.AnythingOfType("int64"), "approved", "accredited").Return(nil)
	dao.On("UpdateRawResponse", ctx, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil)

	resp, err := svc.ProcessPayment(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "approved", resp.Status)
	assert.Equal(t, "accredited", resp.StatusDetail)
	dao.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestProcessPayment_CreatePaymentError(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	req := payment.ProcessPaymentRequest{
		Token:             "tok_visa",
		TransactionAmount: 1000,
		PaymentMethodID:   "visa",
		Installments:      1,
		PayerEmail:        "payer@example.com",
	}

	dao.On("FindByExternalReference", ctx, "").Return(nil, nil)
	dao.On("Create", ctx, mock.Anything).Return(nil)
	client.On("CreatePayment", ctx, "test-access-token", mock.AnythingOfType("mercadopagoclient.CreatePaymentRequest")).Return(nil, fmt.Errorf("mp create error"))

	resp, err := svc.ProcessPayment(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error creating MP payment")
}

func TestGetPaymentStatus_Success(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	paymentRecord := &dbs.Payment{
		ID:           1,
		PaymentID:    "12345",
		Status:       "pending",
		StatusDetail: "",
		Concept:      "order",
		Description:  "Test",
		Amount:       500,
		CurrencyID:   "ARS",
		PayerEmail:   "payer@example.com",
	}

	dao.On("FindByID", ctx, int64(1)).Return(paymentRecord, nil)
	client.On("GetPayment", ctx, "test-access-token", 12345).Return(&mpsdk.Response{
		ID:           12345,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)
	dao.On("UpdateStatus", ctx, int64(1), "approved", "accredited").Return(nil)

	resp, err := svc.GetPaymentStatus(ctx, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "approved", resp.Status)
}

func TestGetPaymentStatus_NotFound(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()

	dao.On("FindByID", ctx, int64(999)).Return(nil, nil)

	resp, err := svc.GetPaymentStatus(ctx, 999)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "payment not found")
}

func TestGetPaymentStatusFromMP_Success(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()

	expMonth := 11
	expYear := 2030
	client.On("GetPayment", ctx, "test-access-token", 1351279383).Return(&mpsdk.Response{
		ID:                1351279383,
		Status:            "approved",
		StatusDetail:      "accredited",
		OperationType:     "regular_payment",
		Description:       "Pago con tarjeta",
		ExternalReference: "ext-ref-1",
		TransactionAmount: 1000,
		CurrencyID:        "ARS",
		PaymentMethodID:   "master",
		PaymentTypeID:     "credit_card",
		Installments:      1,
		IssuerID:          "25",
		LiveMode:          false,
		Captured:          true,
		Payer: mpsdk.PayerResponse{
			ID:    "123",
			Email: "buyer@test.com",
		},
		Card: mpsdk.CardResponse{
			ID:              "card-1",
			LastFourDigits:  "0604",
			FirstSixDigits:  "503175",
			ExpirationMonth: mpsdk.MaskedInt{Value: &expMonth},
			ExpirationYear:  mpsdk.MaskedInt{Value: &expYear},
			Cardholder: mpsdk.CardholderResponse{
				Name: "APRO Test User",
			},
		},
		FeeDetails: []mpsdk.FeeDetailResponse{
			{Type: "commission", FeePayer: "collector", Amount: 3.5},
		},
	}, nil)

	resp, err := svc.GetPaymentStatusFromMP(ctx, "1351279383")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1351279383, resp.ID)
	assert.Equal(t, "approved", resp.Status)
	assert.Equal(t, "accredited", resp.StatusDetail)
	assert.Equal(t, float64(1000), resp.TransactionAmount)
	assert.Equal(t, "ARS", resp.CurrencyID)
	assert.Equal(t, "master", resp.PaymentMethodID)
	assert.Equal(t, "buyer@test.com", resp.Payer.Email)
	assert.Equal(t, "0604", resp.Card.LastFourDigits)
	assert.Equal(t, "11", resp.Card.ExpirationMonth)
	assert.Equal(t, "2030", resp.Card.ExpirationYear)
	assert.Equal(t, "APRO Test User", resp.Card.CardholderName)
	assert.Len(t, resp.FeeDetails, 1)
	assert.Equal(t, float64(3.5), resp.FeeDetails[0].Amount)
}

func TestGetPaymentStatusFromMP_InvalidID(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()

	resp, err := svc.GetPaymentStatusFromMP(ctx, "not-a-number")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid payment ID")
}

func TestHandleWebhook_IgnoresNonPaymentType(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	notification := payment.WebhookNotification{
		Type: "merchant_order",
		Data: struct {
			ID string `json:"id"`
		}{ID: "123"},
	}

	err := svc.HandleWebhook(ctx, notification)

	assert.NoError(t, err)
	dao.AssertNotCalled(t, "FindByPaymentID")
}

func TestHandleWebhook_MissingDataID(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	notification := payment.WebhookNotification{
		Type: "payment",
		Data: struct {
			ID string `json:"id"`
		}{ID: ""},
	}

	err := svc.HandleWebhook(ctx, notification)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing data.id")
}

func TestHandleWebhook_Success(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	notification := payment.WebhookNotification{
		Type: "payment",
		Data: struct {
			ID string `json:"id"`
		}{ID: "12345"},
	}

	paymentRecord := &dbs.Payment{
		ID:        1,
		PaymentID: "12345",
		Status:    "pending",
	}

	client.On("GetPayment", ctx, "test-access-token", 12345).Return(&mpsdk.Response{
		ID:           12345,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)
	dao.On("FindByPaymentID", ctx, "12345").Return(paymentRecord, nil)
	dao.On("UpdateStatus", ctx, int64(1), "approved", "accredited").Return(nil)
	dao.On("UpdateRawResponse", ctx, int64(1), mock.AnythingOfType("string")).Return(nil)

	err := svc.HandleWebhook(ctx, notification)

	assert.NoError(t, err)
	dao.AssertExpectations(t)
}

func TestHandleWebhook_PaymentNotFoundLocally(t *testing.T) {
	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, nil, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	notification := payment.WebhookNotification{
		Type: "payment",
		Data: struct {
			ID string `json:"id"`
		}{ID: "99999"},
	}

	client.On("GetPayment", ctx, "test-access-token", 99999).Return(&mpsdk.Response{
		ID:     99999,
		Status: "approved",
	}, nil)
	dao.On("FindByPaymentID", ctx, "99999").Return(nil, nil)
	dao.On("FindByExternalReference", ctx, "99999").Return(nil, nil)

	err := svc.HandleWebhook(ctx, notification)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment not found locally")
}

// TestHandleWebhook_ApprovedInstallment_RequiresPostgres cubre el flujo D6/D7 contra
// Postgres real (se skipea si no hay TEST_DB_HOST, ver docs/TESTING.md): un pago
// aprobado de la cuota #1 marca la cuota, activa la suscripción, actualiza el tier
// del rol (base -> pagado) y genera la cuota #2 con su due/blocked date. La segunda
// notificación del webhook (duplicado) no repite el flujo (idempotencia por
// MarkPaidConditional).
func TestHandleWebhook_ApprovedInstallment_RequiresPostgres(t *testing.T) {
	db := testutils.SetupTestDB(t)

	user := &dbs.User{
		Name:      "Test",
		Surname:   "User",
		Email:     "webhook-ins@test.com",
		DNI:       "30000001",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  "hashed",
	}
	require.NoError(t, db.Create(user).Error)

	role := &dbs.Role{Name: "entrenador"}
	require.NoError(t, db.Create(role).Error)

	base := &dbs.Tier{Name: "base", RoleID: role.ID, RoleName: role.Name, Hierarchy: 1, PaymentRequired: false}
	require.NoError(t, db.Create(base).Error)
	premium := &dbs.Tier{Name: "premium", RoleID: role.ID, RoleName: role.Name, Hierarchy: 2, PaymentRequired: true, TierAmount: 1500}
	require.NoError(t, db.Create(premium).Error)

	urDao := daos.NewUserRoleDao(db)
	require.NoError(t, urDao.Create(nil, &dbs.UserRole{UserID: user.ID, RoleID: role.ID, TierID: base.ID,
		AssignmentDate: time.Now(), Status: "active"}))

	sub := &dbs.UserRoleTierSubscription{
		UserID:     user.ID,
		RoleID:     role.ID,
		TierID:     premium.ID,
		Status:     string(constants.SubscriptionStatusFirstPaymentPending),
		InitAmount: premium.TierAmount,
		StartDate:  time.Now(),
	}
	subDao := daos.NewTierSubscriptionDao(db)
	require.NoError(t, subDao.Create(nil, sub))

	insDao := daos.NewInstallmentDao(db)
	ins := &dbs.Installment{
		SubscriptionID:    &sub.ID,
		UserID:            user.ID,
		InstallmentNumber: 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            premium.TierAmount,
	}
	require.NoError(t, insDao.Create(nil, ins))

	dao := new(mockPaymentDao)
	client := new(mockMercadoPagoClient)
	svc := NewPaymentService(dao, client, db, nil, nil, nil, nil, nil, nil)

	ctx := config.GetTestContext()
	notification := payment.WebhookNotification{
		Type: "payment",
		Data: struct {
			ID string `json:"id"`
		}{ID: "50001"},
	}

	paymentRecord := &dbs.Payment{
		ID:            501,
		PaymentID:     "50001",
		Status:        "pending",
		InstallmentID: &ins.ID,
	}
	client.On("GetPayment", ctx, "test-access-token", 50001).Return(&mpsdk.Response{
		ID:           50001,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)
	dao.On("FindByPaymentID", ctx, "50001").Return(paymentRecord, nil)
	dao.On("UpdateStatus", ctx, int64(501), "approved", "accredited").Return(nil)
	dao.On("UpdateRawResponse", ctx, int64(501), mock.AnythingOfType("string")).Return(nil)

	err := svc.HandleWebhook(ctx, notification)
	assert.NoError(t, err)

	paid, err := insDao.FindByID(nil, ins.ID)
	require.NoError(t, err)
	require.NotNil(t, paid)
	assert.Equal(t, string(constants.InstallmentStatusPaid), paid.Status)
	assert.NotNil(t, paid.InternalPaymentID)
	assert.Equal(t, int64(501), *paid.InternalPaymentID)
	assert.NotNil(t, paid.ExternalPaymentID)
	assert.Equal(t, "50001", *paid.ExternalPaymentID)

	subActive, err := subDao.FindByID(nil, sub.ID)
	require.NoError(t, err)
	require.NotNil(t, subActive)
	assert.Equal(t, string(constants.SubscriptionStatusActive), subActive.Status)
	assert.Equal(t, 1, subActive.PaidInstallments)

	ur, err := urDao.FindByUserAndRole(nil, user.ID, role.ID)
	require.NoError(t, err)
	require.NotNil(t, ur)
	assert.Equal(t, premium.ID, ur.TierID)

	next, err := insDao.FindNext(nil, sub.ID)
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Equal(t, 2, next.InstallmentNumber)
	assert.Equal(t, string(constants.InstallmentStatusPending), next.Status)
	assert.Equal(t, premium.TierAmount, next.Amount)
	require.NotNil(t, next.DueDate)
	require.NotNil(t, next.BlockedDate)
	expectedDue := subActive.StartDate.AddDate(0, 1, 0)
	assert.True(t, next.DueDate.Equal(expectedDue))
	assert.True(t, next.BlockedDate.Equal(expectedDue.AddDate(0, 0, 7)))

	client.On("GetPayment", ctx, "test-access-token", 50001).Return(&mpsdk.Response{
		ID:           50001,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)
	dao.On("FindByPaymentID", ctx, "50001").Return(paymentRecord, nil)
	dao.On("UpdateStatus", ctx, int64(501), "approved", "accredited").Return(nil)
	dao.On("UpdateRawResponse", ctx, int64(501), mock.AnythingOfType("string")).Return(nil)

	err = svc.HandleWebhook(ctx, notification)
	assert.NoError(t, err)

	next2, err := insDao.FindNext(nil, sub.ID)
	require.NoError(t, err)
	require.NotNil(t, next2)
	assert.Equal(t, 2, next2.InstallmentNumber)

	subAgain, err := subDao.FindByID(nil, sub.ID)
	require.NoError(t, err)
	require.NotNil(t, subAgain)
	assert.Equal(t, 1, subAgain.PaidInstallments)
}
