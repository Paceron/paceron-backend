package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockPasswordResetService struct {
	mockRequestPasswordReset func(ctx *gin.Context, email string) error
	mockResetPassword        func(ctx *gin.Context, email, code, newPassword string) error
}

func (m mockPasswordResetService) RequestPasswordReset(ctx *gin.Context, email string) error {
	return m.mockRequestPasswordReset(ctx, email)
}

func (m mockPasswordResetService) ResetPassword(ctx *gin.Context, email, code, newPassword string) error {
	return m.mockResetPassword(ctx, email, code, newPassword)
}

func TestForgotPassword_ExistingEmail_ReturnsGenericOK(t *testing.T) {
	mockSvc := mockPasswordResetService{
		mockRequestPasswordReset: func(ctx *gin.Context, email string) error {
			return nil
		},
	}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ForgotPassword(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Si el email")
}

func TestForgotPassword_NonExistentEmail_ReturnsSameGenericOK(t *testing.T) {
	mockSvc := mockPasswordResetService{
		mockRequestPasswordReset: func(ctx *gin.Context, email string) error {
			return nil
		},
	}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"nobody@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ForgotPassword(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Si el email")
}

func TestForgotPassword_InvalidBody(t *testing.T) {
	mockSvc := mockPasswordResetService{}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ForgotPassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestForgotPassword_ServiceInfraError_ReturnsInternalServerError(t *testing.T) {
	mockSvc := mockPasswordResetService{
		mockRequestPasswordReset: func(ctx *gin.Context, email string) error {
			return errors.New("db down")
		},
	}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ForgotPassword(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestResetPassword_Success(t *testing.T) {
	mockSvc := mockPasswordResetService{
		mockResetPassword: func(ctx *gin.Context, email, code, newPassword string) error {
			return nil
		},
	}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com","code":"123456","new_password":"NewPass123","confirm_password":"NewPass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ResetPassword(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestResetPassword_PasswordMismatch(t *testing.T) {
	mockSvc := mockPasswordResetService{}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com","code":"123456","new_password":"NewPass123","confirm_password":"Different123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ResetPassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "no coinciden")
}

func TestResetPassword_WeakPassword(t *testing.T) {
	mockSvc := mockPasswordResetService{}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com","code":"123456","new_password":"weak","confirm_password":"weak"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ResetPassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestResetPassword_GenericSecurityError_ReturnsBadRequest(t *testing.T) {
	mockSvc := mockPasswordResetService{
		mockResetPassword: func(ctx *gin.Context, email, code, newPassword string) error {
			return errors.New("código inválido o expirado")
		},
	}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com","code":"000000","new_password":"NewPass123","confirm_password":"NewPass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ResetPassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestResetPassword_ServiceInfraError_ReturnsInternalServerError(t *testing.T) {
	mockSvc := mockPasswordResetService{
		mockResetPassword: func(ctx *gin.Context, email, code, newPassword string) error {
			return errors.New("error al restablecer la contraseña")
		},
	}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"user@test.com","code":"123456","new_password":"NewPass123","confirm_password":"NewPass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ResetPassword(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestResetPassword_InvalidBody(t *testing.T) {
	mockSvc := mockPasswordResetService{}
	controller := NewPasswordResetController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.ResetPassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}
