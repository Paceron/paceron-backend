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
	getCurrentSubscription func(ctx *gin.Context, userID, roleID int64) (*tiersubscription.CurrentSubscriptionResponse, error)
}

func (m *mockTierSubscriptionService) ChangeTier(ctx *gin.Context, userID, roleID int64, req *tiersubscription.ChangeTierRequest) (*tiersubscription.ChangeTierResponse, error) {
	if m.changeTierFn != nil {
		return m.changeTierFn(ctx, userID, roleID, req)
	}
	return nil, nil
}

func (m *mockTierSubscriptionService) GetCurrentSubscription(ctx *gin.Context, userID, roleID int64) (*tiersubscription.CurrentSubscriptionResponse, error) {
	if m.getCurrentSubscription != nil {
		return m.getCurrentSubscription(ctx, userID, roleID)
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
		getCurrentSubscription: func(ctx *gin.Context, userID, roleID int64) (*tiersubscription.CurrentSubscriptionResponse, error) {
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
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tiersubscription.CurrentSubscriptionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(7), result.SubscriptionID)
	assert.Equal(t, int64(1), result.Role.ID)
}

func TestTierSubscriptionController_GetCurrentSubscription_MissingRoleID(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierSubscriptionController_GetCurrentSubscription_ForbiddenNotSelf(t *testing.T) {
	controller := NewTierSubscriptionController(&mockTierSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTierSubscriptionController_GetCurrentSubscription_NotAssigned(t *testing.T) {
	mockSvc := &mockTierSubscriptionService{
		getCurrentSubscription: func(ctx *gin.Context, userID, roleID int64) (*tiersubscription.CurrentSubscriptionResponse, error) {
			return nil, errors.New("el usuario no tiene asignado este rol")
		},
	}

	controller := NewTierSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/subscriptions/current?role_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.GetCurrentSubscription(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}
