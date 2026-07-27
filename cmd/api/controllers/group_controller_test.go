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

	"simple-arq-golang/cmd/api/domains/group"
)

type mockGroupService struct {
	createFn  func(ctx *gin.Context, req *group.CreateGroupRequest) (*group.GroupResponse, error)
	updateFn  func(ctx *gin.Context, id int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error)
	deleteFn  func(ctx *gin.Context, id int64, userID int64) error
	getByIDFn func(ctx *gin.Context, id int64) (*group.GroupResponse, error)
	getAllFn  func(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error)
}

func (m *mockGroupService) Create(ctx *gin.Context, req *group.CreateGroupRequest) (*group.GroupResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, nil
}

func (m *mockGroupService) Update(ctx *gin.Context, id int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, req)
	}
	return nil, nil
}

func (m *mockGroupService) Delete(ctx *gin.Context, id int64, userID int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id, userID)
	}
	return nil
}

func (m *mockGroupService) GetByID(ctx *gin.Context, id int64) (*group.GroupResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockGroupService) GetAll(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx, teamID, userID)
	}
	return nil, nil
}

func TestGroupController_Create_Success(t *testing.T) {
	mockSvc := &mockGroupService{
		createFn: func(ctx *gin.Context, req *group.CreateGroupRequest) (*group.GroupResponse, error) {
			return &group.GroupResponse{ID: 1, Name: req.Name, TeamID: req.TeamID}, nil
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"Grupo 1","team_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusCreated, response.Code)
	var result group.GroupResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Grupo 1", result.Name)
}

func TestGroupController_Create_TeamNotFound(t *testing.T) {
	mockSvc := &mockGroupService{
		createFn: func(ctx *gin.Context, req *group.CreateGroupRequest) (*group.GroupResponse, error) {
			return nil, errors.New("el equipo no existe")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"Grupo 1","team_id":999}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGroupController_GetByID_Success(t *testing.T) {
	mockSvc := &mockGroupService{
		getByIDFn: func(ctx *gin.Context, id int64) (*group.GroupResponse, error) {
			return &group.GroupResponse{ID: 1, Name: "Grupo 1"}, nil
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestGroupController_Delete_Success(t *testing.T) {
	mockSvc := &mockGroupService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return nil
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1?user_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestGroupController_GetAll_Success(t *testing.T) {
	mockSvc := &mockGroupService{
		getAllFn: func(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
			return []group.GroupResponse{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}, nil
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result []group.GroupResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestGroupController_GetAll_Error(t *testing.T) {
	mockSvc := &mockGroupService{
		getAllFn: func(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
			return nil, errors.New("db error")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestGroupController_GetByID_BadRequest(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_GetByID_NotFound(t *testing.T) {
	mockSvc := &mockGroupService{
		getByIDFn: func(ctx *gin.Context, id int64) (*group.GroupResponse, error) {
			return nil, errors.New("grupo no encontrado")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGroupController_Update_Success(t *testing.T) {
	mockSvc := &mockGroupService{
		updateFn: func(ctx *gin.Context, id int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error) {
			name := ""
			if req.Name != nil {
				name = *req.Name
			}
			return &group.GroupResponse{ID: 1, Name: name}, nil
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"Grupo Actualizado"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/groups/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result group.GroupResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Grupo Actualizado", result.Name)
}

func TestGroupController_Update_BadRequest_InvalidID(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/groups/abc", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_Update_BadRequest_Body(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/groups/1", strings.NewReader(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_Update_NotFound(t *testing.T) {
	mockSvc := &mockGroupService{
		updateFn: func(ctx *gin.Context, id int64, req *group.UpdateGroupRequest) (*group.GroupResponse, error) {
			return nil, errors.New("grupo no encontrado")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"Grupo Actualizado"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/groups/999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Update(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGroupController_Delete_BadRequest(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/abc?user_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_Delete_MissingUserID(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_Delete_InvalidUserID(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1?user_id=abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_Delete_NotFound(t *testing.T) {
	mockSvc := &mockGroupService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return errors.New("grupo no encontrado")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/999?user_id=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGroupController_Delete_Forbidden(t *testing.T) {
	mockSvc := &mockGroupService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return errors.New("solo el entrenador puede eliminar el grupo")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1?user_id=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestGroupController_Create_BadRequest(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_GetAll_TeamIDRequired_WithUserID(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups?team_id=1", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_GetAll_UserIDInvalid(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups?team_id=1&user_id=abc", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_GetAll_TeamIDInvalid(t *testing.T) {
	controller := NewGroupController(&mockGroupService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups?team_id=abc&user_id=1", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupController_GetAll_Forbidden(t *testing.T) {
	mockSvc := &mockGroupService{
		getAllFn: func(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
			return nil, errors.New("el usuario no pertenece a este equipo")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups?team_id=1&user_id=99", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestGroupController_GetAll_TeamNotFound(t *testing.T) {
	mockSvc := &mockGroupService{
		getAllFn: func(ctx *gin.Context, teamID *int64, userID *int64) ([]group.GroupResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewGroupController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups?team_id=999&user_id=1", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}
