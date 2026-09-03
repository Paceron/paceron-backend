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

	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/services"
)

type mockTeamService struct {
	createFn        func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error)
	updateFn        func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error)
	deleteFn        func(ctx *gin.Context, id int64, userID int64) error
	getByIDFn       func(ctx *gin.Context, id int64) (*team.TeamResponse, error)
	getAllFn        func(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error)
	updateAddressFn func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error)
	uploadIconFn    func(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error)
	deleteIconFn    func(ctx *gin.Context, id int64, callerID int64) error
}

func (m *mockTeamService) Create(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, ownerID, req)
	}
	return nil, nil
}

func (m *mockTeamService) Update(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, callerID, req)
	}
	return nil, nil
}

func (m *mockTeamService) Delete(ctx *gin.Context, id int64, userID int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id, userID)
	}
	return nil
}

func (m *mockTeamService) GetByID(ctx *gin.Context, id int64) (*team.TeamResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTeamService) GetAll(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx, ownerID, memberID)
	}
	return nil, nil
}

func (m *mockTeamService) UpdateAddress(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
	if m.updateAddressFn != nil {
		return m.updateAddressFn(ctx, id, callerID, req)
	}
	return nil, nil
}

func (m *mockTeamService) UploadIcon(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error) {
	if m.uploadIconFn != nil {
		return m.uploadIconFn(ctx, id, callerID, content)
	}
	return nil, nil
}

func (m *mockTeamService) DeleteIcon(ctx *gin.Context, id int64, callerID int64) error {
	if m.deleteIconFn != nil {
		return m.deleteIconFn(ctx, id, callerID)
	}
	return nil
}

type mockTeamDelegate struct {
	createTeamFn func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error)
}

func (m *mockTeamDelegate) CreateTeam(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
	if m.createTeamFn != nil {
		return m.createTeamFn(ctx, ownerID, req)
	}
	return nil, nil
}

func TestTeamController_Create_Success(t *testing.T) {
	mockSvc := &mockTeamService{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return &team.TeamResponse{ID: 1, Name: req.Name, MaxMembers: req.MaxMembers, OwnerID: ownerID, Status: "active"}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"name":"Alpha","max_members":20}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.Create(c)

	assert.Equal(t, http.StatusCreated, response.Code)
	var result team.TeamResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Alpha", result.Name)
	assert.Equal(t, int64(1), result.OwnerID)
}

func TestTeamController_Create_BadRequest(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_Create_OwnerNotFound(t *testing.T) {
	mockSvc := &mockTeamService{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return nil, errors.New("el usuario owner no existe")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"name":"Alpha","max_members":20}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 999)

	controller.Create(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamController_Create_OwnerNotEntrenador(t *testing.T) {
	mockSvc := &mockTeamService{
		createFn: func(ctx *gin.Context, ownerID int64, req *team.CreateTeamRequest) (*team.TeamResponse, error) {
			return nil, errors.New("el owner debe tener el rol 'entrenador'")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"name":"Alpha","max_members":20}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)

	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_GetByID_Success(t *testing.T) {
	mockSvc := &mockTeamService{
		getByIDFn: func(ctx *gin.Context, id int64) (*team.TeamResponse, error) {
			return &team.TeamResponse{ID: 1, Name: "Alpha"}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result team.TeamResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Alpha", result.Name)
}

func TestTeamController_GetByID_NotFound(t *testing.T) {
	mockSvc := &mockTeamService{
		getByIDFn: func(ctx *gin.Context, id int64) (*team.TeamResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamController_GetByID_InternalError(t *testing.T) {
	mockSvc := &mockTeamService{
		getByIDFn: func(ctx *gin.Context, id int64) (*team.TeamResponse, error) {
			return nil, errors.New("db connection failed")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetByID(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestTeamController_Delete_Success(t *testing.T) {
	mockSvc := &mockTeamService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Delete(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestTeamController_Delete_BadRequest_InvalidID(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_Delete_NotFound(t *testing.T) {
	mockSvc := &mockTeamService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 1)

	controller.Delete(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamController_Delete_Forbidden(t *testing.T) {
	mockSvc := &mockTeamService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return errors.New("solo el entrenador puede eliminar el equipo")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.Delete(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamController_Delete_HasMembers(t *testing.T) {
	mockSvc := &mockTeamService{
		deleteFn: func(ctx *gin.Context, id int64, userID int64) error {
			return errors.New("no se puede eliminar un equipo con miembros activos")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Delete(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_Update_Success(t *testing.T) {
	mockSvc := &mockTeamService{
		updateFn: func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
			name := ""
			if req.Name != nil {
				name = *req.Name
			}
			return &team.TeamResponse{ID: 1, Name: name}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"name":"Beta"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result team.TeamResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Beta", result.Name)
}

func TestTeamController_Update_BadRequest_InvalidID(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/abc", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_Update_BadRequest_Body(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/1", strings.NewReader(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_Update_NotFound(t *testing.T) {
	mockSvc := &mockTeamService{
		updateFn: func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"name":"Beta"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamController_Update_Forbidden(t *testing.T) {
	mockSvc := &mockTeamService{
		updateFn: func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamRequest) (*team.TeamResponse, error) {
			return nil, errors.New("solo el entrenador puede actualizar el equipo")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"name":"Beta"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.Update(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamController_GetAll_Success(t *testing.T) {
	mockSvc := &mockTeamService{
		getAllFn: func(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
			return []team.TeamResponse{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result []team.TeamResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestTeamController_GetAll_Error(t *testing.T) {
	mockSvc := &mockTeamService{
		getAllFn: func(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
			return nil, errors.New("db error")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestTeamController_GetAll_FilterByOwnerID(t *testing.T) {
	var gotOwnerID, gotMemberID *int64
	mockSvc := &mockTeamService{
		getAllFn: func(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
			gotOwnerID = ownerID
			gotMemberID = memberID
			return []team.TeamResponse{{ID: 1, Name: "A"}}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams?owner_id=5", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.NotNil(t, gotOwnerID)
	assert.Equal(t, int64(5), *gotOwnerID)
	assert.Nil(t, gotMemberID)
}

func TestTeamController_GetAll_FilterByMemberID(t *testing.T) {
	var gotMemberID *int64
	mockSvc := &mockTeamService{
		getAllFn: func(ctx *gin.Context, ownerID *int64, memberID *int64) ([]team.TeamResponse, error) {
			gotMemberID = memberID
			return []team.TeamResponse{{ID: 2, Name: "B"}}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams?member_id=7", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.NotNil(t, gotMemberID)
	assert.Equal(t, int64(7), *gotMemberID)
}

func TestTeamController_GetAll_InvalidOwnerID(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams?owner_id=abc", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_GetAll_InvalidMemberID(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams?member_id=abc", nil)

	controller.GetAll(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_UpdateAddress_Success(t *testing.T) {
	mockSvc := &mockTeamService{
		updateAddressFn: func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
			return &team.TeamResponse{ID: 1, Country: req.Country, City: req.City}, nil
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"country":"Argentina","city":"Córdoba","street":"Av. General Paz","number":"1234"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/1/address", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.UpdateAddress(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result team.TeamResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Argentina", result.Country)
}

func TestTeamController_UpdateAddress_BadRequest_InvalidID(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/abc/address", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.UpdateAddress(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_UpdateAddress_BadRequest_Body(t *testing.T) {
	controller := NewTeamController(&mockTeamService{}, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/1/address", strings.NewReader(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.UpdateAddress(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_UpdateAddress_NotFound(t *testing.T) {
	mockSvc := &mockTeamService{
		updateAddressFn: func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"country":"Argentina","city":"Córdoba","street":"Av. General Paz","number":"1234"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/999/address", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 1)

	controller.UpdateAddress(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamController_UpdateAddress_Forbidden(t *testing.T) {
	mockSvc := &mockTeamService{
		updateAddressFn: func(ctx *gin.Context, id int64, callerID int64, req *team.UpdateTeamAddressRequest) (*team.TeamResponse, error) {
			return nil, errors.New("solo el entrenador puede actualizar el equipo")
		},
	}

	controller := NewTeamController(mockSvc, &mockTeamDelegate{createTeamFn: mockSvc.createFn})
	response := httptest.NewRecorder()
	body := `{"country":"Argentina"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/teams/1/address", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.UpdateAddress(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamController_UploadIcon_Success(t *testing.T) {
	expectedURL := "https://bucket.example.com/teams/team-1-icon.png?v=123"
	mockSvc := &mockTeamService{
		uploadIconFn: func(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error) {
			assert.Equal(t, int64(1), id)
			assert.Equal(t, int64(1), callerID)
			return &expectedURL, nil
		},
	}
	controller := NewTeamController(mockSvc, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = newMultipartPhotoRequest(t, http.MethodPut, "/api/v1/teams/1/icon", "icon.png", validPNGContentForTest)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.UploadIcon(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result map[string]string
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, expectedURL, result["icon_url"])
}

func TestTeamController_UploadIcon_Forbidden(t *testing.T) {
	mockSvc := &mockTeamService{
		uploadIconFn: func(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error) {
			return nil, errors.New("solo el entrenador dueño del equipo puede cambiar el ícono")
		},
	}
	controller := NewTeamController(mockSvc, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = newMultipartPhotoRequest(t, http.MethodPut, "/api/v1/teams/1/icon", "icon.png", validPNGContentForTest)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.UploadIcon(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamController_UploadIcon_InvalidType(t *testing.T) {
	mockSvc := &mockTeamService{
		uploadIconFn: func(ctx *gin.Context, id int64, callerID int64, content []byte) (*string, error) {
			return nil, services.ErrPhotoInvalidType
		},
	}
	controller := NewTeamController(mockSvc, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = newMultipartPhotoRequest(t, http.MethodPut, "/api/v1/teams/1/icon", "icon.txt", []byte("not an image"))
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.UploadIcon(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamController_DeleteIcon_Success(t *testing.T) {
	deleteCalled := false
	mockSvc := &mockTeamService{
		deleteIconFn: func(ctx *gin.Context, id int64, callerID int64) error {
			deleteCalled = true
			assert.Equal(t, int64(1), id)
			assert.Equal(t, int64(1), callerID)
			return nil
		},
	}
	controller := NewTeamController(mockSvc, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1/icon", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.DeleteIcon(c)
	c.Writer.WriteHeaderNow()

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.True(t, deleteCalled)
}

func TestTeamController_DeleteIcon_Forbidden(t *testing.T) {
	mockSvc := &mockTeamService{
		deleteIconFn: func(ctx *gin.Context, id int64, callerID int64) error {
			return errors.New("solo el entrenador dueño del equipo puede cambiar el ícono")
		},
	}
	controller := NewTeamController(mockSvc, &mockTeamDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1/icon", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.DeleteIcon(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}
