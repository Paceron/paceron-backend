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
	"simple-arq-golang/cmd/api/domains/tierpermission"
)

type mockTierPermissionService struct {
	assignFn   func(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error)
	unassignFn func(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error)
}

func (m *mockTierPermissionService) Assign(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
	if m.assignFn != nil {
		return m.assignFn(ctx, tierID, req)
	}
	return nil, nil
}

func (m *mockTierPermissionService) Unassign(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error) {
	if m.unassignFn != nil {
		return m.unassignFn(ctx, tierID, permissionID)
	}
	return nil, nil
}

func TestTierPermissionController_Assign_Success(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		assignFn: func(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
			return &tierpermission.TierPermissionResponse{
				ID:           1,
				TierID:       tierID,
				PermissionID: req.PermissionID,
			}, nil
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"permission_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result tierpermission.TierPermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.TierID)
	assert.Equal(t, int64(1), result.PermissionID)
}

func TestTierPermissionController_Assign_TierNotFound(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		assignFn: func(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"permission_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/999/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestTierPermissionController_Assign_AlreadyAssigned(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		assignFn: func(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
			return nil, errors.New("el permiso ya está asignado a este tier")
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"permission_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestTierPermissionController_Unassign_Success(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		unassignFn: func(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error) {
			return &tierpermission.DeleteTierPermissionResponse{
				Message: "Permiso desasignado del tier correctamente",
			}, nil
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/1/permissions/1", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: "1"},
		{Key: "permission_id", Value: "1"},
	}

	controller.Unassign(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tierpermission.DeleteTierPermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Permiso desasignado del tier correctamente", result.Message)
}

func TestTierPermissionController_Unassign_NotFound(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		unassignFn: func(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error) {
			return nil, errors.New("asignación no encontrada")
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/1/permissions/999", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: "1"},
		{Key: "permission_id", Value: "999"},
	}

	controller.Unassign(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestTierPermissionController_Assign_InvalidTierID(t *testing.T) {
	controller := NewTierPermissionController(&mockTierPermissionService{})
	response := httptest.NewRecorder()
	body := `{"permission_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/abc/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierPermissionController_Unassign_InvalidTierID(t *testing.T) {
	controller := NewTierPermissionController(&mockTierPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/abc/permissions/1", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: "abc"},
		{Key: "permission_id", Value: "1"},
	}

	controller.Unassign(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierPermissionController_Unassign_InvalidPermissionID(t *testing.T) {
	controller := NewTierPermissionController(&mockTierPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/1/permissions/abc", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: "1"},
		{Key: "permission_id", Value: "abc"},
	}

	controller.Unassign(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierPermissionController_Assign_InvalidBody(t *testing.T) {
	controller := NewTierPermissionController(&mockTierPermissionService{})
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierPermissionController_Assign_PermissionNotFound(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		assignFn: func(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
			return nil, errors.New("permiso no encontrado")
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"permission_id":999}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestTierPermissionController_Assign_InternalError(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		assignFn: func(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
			return nil, errors.New("error al asignar permiso")
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"permission_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestTierPermissionController_Unassign_InternalError(t *testing.T) {
	mockSvc := &mockTierPermissionService{
		unassignFn: func(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error) {
			return nil, errors.New("error al desasignar permiso")
		},
	}

	controller := NewTierPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/1/permissions/1", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: "1"},
		{Key: "permission_id", Value: "1"},
	}

	controller.Unassign(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestTierPermissionController_Unassign_BothInvalidIDs(t *testing.T) {
	controller := NewTierPermissionController(&mockTierPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/abc/permissions/xyz", nil)
	c.Params = []gin.Param{
		{Key: "id", Value: "abc"},
		{Key: "permission_id", Value: "xyz"},
	}

	controller.Unassign(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "tier id debe ser un número válido", result.Message)
}

func TestTierPermissionController_Assign_InvalidBodyMalformed(t *testing.T) {
	controller := NewTierPermissionController(&mockTierPermissionService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers/1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Assign(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}
