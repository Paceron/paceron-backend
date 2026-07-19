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
	"simple-arq-golang/cmd/api/domains/userrole"
)

type mockUserRoleService struct {
	assignRoleFn func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error)
}

func (m *mockUserRoleService) AssignRole(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
	if m.assignRoleFn != nil {
		return m.assignRoleFn(ctx, userID, req)
	}
	return nil, nil
}

func TestUserRoleController_AssignRole_Success(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return &userrole.UserRoleResponse{
				ID:     1,
				UserID: userID,
				RoleID: req.RoleID,
				TierID: req.TierID,
				Status: "active",
			}, nil
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1,"tier_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result userrole.UserRoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
	assert.Equal(t, int64(1), result.RoleID)
}

func TestUserRoleController_AssignRole_UserNotFound(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/999/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestUserRoleController_AssignRole_RoleNotFound(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":999}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestUserRoleController_AssignRole_AlreadyAssigned(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("el usuario ya tiene asignado este rol")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestUserRoleController_AssignRole_InvalidUserID(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	body := `{"role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/abc/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "user id debe ser un número válido", result.Message)
}

func TestUserRoleController_AssignRole_InvalidBody(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_AssignRole_TierNotFound(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1,"tier_id":999}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestUserRoleController_AssignRole_TierNotBelongToRole(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("el tier no pertenece al rol especificado")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1,"tier_id":2}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestUserRoleController_AssignRole_InternalError(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("error al asignar rol")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1,"tier_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestUserRoleController_AssignRole_DefaultTierNotFound(t *testing.T) {
	mockSvc := &mockUserRoleService{
		assignRoleFn: func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("el tier por defecto 'base' no existe para este rol")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestUserRoleController_AssignRole_InvalidUserIDOverflow(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	body := `{"role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/99999999999999999999999/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_AssignRole_InvalidBodyMalformed(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}
