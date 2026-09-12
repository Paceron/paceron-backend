package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/tiersubscription"
)

type mockTierSubscriptionService struct {
	changeTierFn           func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error)
	getCurrentSubscription func(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error)
	cancelPendingFn        func(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error)
}

func (m *mockTierSubscriptionService) ChangeTier(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
	if m.changeTierFn != nil {
		return m.changeTierFn(ctx, userID, roleID, req)
	}
	return nil, nil
}

func (m *mockTierSubscriptionService) GetCurrentSubscription(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error) {
	if m.getCurrentSubscription != nil {
		return m.getCurrentSubscription(ctx, userID, roleID, period)
	}
	return nil, nil
}

func (m *mockTierSubscriptionService) CancelPendingSubscription(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error) {
	if m.cancelPendingFn != nil {
		return m.cancelPendingFn(ctx, userID, roleID, tierID)
	}
	return nil, nil
}

func TestTierSubscriptionController_ChangeTier_Success(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return &tiersubscription.ChangeTierResponse{
				CurrentSubscriptionResponse: tiersubscription.CurrentSubscriptionResponse{
					Tier: tiersubscription.TierInfo{ID: req.TierID, Name: "premium", Hierarchy: 3, PaymentRequired: true},
					Role: tiersubscription.RoleInfo{ID: roleID, Name: "corredor"},
				},
			}, nil
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tiersubscription.ChangeTierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(2), result.Tier.ID)
	assert.Equal(t, int64(1), result.Role.ID)
}

func TestTierSubscriptionController_ChangeTier_ForbiddenNotSelf(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTierSubscriptionController_ChangeTier_InvalidUserID(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/abc/roles/1/tier", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}, {Key: "role_id", Value: "1"}}

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierSubscriptionController_ChangeTier_InvalidRoleID(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/abc/tier", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "abc"}}

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierSubscriptionController_ChangeTier_TierNotFound(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":999}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "TIER_NOT_FOUND", result.Code)
}

func TestTierSubscriptionController_ChangeTier_RoleMismatch(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return nil, errors.New("el tier no pertenece al rol especificado")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "TIER_ROLE_MISMATCH", result.Code)
}

func TestTierSubscriptionController_ChangeTier_DebtBlocks(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return nil, errors.New("no podés cambiar de tier con deuda pendiente")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "DEBT_BLOCKS_OPERATION", result.Code)
}

func TestTierSubscriptionController_ChangeTier_PendingFirstPayment(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return nil, errors.New("no podés cambiar de tier con el primer pago pendiente")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "SUBSCRIPTION_PENDING_FIRST_PAYMENT", result.Code)
}

func TestTierSubscriptionController_ChangeTier_NotAssigned(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return nil, errors.New("el usuario no tiene asignado este rol")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTierSubscriptionController_ChangeTier_GenericError(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		changeTierFn: func(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
			return nil, errors.New("error al cambiar de tier")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/roles/1/tier", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeTier(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestTierSubscriptionController_GetCurrentSubscription_Success(t *testing.T) {
	tierID := int64(2)
	amount := float64(1500)
	mockSvc := &mockTierSubscriptionService{
		getCurrentSubscription: func(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error) {
			assert.Equal(t, "current", period)
			return &tiersubscription.CurrentSubscriptionResponse{
				SubscriptionID:     7,
				SubscriptionStatus: "active",
				InstallmentID:      &tierID,
				InstallmentAmount:  &amount,
				Tier:               tiersubscription.TierInfo{ID: 2, Name: "premium", Hierarchy: 3, PaymentRequired: true},
				Role:               tiersubscription.RoleInfo{ID: roleID, Name: "corredor"},
			}, nil
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "current"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tiersubscription.CurrentSubscriptionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(7), result.SubscriptionID)
	assert.Equal(t, int64(1), result.Role.ID)
}

func TestTierSubscriptionController_GetCurrentSubscription_Next(t *testing.T) {
	amount := float64(1500)
	mockSvc := &mockTierSubscriptionService{
		getCurrentSubscription: func(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error) {
			assert.Equal(t, "next", period)
			return &tiersubscription.CurrentSubscriptionResponse{
				SubscriptionID:     9,
				SubscriptionStatus: "first_payment_pending",
				InstallmentID:      &roleID,
				InstallmentAmount:  &amount,
				Tier:               tiersubscription.TierInfo{ID: 2, Name: "premium", Hierarchy: 3, PaymentRequired: true},
				Role:               tiersubscription.RoleInfo{ID: roleID, Name: "corredor"},
			}, nil
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/next?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "next"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tiersubscription.CurrentSubscriptionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(9), result.SubscriptionID)
	assert.Equal(t, "first_payment_pending", result.SubscriptionStatus)
}

func TestTierSubscriptionController_GetCurrentSubscription_EmptyBody(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		getCurrentSubscription: func(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error) {
			return nil, nil
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "current"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "{}", response.Body.String())
}

func TestTierSubscriptionController_GetCurrentSubscription_InvalidPeriod(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/whatever", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "whatever"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierSubscriptionController_GetCurrentSubscription_MissingRoleID(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "current"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierSubscriptionController_GetCurrentSubscription_ForbiddenNotSelf(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "current"}}
	setAuthUserID(c, 2)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTierSubscriptionController_GetCurrentSubscription_NotAssigned(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		getCurrentSubscription: func(ctx *gin.Context, userID, roleID int64, period string) (*tiersubscription.CurrentSubscriptionResponse, error) {
			return nil, errors.New("el usuario no tiene asignado este rol")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "period", Value: "current"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTierSubscriptionController_CancelPendingSubscription_Success(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		cancelPendingFn: func(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error) {
			return &tiersubscription.CancelSubscriptionResponse{
				SubscriptionID:     7,
				SubscriptionStatus: "canceled",
			}, nil
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/1/subscriptions/pending?tier_id=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.CancelPendingSubscription(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tiersubscription.CancelSubscriptionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(7), result.SubscriptionID)
	assert.Equal(t, "canceled", result.SubscriptionStatus)
}

func TestTierSubscriptionController_CancelPendingSubscription_NotFound(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		cancelPendingFn: func(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error) {
			return nil, errors.New("suscripción no encontrada")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/1/subscriptions/pending?tier_id=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.CancelPendingSubscription(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "SUBSCRIPTION_NOT_FOUND", result.Code)
}

func TestTierSubscriptionController_CancelPendingSubscription_NotPending(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		cancelPendingFn: func(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error) {
			return nil, errors.New("la suscripción no está en primer pago pendiente")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/1/subscriptions/pending?tier_id=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.CancelPendingSubscription(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "SUBSCRIPTION_NOT_PENDING_FIRST_PAYMENT", result.Code)
}

func TestTierSubscriptionController_CancelPendingSubscription_MissingTierID(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/1/subscriptions/pending", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.CancelPendingSubscription(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierSubscriptionController_CancelPendingSubscription_ForbiddenNotSelf(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/1/subscriptions/pending?tier_id=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.CancelPendingSubscription(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTierSubscriptionController_CancelPendingSubscription_GenericError(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		cancelPendingFn: func(ctx *gin.Context, userID, roleID, tierID int64) (*tiersubscription.CancelSubscriptionResponse, error) {
			return nil, errors.New("error al cancelar la suscripción")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/1/subscriptions/pending?tier_id=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.CancelPendingSubscription(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}