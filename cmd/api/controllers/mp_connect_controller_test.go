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

	"simple-arq-golang/cmd/api/domains/mpconnect"
)

type mockMPConnectService struct {
	getAuthURLFn          func(ctx *gin.Context, userID int64) (*mpconnect.AuthURLResponse, error)
	handleCallbackFn      func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error)
	getStatusFn           func(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error)
	handleDeauthWebhookFn func(ctx *gin.Context, mpUserID int64) error
}

func (m *mockMPConnectService) GetAuthURL(ctx *gin.Context, userID int64) (*mpconnect.AuthURLResponse, error) {
	if m.getAuthURLFn != nil {
		return m.getAuthURLFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockMPConnectService) HandleCallback(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
	if m.handleCallbackFn != nil {
		return m.handleCallbackFn(ctx, req)
	}
	return nil, nil
}

func (m *mockMPConnectService) GetStatus(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockMPConnectService) HandleDeauthorization(ctx *gin.Context, mpUserID int64) error {
	if m.handleDeauthWebhookFn != nil {
		return m.handleDeauthWebhookFn(ctx, mpUserID)
	}
	return nil
}

func TestMPConnectController_GetAuthURL_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		getAuthURLFn: func(ctx *gin.Context, userID int64) (*mpconnect.AuthURLResponse, error) {
			return &mpconnect.AuthURLResponse{AuthURL: "https://auth.mercadopago.com/authorization?client_id=1", State: "123-456"}, nil
		},
	}

	controller := NewMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect", nil)
	setAuthUserID(c, 1)

	controller.GetAuthURL(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result mpconnect.AuthURLResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "https://auth.mercadopago.com/authorization?client_id=1", result.AuthURL)
	assert.Equal(t, "123-456", result.State)
}

func TestMPConnectController_GetAuthURL_Unauthorized(t *testing.T) {
	controller := NewMPConnectController(&mockMPConnectService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect", nil)

	controller.GetAuthURL(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestMPConnectController_GetAuthURL_ServiceError(t *testing.T) {
	mockSvc := &mockMPConnectService{
		getAuthURLFn: func(ctx *gin.Context, userID int64) (*mpconnect.AuthURLResponse, error) {
			return nil, errors.New("configuración de Mercado Pago incompleta")
		},
	}

	controller := NewMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect", nil)
	setAuthUserID(c, 1)

	controller.GetAuthURL(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestMPConnectController_HandleCallback_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return &mpconnect.CallbackResponse{Success: true, Message: "connected"}, nil
		},
	}

	controller := NewMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/callback?code=CODE&state=state", nil)

	controller.HandleCallback(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result mpconnect.CallbackResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.True(t, result.Success)
}

func TestMPConnectController_HandleCallback_ServiceError(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return nil, errors.New("state inválido")
		},
	}

	controller := NewMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/callback?code=&state=bad", nil)

	controller.HandleCallback(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestMPConnectController_GetStatus_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		getStatusFn: func(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error) {
			return &mpconnect.StatusResponse{Connected: true, AccountStatus: "authorized"}, nil
		},
	}

	controller := NewMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/status", nil)
	setAuthUserID(c, 1)

	controller.GetStatus(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result mpconnect.StatusResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.True(t, result.Connected)
	assert.Equal(t, "authorized", result.AccountStatus)
}

func TestMPConnectController_GetStatus_Unauthorized(t *testing.T) {
	controller := NewMPConnectController(&mockMPConnectService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/status", nil)

	controller.GetStatus(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestMPConnectController_HandleDeauthWebhook_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleDeauthWebhookFn: func(ctx *gin.Context, mpUserID int64) error {
			assert.Equal(t, int64(987), mpUserID)
			return nil
		},
	}

	controller := NewMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":987}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/mercadopago/webhook/connect", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.HandleDeauthWebhook(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestMPConnectController_HandleDeauthWebhook_InvalidBody(t *testing.T) {
	controller := NewMPConnectController(&mockMPConnectService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/mercadopago/webhook/connect", strings.NewReader(`{"user_id":0}`))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.HandleDeauthWebhook(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}