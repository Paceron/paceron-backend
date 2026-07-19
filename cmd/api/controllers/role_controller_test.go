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
	"simple-arq-golang/cmd/api/domains/role"
)

type mockRoleService struct {
	createFn func(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error)
	updateFn func(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error)
	deleteFn func(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error)
	getByIDFn  func(ctx *gin.Context, id int64) (*role.RoleResponse, error)
	getByNameFn func(ctx *gin.Context, name string) (*role.RoleResponse, error)
	getAllFn    func(ctx *gin.Context) ([]role.RoleResponse, error)
}

func (m *mockRoleService) Create(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, nil
}

func (m *mockRoleService) Update(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, req)
	}
	return nil, nil
}

func (m *mockRoleService) Delete(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error) {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRoleService) GetByID(ctx *gin.Context, id int64) (*role.RoleResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRoleService) GetByName(ctx *gin.Context, name string) (*role.RoleResponse, error) {
	if m.getByNameFn != nil {
		return m.getByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockRoleService) GetAll(ctx *gin.Context) ([]role.RoleResponse, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return nil, nil
}

func TestRoleController_Create_Success(t *testing.T) {
	mockSvc := &mockRoleService{
		createFn: func(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error) {
			return &role.RoleResponse{
				ID:          1,
				Name:        req.Name,
				Description: req.Description,
			}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"corredor","description":"Rol de corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result role.RoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "corredor", result.Name)
}

func TestRoleController_Create_InvalidBody(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Cuerpo de solicitud inválido", result.Message)
}

func TestRoleController_Create_DuplicateName(t *testing.T) {
	mockSvc := &mockRoleService{
		createFn: func(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("el nombre del rol ya existe")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestRoleController_Update_Success(t *testing.T) {
	mockSvc := &mockRoleService{
		updateFn: func(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
			return &role.RoleResponse{
				ID:   id,
				Name: "corredor_v2",
			}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"corredor_v2"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result role.RoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "corredor_v2", result.Name)
}

func TestRoleController_Update_NotFound(t *testing.T) {
	mockSvc := &mockRoleService{
		updateFn: func(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"nuevo"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestRoleController_Delete_Success(t *testing.T) {
	mockSvc := &mockRoleService{
		deleteFn: func(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error) {
			return &role.DeleteRoleResponse{
				Message: "Rol eliminado correctamente",
			}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/roles/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result role.DeleteRoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Rol eliminado correctamente", result.Message)
}

func TestRoleController_Delete_NotFound(t *testing.T) {
	mockSvc := &mockRoleService{
		deleteFn: func(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/roles/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestRoleController_Update_InvalidID(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	body := `{"name":"nuevo"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/abc", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_Delete_InvalidID(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/roles/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_Create_EmptyName(t *testing.T) {
	mockSvc := &mockRoleService{
		createFn: func(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("el nombre es requerido")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"test"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestRoleController_Create_InternalError(t *testing.T) {
	mockSvc := &mockRoleService{
		createFn: func(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("error al crear rol")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestRoleController_Update_InternalError(t *testing.T) {
	mockSvc := &mockRoleService{
		updateFn: func(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("error al actualizar rol")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestRoleController_Update_InvalidBody(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_Update_EmptyName(t *testing.T) {
	mockSvc := &mockRoleService{
		updateFn: func(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("el nombre no puede estar vacío")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":""}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestRoleController_Update_DuplicateName(t *testing.T) {
	mockSvc := &mockRoleService{
		updateFn: func(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
			return nil, errors.New("el nombre del rol ya existe")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"existing"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestRoleController_Delete_InternalError(t *testing.T) {
	mockSvc := &mockRoleService{
		deleteFn: func(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error) {
			return nil, errors.New("error al eliminar rol")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/roles/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestRoleController_Update_InvalidIDOverflow(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	body := `{"name":"nuevo"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/roles/99999999999999999999999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_Delete_InvalidIDOverflow(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/roles/99999999999999999999999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_Create_InvalidBodyEmptyObject(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	body := `{}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_GetByID_Success(t *testing.T) {
	mockSvc := &mockRoleService{
		getByIDFn: func(ctx *gin.Context, id int64) (*role.RoleResponse, error) {
			return &role.RoleResponse{ID: 1, Name: "corredor", Description: "desc"}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result role.RoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "corredor", result.Name)
}

func TestRoleController_GetByID_NotFound(t *testing.T) {
	mockSvc := &mockRoleService{
		getByIDFn: func(ctx *gin.Context, id int64) (*role.RoleResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestRoleController_GetByID_InvalidID(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_GetByName_Success(t *testing.T) {
	mockSvc := &mockRoleService{
		getByNameFn: func(ctx *gin.Context, name string) (*role.RoleResponse, error) {
			return &role.RoleResponse{ID: 1, Name: name, Description: "desc"}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles/by-name?name=corredor", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result role.RoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "corredor", result.Name)
}

func TestRoleController_GetByName_NotFound(t *testing.T) {
	mockSvc := &mockRoleService{
		getByNameFn: func(ctx *gin.Context, name string) (*role.RoleResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles/by-name?name=nonexistent", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestRoleController_GetByName_EmptyName(t *testing.T) {
	controller := NewRoleController(&mockRoleService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles/by-name", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRoleController_GetAll_Success(t *testing.T) {
	mockSvc := &mockRoleService{
		getAllFn: func(ctx *gin.Context) ([]role.RoleResponse, error) {
			return []role.RoleResponse{
				{ID: 1, Name: "corredor"},
				{ID: 2, Name: "entrenador"},
			}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result []role.RoleResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestRoleController_GetAll_Empty(t *testing.T) {
	mockSvc := &mockRoleService{
		getAllFn: func(ctx *gin.Context) ([]role.RoleResponse, error) {
			return []role.RoleResponse{}, nil
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestRoleController_GetAll_InternalError(t *testing.T) {
	mockSvc := &mockRoleService{
		getAllFn: func(ctx *gin.Context) ([]role.RoleResponse, error) {
			return nil, errors.New("error al obtener roles")
		},
	}

	controller := NewRoleController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/roles", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
