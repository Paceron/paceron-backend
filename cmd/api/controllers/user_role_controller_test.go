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
	assignRoleFn           func(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error)
	removeRoleFn           func(ctx *gin.Context, userID, roleID int64) error
	activateEntrenadorFn   func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error)
	deactivateEntrenadorFn func(ctx *gin.Context, userID int64) error
}

func (m *mockUserRoleService) AssignRole(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
	if m.assignRoleFn != nil {
		return m.assignRoleFn(ctx, userID, req)
	}
	return nil, nil
}

func (m *mockUserRoleService) RemoveRole(ctx *gin.Context, userID, roleID int64) error {
	if m.removeRoleFn != nil {
		return m.removeRoleFn(ctx, userID, roleID)
	}
	return nil
}

func (m *mockUserRoleService) ActivateEntrenador(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
	if m.activateEntrenadorFn != nil {
		return m.activateEntrenadorFn(ctx, userID, req)
	}
	return nil, nil
}

func (m *mockUserRoleService) DeactivateEntrenador(ctx *gin.Context, userID int64) error {
	if m.deactivateEntrenadorFn != nil {
		return m.deactivateEntrenadorFn(ctx, userID)
	}
	return nil
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
	setAuthUserID(c, 1)

	controller.AssignRole(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result userrole.UserRoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
	assert.Equal(t, int64(1), result.RoleID)
}

func TestUserRoleController_AssignRole_Forbidden(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	body := `{"role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.AssignRole(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
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
	setAuthUserID(c, 999)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

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
	setAuthUserID(c, 1)

	controller.AssignRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_RemoveRole_Success(t *testing.T) {
	mockSvc := &mockUserRoleService{
		removeRoleFn: func(ctx *gin.Context, userID, roleID int64) error {
			return nil
		},
	}
	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "2"}}
	setAuthUserID(c, 1)

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUserRoleController_RemoveRole_Forbidden(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "2"}}
	setAuthUserID(c, 2)

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestUserRoleController_RemoveRole_NotAssigned(t *testing.T) {
	mockSvc := &mockUserRoleService{
		removeRoleFn: func(ctx *gin.Context, userID, roleID int64) error {
			return errors.New("el usuario no tiene asignado este rol")
		},
	}
	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "2"}}
	setAuthUserID(c, 1)

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestUserRoleController_RemoveRole_ProtectedRole(t *testing.T) {
	mockSvc := &mockUserRoleService{
		removeRoleFn: func(ctx *gin.Context, userID, roleID int64) error {
			return errors.New("el rol 'corredor' no se puede eliminar, es el rol base de todo usuario")
		},
	}
	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "2"}}
	setAuthUserID(c, 1)

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusForbidden, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Forbidden", result.Code)
}

func TestUserRoleController_RemoveRole_InvalidUserID(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/abc/roles/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}, {Key: "role_id", Value: "2"}}

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_RemoveRole_InvalidRoleID(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "abc"}}
	setAuthUserID(c, 1)

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_RemoveRole_ServiceError(t *testing.T) {
	mockSvc := &mockUserRoleService{
		removeRoleFn: func(ctx *gin.Context, userID, roleID int64) error {
			return errors.New("error al eliminar rol")
		},
	}
	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/roles/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "role_id", Value: "2"}}
	setAuthUserID(c, 1)

	controller.RemoveRole(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestUserRoleController_ActivateEntrenador_Success(t *testing.T) {
	mockSvc := &mockUserRoleService{
		activateEntrenadorFn: func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
			return &userrole.UserRoleResponse{ID: 1, UserID: userID, RoleID: 2, Status: "active"}, nil
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"password":"CorrectPass123","bank_alias":"mi.alias-1"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusCreated, response.Code)
}

func TestUserRoleController_ActivateEntrenador_Forbidden(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	body := `{"password":"x"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestUserRoleController_ActivateEntrenador_InvalidUserID(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/abc/entrenador-role", strings.NewReader(`{"password":"x"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_ActivateEntrenador_InvalidBody(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_ActivateEntrenador_WrongPassword(t *testing.T) {
	mockSvc := &mockUserRoleService{
		activateEntrenadorFn: func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("contraseña actual incorrecta")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"password":"wrong","bank_alias":"alias-valido"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestUserRoleController_ActivateEntrenador_MissingBankAlias(t *testing.T) {
	mockSvc := &mockUserRoleService{
		activateEntrenadorFn: func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("se requiere un alias bancario para activar el rol entrenador")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"password":"CorrectPass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_ActivateEntrenador_AlreadyAssigned(t *testing.T) {
	mockSvc := &mockUserRoleService{
		activateEntrenadorFn: func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("el usuario ya tiene asignado este rol")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"password":"CorrectPass123","bank_alias":"alias-valido"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestUserRoleController_ActivateEntrenador_UserNotFound(t *testing.T) {
	mockSvc := &mockUserRoleService{
		activateEntrenadorFn: func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"password":"CorrectPass123","bank_alias":"alias-valido"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/999/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 999)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUserRoleController_ActivateEntrenador_InternalError(t *testing.T) {
	mockSvc := &mockUserRoleService{
		activateEntrenadorFn: func(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
			return nil, errors.New("error al activar rol entrenador")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"password":"CorrectPass123","bank_alias":"alias-valido"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/users/1/entrenador-role", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ActivateEntrenador(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestUserRoleController_DeactivateEntrenador_Success(t *testing.T) {
	mockSvc := &mockUserRoleService{
		deactivateEntrenadorFn: func(ctx *gin.Context, userID int64) error {
			return nil
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/entrenador-role", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.DeactivateEntrenador(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUserRoleController_DeactivateEntrenador_Forbidden(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/entrenador-role", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.DeactivateEntrenador(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestUserRoleController_DeactivateEntrenador_InvalidUserID(t *testing.T) {
	controller := NewUserRoleController(&mockUserRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/abc/entrenador-role", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.DeactivateEntrenador(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRoleController_DeactivateEntrenador_BlockedByActiveTeam(t *testing.T) {
	mockSvc := &mockUserRoleService{
		deactivateEntrenadorFn: func(ctx *gin.Context, userID int64) error {
			return errors.New("no podés desactivar el rol entrenador mientras lideres equipos activos")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/entrenador-role", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.DeactivateEntrenador(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestUserRoleController_DeactivateEntrenador_NotAssigned(t *testing.T) {
	mockSvc := &mockUserRoleService{
		deactivateEntrenadorFn: func(ctx *gin.Context, userID int64) error {
			return errors.New("el usuario no tiene asignado este rol")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/entrenador-role", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.DeactivateEntrenador(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUserRoleController_DeactivateEntrenador_InternalError(t *testing.T) {
	mockSvc := &mockUserRoleService{
		deactivateEntrenadorFn: func(ctx *gin.Context, userID int64) error {
			return errors.New("error al desactivar rol entrenador")
		},
	}

	controller := NewUserRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/entrenador-role", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.DeactivateEntrenador(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
