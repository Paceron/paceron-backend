package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/services"
)

type mockPermissionsQueryService struct {
	getUserPermissionsFn func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error)
}

func (m *mockPermissionsQueryService) GetUserPermissions(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
	if m.getUserPermissionsFn != nil {
		return m.getUserPermissionsFn(ctx, userID)
	}
	return nil, nil
}

func TestPermissionsQueryController_GetUserPermissions_Success(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return &services.PermissionsQueryResponse{
				UserID: userID,
				Roles: []services.RolePermission{
					{
						ID:          1,
						Name:        "corredor",
						Tier:        "base",
						Permissions: []string{"crear_venta"},
					},
				},
			}, nil
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=1", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result services.PermissionsQueryResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
	assert.Len(t, result.Roles, 1)
	assert.Equal(t, "corredor", result.Roles[0].Name)
}

func TestPermissionsQueryController_GetUserPermissions_MissingUserID(t *testing.T) {
	controller := NewPermissionsQueryController(&mockPermissionsQueryService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "El parámetro user_id es requerido", result.Message)
}

func TestPermissionsQueryController_GetUserPermissions_UserNotFound(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=999", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestPermissionsQueryController_GetUserPermissions_InvalidUserID(t *testing.T) {
	controller := NewPermissionsQueryController(&mockPermissionsQueryService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=abc", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "user_id debe ser un número válido", result.Message)
}

func TestPermissionsQueryController_GetUserPermissions_InternalError(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return nil, errors.New("error al obtener permisos")
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=1", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestPermissionsQueryController_GetUserPermissions_NegativeUserID(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=-1", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestPermissionsQueryController_GetUserPermissions_ZeroUserID(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=0", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestPermissionsQueryController_GetUserPermissions_ShortErrorMessage(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return nil, errors.New("short error")
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=1", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestPermissionsQueryController_GetUserPermissions_MissingDataError(t *testing.T) {
	mockSvc := &mockPermissionsQueryService{
		getUserPermissionsFn: func(ctx *gin.Context, userID int64) (*services.PermissionsQueryResponse, error) {
			return nil, errors.New("datos faltantes: rol_id=1 no configurado")
		},
	}

	controller := NewPermissionsQueryController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/permissions?user_id=1", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	controller.GetUserPermissions(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}
