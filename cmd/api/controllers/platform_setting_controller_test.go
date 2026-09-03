package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/platformsettings"
)

type mockPlatformSettingService struct {
	getMarketplaceFeeFn      func(ctx *gin.Context) (*platformsettings.MarketplaceFeeResponse, error)
	updateMarketplaceFeeFn   func(ctx *gin.Context, callerID int64, req *platformsettings.UpdateMarketplaceFeeRequest) (*platformsettings.MarketplaceFeeResponse, error)
}

func (m *mockPlatformSettingService) GetMarketplaceFee(ctx *gin.Context) (*platformsettings.MarketplaceFeeResponse, error) {
	if m.getMarketplaceFeeFn != nil {
		return m.getMarketplaceFeeFn(ctx)
	}
	return nil, nil
}

func (m *mockPlatformSettingService) UpdateMarketplaceFee(ctx *gin.Context, callerID int64, req *platformsettings.UpdateMarketplaceFeeRequest) (*platformsettings.MarketplaceFeeResponse, error) {
	if m.updateMarketplaceFeeFn != nil {
		return m.updateMarketplaceFeeFn(ctx, callerID, req)
	}
	return nil, nil
}

func TestPlatformSettingController_GetMarketplaceFee_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockPlatformSettingService{
		getMarketplaceFeeFn: func(ctx *gin.Context) (*platformsettings.MarketplaceFeeResponse, error) {
			return &platformsettings.MarketplaceFeeResponse{MarketplaceFeePercent: 5.0, UpdatedAt: &now}, nil
		},
	}

	controller := NewPlatformSettingController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/platform-settings/marketplace-fee", nil)

	controller.GetMarketplaceFee(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result platformsettings.MarketplaceFeeResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, 5.0, result.MarketplaceFeePercent)
	assert.NotNil(t, result.UpdatedAt)
}

func TestPlatformSettingController_GetMarketplaceFee_ServiceError(t *testing.T) {
	mockSvc := &mockPlatformSettingService{
		getMarketplaceFeeFn: func(ctx *gin.Context) (*platformsettings.MarketplaceFeeResponse, error) {
			return nil, errors.New("error consultando comisión")
		},
	}

	controller := NewPlatformSettingController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/platform-settings/marketplace-fee", nil)

	controller.GetMarketplaceFee(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestPlatformSettingController_UpdateMarketplaceFee_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockPlatformSettingService{
		updateMarketplaceFeeFn: func(ctx *gin.Context, callerID int64, req *platformsettings.UpdateMarketplaceFeeRequest) (*platformsettings.MarketplaceFeeResponse, error) {
			assert.Equal(t, int64(1), callerID)
			return &platformsettings.MarketplaceFeeResponse{MarketplaceFeePercent: req.MarketplaceFeePercent, UpdatedAt: &now}, nil
		},
	}

	controller := NewPlatformSettingController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"marketplace_fee_percent":7.5}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/platform-settings/marketplace-fee", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.UpdateMarketplaceFee(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result platformsettings.MarketplaceFeeResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, 7.5, result.MarketplaceFeePercent)
}

func TestPlatformSettingController_UpdateMarketplaceFee_Unauthorized(t *testing.T) {
	controller := NewPlatformSettingController(&mockPlatformSettingService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/platform-settings/marketplace-fee", strings.NewReader(`{"marketplace_fee_percent":5}`))

	controller.UpdateMarketplaceFee(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestPlatformSettingController_UpdateMarketplaceFee_InvalidBody(t *testing.T) {
	controller := NewPlatformSettingController(&mockPlatformSettingService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/platform-settings/marketplace-fee", strings.NewReader(`{bad`))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.UpdateMarketplaceFee(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPlatformSettingController_UpdateMarketplaceFee_ForbiddenNotOwner(t *testing.T) {
	mockSvc := &mockPlatformSettingService{
		updateMarketplaceFeeFn: func(ctx *gin.Context, callerID int64, req *platformsettings.UpdateMarketplaceFeeRequest) (*platformsettings.MarketplaceFeeResponse, error) {
			return nil, errors.New("no tenés permisos para actualizar la configuración")
		},
	}

	controller := NewPlatformSettingController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"marketplace_fee_percent":7.5}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/platform-settings/marketplace-fee", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.UpdateMarketplaceFee(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}