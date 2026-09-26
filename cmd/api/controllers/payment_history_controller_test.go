package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockPaymentHistoryService struct {
	listReceivedFn       func(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error)
	listMyTierPaymentsFn func(ctx *gin.Context, userID int64, roleName string, page int) (*payment.TierPaymentsResponse, error)
	getReceivedSummaryFn func(ctx *gin.Context, sellerID int64, months int) (*payment.ReceivedSummaryResponse, error)
}

func (m *mockPaymentHistoryService) ListReceived(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error) {
	return m.listReceivedFn(ctx, sellerID, teamID, statusGroup, page)
}

func (m *mockPaymentHistoryService) ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page int) (*payment.TierPaymentsResponse, error) {
	return m.listMyTierPaymentsFn(ctx, userID, roleName, page)
}

func (m *mockPaymentHistoryService) GetReceivedSummary(ctx *gin.Context, sellerID int64, months int) (*payment.ReceivedSummaryResponse, error) {
	return m.getReceivedSummaryFn(ctx, sellerID, months)
}

func paymentHistoryRequest(target string, authenticated bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	if authenticated {
		c.Set(utils.AuthUserIDKey, int64(7))
	}
	return c, w
}

// --- ListReceived ---

func TestPaymentHistoryController_ListReceived_Success(t *testing.T) {
	svc := &mockPaymentHistoryService{listReceivedFn: func(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error) {
		assert.Equal(t, int64(7), sellerID)
		if assert.NotNil(t, teamID) {
			assert.Equal(t, int64(12), *teamID)
		}
		assert.Equal(t, "rejected", statusGroup)
		assert.Equal(t, 2, page)
		return &payment.ReceivedPaymentsResponse{Payments: []payment.ReceivedPaymentItem{{ID: 1}}, HasMore: true}, nil
	}}
	c, w := paymentHistoryRequest("/api/v1/payments/received?page=2&team_id=12&status=rejected", true)

	NewPaymentHistoryController(svc).ListReceived(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"has_more":true`)
}

func TestPaymentHistoryController_ListReceived_Defaults(t *testing.T) {
	svc := &mockPaymentHistoryService{listReceivedFn: func(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error) {
		assert.Nil(t, teamID)
		assert.Equal(t, "", statusGroup)
		assert.Equal(t, 1, page)
		return &payment.ReceivedPaymentsResponse{Payments: []payment.ReceivedPaymentItem{}}, nil
	}}
	c, w := paymentHistoryRequest("/api/v1/payments/received", true)

	NewPaymentHistoryController(svc).ListReceived(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPaymentHistoryController_ListReceived_BadQuery(t *testing.T) {
	for _, target := range []string{"/api/v1/payments/received?page=abc", "/api/v1/payments/received?team_id=x"} {
		c, w := paymentHistoryRequest(target, true)
		NewPaymentHistoryController(&mockPaymentHistoryService{}).ListReceived(c)
		assert.Equal(t, http.StatusBadRequest, w.Code, target)
		assert.Contains(t, w.Body.String(), "INVALID_QUERY", target)
	}
}

func TestPaymentHistoryController_ListReceived_ServiceErrors(t *testing.T) {
	cases := map[error]int{
		fmt.Errorf("%w: page", services.ErrInvalidPaymentHistoryQuery): http.StatusBadRequest,
		errors.New("db caída"): http.StatusInternalServerError,
	}
	for svcErr, want := range cases {
		svc := &mockPaymentHistoryService{listReceivedFn: func(*gin.Context, int64, *int64, string, int) (*payment.ReceivedPaymentsResponse, error) {
			return nil, svcErr
		}}
		c, w := paymentHistoryRequest("/api/v1/payments/received?page=0", true)
		NewPaymentHistoryController(svc).ListReceived(c)
		assert.Equal(t, want, w.Code, svcErr.Error())
	}
}

func TestPaymentHistoryController_ListReceived_Unauthorized(t *testing.T) {
	c, w := paymentHistoryRequest("/api/v1/payments/received", false)
	NewPaymentHistoryController(&mockPaymentHistoryService{}).ListReceived(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- GetReceivedSummary ---

func TestPaymentHistoryController_GetReceivedSummary_DefaultMonths(t *testing.T) {
	svc := &mockPaymentHistoryService{getReceivedSummaryFn: func(ctx *gin.Context, sellerID int64, months int) (*payment.ReceivedSummaryResponse, error) {
		assert.Equal(t, services.PaymentSummaryDefaultMonths, months)
		return &payment.ReceivedSummaryResponse{Months: months}, nil
	}}
	c, w := paymentHistoryRequest("/api/v1/payments/received/summary", true)

	NewPaymentHistoryController(svc).GetReceivedSummary(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPaymentHistoryController_GetReceivedSummary_Errors(t *testing.T) {
	c, w := paymentHistoryRequest("/api/v1/payments/received/summary?months=seis", true)
	NewPaymentHistoryController(&mockPaymentHistoryService{}).GetReceivedSummary(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	svc := &mockPaymentHistoryService{getReceivedSummaryFn: func(*gin.Context, int64, int) (*payment.ReceivedSummaryResponse, error) {
		return nil, fmt.Errorf("%w: months", services.ErrInvalidPaymentHistoryQuery)
	}}
	c, w = paymentHistoryRequest("/api/v1/payments/received/summary?months=13", true)
	NewPaymentHistoryController(svc).GetReceivedSummary(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = paymentHistoryRequest("/api/v1/payments/received/summary", false)
	NewPaymentHistoryController(svc).GetReceivedSummary(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- ListMine ---

func TestPaymentHistoryController_ListMine(t *testing.T) {
	svc := &mockPaymentHistoryService{listMyTierPaymentsFn: func(ctx *gin.Context, userID int64, roleName string, page int) (*payment.TierPaymentsResponse, error) {
		assert.Equal(t, "entrenador", roleName)
		return &payment.TierPaymentsResponse{Payments: []payment.TierPaymentItem{}}, nil
	}}
	c, w := paymentHistoryRequest("/api/v1/payments/mine?role=entrenador", true)
	NewPaymentHistoryController(svc).ListMine(c)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"payments":[]`)

	c, w = paymentHistoryRequest("/api/v1/payments/mine?page=x", true)
	NewPaymentHistoryController(svc).ListMine(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = paymentHistoryRequest("/api/v1/payments/mine", false)
	NewPaymentHistoryController(svc).ListMine(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	failing := &mockPaymentHistoryService{listMyTierPaymentsFn: func(*gin.Context, int64, string, int) (*payment.TierPaymentsResponse, error) {
		return nil, errors.New("db caída")
	}}
	c, w = paymentHistoryRequest("/api/v1/payments/mine", true)
	NewPaymentHistoryController(failing).ListMine(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
