package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/pushtoken"
)

type mockPushTokenService struct {
	registerTokenFn func(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error
}

func (m *mockPushTokenService) RegisterToken(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error {
	return m.registerTokenFn(ctx, userID, req)
}

func TestPushTokenController_RegisterToken_Success(t *testing.T) {
	mockSvc := &mockPushTokenService{
		registerTokenFn: func(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error {
			assert.Equal(t, int64(1), userID)
			return nil
		},
	}
	controller := NewPushTokenController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"token":"ExponentPushToken[abc]","platform":"android"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/push-tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.RegisterToken(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestPushTokenController_RegisterToken_Unauthorized(t *testing.T) {
	mockSvc := &mockPushTokenService{}
	controller := NewPushTokenController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"token":"ExponentPushToken[abc]","platform":"android"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/push-tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.RegisterToken(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestPushTokenController_RegisterToken_InvalidBody(t *testing.T) {
	mockSvc := &mockPushTokenService{}
	controller := NewPushTokenController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/push-tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.RegisterToken(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPushTokenController_RegisterToken_InvalidPlatform(t *testing.T) {
	mockSvc := &mockPushTokenService{
		registerTokenFn: func(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error {
			return errors.New("platform inválida: ios. Valores permitidos: [android web]")
		},
	}
	controller := NewPushTokenController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"token":"ExponentPushToken[abc]","platform":"ios"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/push-tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.RegisterToken(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPushTokenController_RegisterToken_InternalError(t *testing.T) {
	mockSvc := &mockPushTokenService{
		registerTokenFn: func(ctx *gin.Context, userID int64, req *pushtoken.RegisterPushTokenRequest) error {
			return errors.New("error al registrar el token de push")
		},
	}
	controller := NewPushTokenController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"token":"ExponentPushToken[abc]","platform":"android"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/push-tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.RegisterToken(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
