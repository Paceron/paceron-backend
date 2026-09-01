package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"simple-arq-golang/cmd/api/domains/payment"
)

type mockPaymentService struct {
	mock.Mock
}

func (m *mockPaymentService) CreatePreference(c *gin.Context, req payment.CreatePreferenceRequest) (*payment.CreatePreferenceResponse, error) {
	args := m.Called(c, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.CreatePreferenceResponse), args.Error(1)
}

func (m *mockPaymentService) ProcessPayment(c *gin.Context, req payment.ProcessPaymentRequest) (*payment.PaymentResponse, error) {
	args := m.Called(c, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.PaymentResponse), args.Error(1)
}

func (m *mockPaymentService) GetPaymentStatus(c *gin.Context, paymentID int64) (*payment.PaymentResponse, error) {
	args := m.Called(c, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.PaymentResponse), args.Error(1)
}

func (m *mockPaymentService) GetPaymentStatusFromMP(c *gin.Context, mpPaymentID string) (*payment.MPPaymentStatusResponse, error) {
	args := m.Called(c, mpPaymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.MPPaymentStatusResponse), args.Error(1)
}

func (m *mockPaymentService) HandleWebhook(c *gin.Context, notification payment.WebhookNotification) error {
	args := m.Called(c, notification)
	return args.Error(0)
}

func (m *mockPaymentService) GenerateTestCardToken(c *gin.Context, req payment.TestCardTokenRequest) (*payment.TestCardTokenResponse, error) {
	args := m.Called(c, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.TestCardTokenResponse), args.Error(1)
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestCreatePreference_Success(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/preference", controller.CreatePreference)

	reqBody := payment.CreatePreferenceRequest{
		Concept:     "order",
		Description: "Test preference",
		Items: []payment.PreferenceItem{
			{Title: "Item 1", Quantity: 1, UnitPrice: 100},
		},
	}
	body, _ := json.Marshal(reqBody)

	service.On("CreatePreference", mock.AnythingOfType("*gin.Context"), reqBody).Return(&payment.CreatePreferenceResponse{
		PreferenceID: "pref-123",
		PublicKey:    "test-public-key",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/preference", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp payment.CreatePreferenceResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "pref-123", resp.PreferenceID)
	assert.Equal(t, "test-public-key", resp.PublicKey)
	service.AssertExpectations(t)
}

func TestCreatePreference_InvalidBody(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/preference", controller.CreatePreference)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/preference", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "CreatePreference")
}

func TestCreatePreference_MissingItems(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/preference", controller.CreatePreference)

	reqBody := payment.CreatePreferenceRequest{
		Concept:     "order",
		Description: "Test",
		Items:       []payment.PreferenceItem{},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/preference", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "CreatePreference")
}

func TestCreatePreference_MissingDescription(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/preference", controller.CreatePreference)

	reqBody := payment.CreatePreferenceRequest{
		Concept: "order",
		Items: []payment.PreferenceItem{
			{Title: "Item", Quantity: 1, UnitPrice: 100},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/preference", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "CreatePreference")
}

func TestProcessPayment_Success(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments", controller.ProcessPayment)

	reqBody := payment.ProcessPaymentRequest{
		Token:             "tok_visa",
		TransactionAmount: 1500,
		PaymentMethodID:   "visa",
		Installments:      1,
		PayerEmail:        "payer@example.com",
		PreferenceID:      "pref-123",
	}
	body, _ := json.Marshal(reqBody)

	service.On("ProcessPayment", mock.AnythingOfType("*gin.Context"), reqBody).Return(&payment.PaymentResponse{
		ID:           1,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp payment.PaymentResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "approved", resp.Status)
	service.AssertExpectations(t)
}

func TestProcessPayment_MissingToken(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments", controller.ProcessPayment)

	reqBody := payment.ProcessPaymentRequest{
		TransactionAmount: 1000,
		PaymentMethodID:   "visa",
		Installments:      1,
		PayerEmail:        "payer@example.com",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "ProcessPayment")
}

func TestGetPaymentStatus_Success(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.GET("/api/v1/payments/:id", controller.GetPaymentStatus)

	service.On("GetPaymentStatus", mock.AnythingOfType("*gin.Context"), int64(1)).Return(&payment.PaymentResponse{
		ID:     1,
		Status: "approved",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/payments/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp payment.PaymentResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "approved", resp.Status)
	service.AssertExpectations(t)
}

func TestGetPaymentStatus_InvalidID(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.GET("/api/v1/payments/:id", controller.GetPaymentStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/payments/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "GetPaymentStatus")
}

func TestGetPaymentStatus_NotFound(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.GET("/api/v1/payments/:id", controller.GetPaymentStatus)

	service.On("GetPaymentStatus", mock.AnythingOfType("*gin.Context"), int64(999)).Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/payments/999", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	service.AssertExpectations(t)
}

func TestGetPaymentStatusFromMP_Success(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.GET("/api/v1/payments/mp/:id", controller.GetPaymentStatusFromMP)

	service.On("GetPaymentStatusFromMP", mock.AnythingOfType("*gin.Context"), "1351279383").Return(&payment.MPPaymentStatusResponse{
		ID:           1351279383,
		Status:       "approved",
		StatusDetail: "accredited",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/payments/mp/1351279383", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"approved"`)
	service.AssertExpectations(t)
}

func TestGetPaymentStatusFromMP_Error(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.GET("/api/v1/payments/mp/:id", controller.GetPaymentStatusFromMP)

	service.On("GetPaymentStatusFromMP", mock.AnythingOfType("*gin.Context"), "123").Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/payments/mp/123", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	service.AssertExpectations(t)
}

func TestHandleWebhook_Success(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/webhook", controller.HandleWebhook)

	reqBody := payment.WebhookNotification{
		Type: "payment",
		Data: struct {
			ID string `json:"id"`
		}{ID: "12345"},
	}
	body, _ := json.Marshal(reqBody)

	service.On("HandleWebhook", mock.AnythingOfType("*gin.Context"), reqBody).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "test-signature")
	req.Header.Set("X-Request-Id", "test-request-id")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	service.AssertExpectations(t)
}

func TestHandleWebhook_InvalidBody(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/webhook", controller.HandleWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "HandleWebhook")
}

func TestHandleWebhook_MissingSignature(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/webhook", controller.HandleWebhook)

	reqBody := payment.WebhookNotification{
		Type: "payment",
		Data: struct {
			ID string `json:"id"`
		}{ID: "12345"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	service.AssertNotCalled(t, "HandleWebhook")
}

func TestGenerateTestCardToken_Success(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/test-card-token", controller.GenerateTestCardToken)

	reqBody := payment.TestCardTokenRequest{
		CardNumber:           "5483928164574623",
		ExpirationMonth:      "11",
		ExpirationYear:       "2025",
		SecurityCode:         "123",
		CardholderName:       "Test User",
		IdentificationType:   "DNI",
		IdentificationNumber: "12345678",
	}
	body, _ := json.Marshal(reqBody)

	service.On("GenerateTestCardToken", mock.AnythingOfType("*gin.Context"), reqBody).Return(&payment.TestCardTokenResponse{
		Token: "test-token-abc",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/test-card-token", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp payment.TestCardTokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "test-token-abc", resp.Token)
	service.AssertExpectations(t)
}

func TestGenerateTestCardToken_InvalidBody(t *testing.T) {
	service := new(mockPaymentService)
	controller := NewPaymentController(service)

	router := setupRouter()
	router.POST("/api/v1/payments/test-card-token", controller.GenerateTestCardToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payments/test-card-token", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	service.AssertNotCalled(t, "GenerateTestCardToken")
}
