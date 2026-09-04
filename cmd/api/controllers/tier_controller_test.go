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
	"simple-arq-golang/cmd/api/domains/tier"
)

type mockTierService struct {
	createFn func(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error)
	updateFn func(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error)
	deleteFn func(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error)
	getByIDFn  func(ctx *gin.Context, id int64) (*tier.TierResponse, error)
	getByNameFn func(ctx *gin.Context, name string) (*tier.TierResponse, error)
	getAllFn    func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error)
}

func (m *mockTierService) Create(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, nil
}

func (m *mockTierService) Update(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, req)
	}
	return nil, nil
}

func (m *mockTierService) Delete(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error) {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTierService) GetByID(ctx *gin.Context, id int64) (*tier.TierResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTierService) GetByName(ctx *gin.Context, name string) (*tier.TierResponse, error) {
	if m.getByNameFn != nil {
		return m.getByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockTierService) GetAll(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx, roleID)
	}
	return nil, nil
}

func TestTierController_Create_Success(t *testing.T) {
	mockSvc := &mockTierService{
		createFn: func(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
			return &tier.TierResponse{
				ID:       1,
				Name:     req.Name,
				RoleID:   req.RoleID,
				RoleName: "corredor",
			}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"base","role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result tier.TierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "base", result.Name)
	assert.Equal(t, int64(1), result.RoleID)
}

func TestTierController_Create_InvalidBody(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Cuerpo de solicitud inválido", result.Message)
}

func TestTierController_Create_RoleNotFound(t *testing.T) {
	mockSvc := &mockTierService{
		createFn: func(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"base","role_id":999}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestTierController_Update_Success(t *testing.T) {
	mockSvc := &mockTierService{
		updateFn: func(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
			return &tier.TierResponse{
				ID:   id,
				Name: "premium",
			}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"premium"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tier.TierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "premium", result.Name)
}

func TestTierController_Update_NotFound(t *testing.T) {
	mockSvc := &mockTierService{
		updateFn: func(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"premium"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestTierController_Delete_Success(t *testing.T) {
	mockSvc := &mockTierService{
		deleteFn: func(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error) {
			return &tier.DeleteTierResponse{
				Message: "Tier eliminado correctamente",
			}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tier.DeleteTierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Tier eliminado correctamente", result.Message)
}

func TestTierController_Delete_NotFound(t *testing.T) {
	mockSvc := &mockTierService{
		deleteFn: func(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestTierController_Update_InvalidID(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	body := `{"name":"premium"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/abc", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_Delete_InvalidID(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_Create_DuplicateName(t *testing.T) {
	mockSvc := &mockTierService{
		createFn: func(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("ya existe un tier con ese nombre para este rol")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"base","role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestTierController_Create_EmptyName(t *testing.T) {
	mockSvc := &mockTierService{
		createFn: func(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("el nombre es requerido")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"test","role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestTierController_Create_InternalError(t *testing.T) {
	mockSvc := &mockTierService{
		createFn: func(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("error al crear tier")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"base","role_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestTierController_Update_InternalError(t *testing.T) {
	mockSvc := &mockTierService{
		updateFn: func(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("error al actualizar tier")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"premium"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestTierController_Update_InvalidBody(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_Update_EmptyName(t *testing.T) {
	mockSvc := &mockTierService{
		updateFn: func(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("el nombre no puede estar vacío")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":""}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestTierController_Update_DuplicateName(t *testing.T) {
	mockSvc := &mockTierService{
		updateFn: func(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
			return nil, errors.New("ya existe un tier con ese nombre para este rol")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"existing"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
}

func TestTierController_Delete_InternalError(t *testing.T) {
	mockSvc := &mockTierService{
		deleteFn: func(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error) {
			return nil, errors.New("error al eliminar tier")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}

func TestTierController_Update_InvalidIDOverflow(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	body := `{"name":"premium"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/tiers/99999999999999999999999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_Delete_InvalidIDOverflow(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/tiers/99999999999999999999999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99999999999999999999999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_Create_InvalidBodyEmptyObject(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	body := `{"name":"base"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/tiers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_GetByID_Success(t *testing.T) {
	mockSvc := &mockTierService{
		getByIDFn: func(ctx *gin.Context, id int64) (*tier.TierResponse, error) {
			return &tier.TierResponse{ID: 1, Name: "base", RoleID: 1, RoleName: "corredor"}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tier.TierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "base", result.Name)
}

func TestTierController_GetByID_NotFound(t *testing.T) {
	mockSvc := &mockTierService{
		getByIDFn: func(ctx *gin.Context, id int64) (*tier.TierResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTierController_GetByID_InvalidID(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_GetByName_Success(t *testing.T) {
	mockSvc := &mockTierService{
		getByNameFn: func(ctx *gin.Context, name string) (*tier.TierResponse, error) {
			return &tier.TierResponse{ID: 1, Name: name, RoleID: 1, RoleName: "corredor"}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers/by-name?name=base", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result tier.TierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "base", result.Name)
}

func TestTierController_GetByName_NotFound(t *testing.T) {
	mockSvc := &mockTierService{
		getByNameFn: func(ctx *gin.Context, name string) (*tier.TierResponse, error) {
			return nil, errors.New("tier no encontrado")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers/by-name?name=nonexistent", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTierController_GetByName_EmptyName(t *testing.T) {
	controller := NewTierController(&mockTierService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers/by-name", nil)

	controller.GetByName(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_GetAll_Success(t *testing.T) {
	mockSvc := &mockTierService{
		getAllFn: func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
			return []tier.TierResponse{
				{ID: 1, Name: "base"},
				{ID: 2, Name: "premium"},
			}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result []tier.TierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestTierController_GetAll_Empty(t *testing.T) {
	mockSvc := &mockTierService{
		getAllFn: func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
			return []tier.TierResponse{}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestTierController_GetAll_InternalError(t *testing.T) {
	mockSvc := &mockTierService{
		getAllFn: func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
			return nil, errors.New("error al obtener tiers")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestTierController_GetAll_WithRoleID(t *testing.T) {
	var gotRoleID *int64
	mockSvc := &mockTierService{
		getAllFn: func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
			gotRoleID = roleID
			return []tier.TierResponse{{ID: 3, Name: "base entrenador", RoleID: 2}}, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers?role_id=2", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.NotNil(t, gotRoleID)
	assert.Equal(t, int64(2), *gotRoleID)

	var result []tier.TierResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 1)
}

func TestTierController_GetAll_RoleNotFound(t *testing.T) {
	mockSvc := &mockTierService{
		getAllFn: func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
			return nil, errors.New("rol no encontrado")
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers?role_id=99", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTierController_GetAll_InvalidRoleID(t *testing.T) {
	called := false
	mockSvc := &mockTierService{
		getAllFn: func(ctx *gin.Context, roleID *int64) ([]tier.TierResponse, error) {
			called = true
			return nil, nil
		},
	}

	controller := NewTierController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/tiers?role_id=abc", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.False(t, called, "no debe llamar al service con role_id inválido")
}
