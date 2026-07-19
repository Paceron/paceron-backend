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
	"simple-arq-golang/cmd/api/domains/permission"
)

type mockPermissionService struct {
	createFn func(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error)
	updateFn func(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error)
	deleteFn func(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error)
	getByIDFn  func(ctx *gin.Context, id int64) (*permission.PermissionResponse, error)
	getByNameFn func(ctx *gin.Context, name string) (*permission.PermissionResponse, error)
	getAllFn    func(ctx *gin.Context) ([]permission.PermissionResponse, error)
}

func (m *mockPermissionService) Create(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, nil
}

func (m *mockPermissionService) Update(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, req)
	}
	return nil, nil
}

func (m *mockPermissionService) Delete(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error) {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil, nil
}

func (m *mockPermissionService) GetByID(ctx *gin.Context, id int64) (*permission.PermissionResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockPermissionService) GetByName(ctx *gin.Context, name string) (*permission.PermissionResponse, error) {
	if m.getByNameFn != nil {
		return m.getByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockPermissionService) GetAll(ctx *gin.Context) ([]permission.PermissionResponse, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return nil, nil
}

func TestPermissionController_Create_Success(t *testing.T) {
	mockSvc := &mockPermissionService{
		createFn: func(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error) {
			return &permission.PermissionResponse{
				ID:   1,
				Name: req.Name,
			}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"crear_venta","description":"Permiso para crear ventas"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result permission.PermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "crear_venta", result.Name)
}

func TestPermissionController_Create_InvalidBody(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Cuerpo de solicitud inválido", result.Message)
}

func TestPermissionController_Create_DuplicateName(t *testing.T) {
	mockSvc := &mockPermissionService{
		createFn: func(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("el nombre del permiso ya existe")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"crear_venta"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestPermissionController_Update_Success(t *testing.T) {
	mockSvc := &mockPermissionService{
		updateFn: func(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
			return &permission.PermissionResponse{
				ID:   id,
				Name: "updated",
			}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result permission.PermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "updated", result.Name)
}

func TestPermissionController_Update_NotFound(t *testing.T) {
	mockSvc := &mockPermissionService{
		updateFn: func(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("permiso no encontrado")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestPermissionController_Update_InvalidID(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	body := `{"name":"updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/abc", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "id debe ser un número válido", result.Message)
}

func TestPermissionController_Delete_Success(t *testing.T) {
	mockSvc := &mockPermissionService{
		deleteFn: func(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error) {
			return &permission.DeletePermissionResponse{
				Message: "Permiso eliminado correctamente",
			}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/permissions/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result permission.DeletePermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Permiso eliminado correctamente", result.Message)
}

func TestPermissionController_Delete_NotFound(t *testing.T) {
	mockSvc := &mockPermissionService{
		deleteFn: func(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error) {
			return nil, errors.New("permiso no encontrado")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/permissions/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestPermissionController_Delete_InvalidID(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/permissions/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_Create_EmptyName(t *testing.T) {
	mockSvc := &mockPermissionService{
		createFn: func(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("el nombre es requerido")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"test"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestPermissionController_Create_InternalError(t *testing.T) {
	mockSvc := &mockPermissionService{
		createFn: func(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("error al crear permiso")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"test"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestPermissionController_Update_InternalError(t *testing.T) {
	mockSvc := &mockPermissionService{
		updateFn: func(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("error al actualizar permiso")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestPermissionController_Update_InvalidBody(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_Update_EmptyName(t *testing.T) {
	mockSvc := &mockPermissionService{
		updateFn: func(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("el nombre no puede estar vacío")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":""}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestPermissionController_Update_DuplicateName(t *testing.T) {
	mockSvc := &mockPermissionService{
		updateFn: func(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
			return nil, errors.New("el nombre del permiso ya existe")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"duplicate"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestPermissionController_Delete_InternalError(t *testing.T) {
	mockSvc := &mockPermissionService{
		deleteFn: func(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error) {
			return nil, errors.New("error al eliminar permiso")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/permissions/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestPermissionController_Create_InvalidBodyMalformed(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_Update_InvalidIDOverflow(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	body := `{"name":"updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/permissions/99999999999999999999999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_Delete_InvalidIDOverflow(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/permissions/99999999999999999999999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_Create_InvalidBodyEmptyObject(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	body := `{}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/permissions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_GetByID_Success(t *testing.T) {
	mockSvc := &mockPermissionService{
		getByIDFn: func(ctx *gin.Context, id int64) (*permission.PermissionResponse, error) {
			return &permission.PermissionResponse{ID: 1, Name: "crear_venta", Description: "desc"}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result permission.PermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "crear_venta", result.Name)
}

func TestPermissionController_GetByID_NotFound(t *testing.T) {
	mockSvc := &mockPermissionService{
		getByIDFn: func(ctx *gin.Context, id int64) (*permission.PermissionResponse, error) {
			return nil, errors.New("permiso no encontrado")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestPermissionController_GetByID_InvalidID(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_GetByName_Success(t *testing.T) {
	mockSvc := &mockPermissionService{
		getByNameFn: func(ctx *gin.Context, name string) (*permission.PermissionResponse, error) {
			return &permission.PermissionResponse{ID: 1, Name: name, Description: "desc"}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions/by-name?name=crear_venta", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result permission.PermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "crear_venta", result.Name)
}

func TestPermissionController_GetByName_NotFound(t *testing.T) {
	mockSvc := &mockPermissionService{
		getByNameFn: func(ctx *gin.Context, name string) (*permission.PermissionResponse, error) {
			return nil, errors.New("permiso no encontrado")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions/by-name?name=nonexistent", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestPermissionController_GetByName_EmptyName(t *testing.T) {
	controller := NewPermissionController(&mockPermissionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions/by-name", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestPermissionController_GetAll_Success(t *testing.T) {
	mockSvc := &mockPermissionService{
		getAllFn: func(ctx *gin.Context) ([]permission.PermissionResponse, error) {
			return []permission.PermissionResponse{
				{ID: 1, Name: "crear_venta"},
				{ID: 2, Name: "ver_venta"},
			}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result []permission.PermissionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestPermissionController_GetAll_Empty(t *testing.T) {
	mockSvc := &mockPermissionService{
		getAllFn: func(ctx *gin.Context) ([]permission.PermissionResponse, error) {
			return []permission.PermissionResponse{}, nil
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestPermissionController_GetAll_InternalError(t *testing.T) {
	mockSvc := &mockPermissionService{
		getAllFn: func(ctx *gin.Context) ([]permission.PermissionResponse, error) {
			return nil, errors.New("error al obtener permisos")
		},
	}

	controller := NewPermissionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/permissions", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
