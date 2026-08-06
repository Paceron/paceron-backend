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

	"simple-arq-golang/cmd/api/domains/teamuser"
)

type mockTeamUserService struct {
	addUserFn        func(ctx *gin.Context, teamID int64, callerID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error)
	removeUserFn     func(ctx *gin.Context, teamID, callerID, targetUserID int64) error
	getUsersByTeamFn func(ctx *gin.Context, teamID int64) ([]teamuser.TeamUserResponse, error)
}

func (m *mockTeamUserService) AddUser(ctx *gin.Context, teamID int64, callerID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error) {
	if m.addUserFn != nil {
		return m.addUserFn(ctx, teamID, callerID, req)
	}
	return nil, nil
}

func (m *mockTeamUserService) RemoveUser(ctx *gin.Context, teamID, callerID, targetUserID int64) error {
	if m.removeUserFn != nil {
		return m.removeUserFn(ctx, teamID, callerID, targetUserID)
	}
	return nil
}

func (m *mockTeamUserService) GetUsersByTeam(ctx *gin.Context, teamID int64) ([]teamuser.TeamUserResponse, error) {
	if m.getUsersByTeamFn != nil {
		return m.getUsersByTeamFn(ctx, teamID)
	}
	return nil, nil
}

func TestTeamUserController_AddUser_Success(t *testing.T) {
	mockSvc := &mockTeamUserService{
		addUserFn: func(ctx *gin.Context, teamID int64, callerID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error) {
			return &teamuser.TeamUserResponse{ID: 1, TeamID: teamID, UserID: req.UserID, RoleInTeam: req.RoleInTeam, Status: "active"}, nil
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1,"role_in_team":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.AddUser(c)

	assert.Equal(t, http.StatusCreated, response.Code)
	var result teamuser.TeamUserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
}

func TestTeamUserController_AddUser_Conflict(t *testing.T) {
	mockSvc := &mockTeamUserService{
		addUserFn: func(ctx *gin.Context, teamID int64, callerID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error) {
			return nil, errors.New("el usuario ya pertenece a este equipo")
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1,"role_in_team":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.AddUser(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestTeamUserController_AddUser_Forbidden(t *testing.T) {
	mockSvc := &mockTeamUserService{
		addUserFn: func(ctx *gin.Context, teamID int64, callerID int64, req *teamuser.AddTeamUserRequest) (*teamuser.TeamUserResponse, error) {
			return nil, errors.New("solo el entrenador puede agregar usuarios al equipo")
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1,"role_in_team":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.AddUser(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamUserController_RemoveUser_Success(t *testing.T) {
	mockSvc := &mockTeamUserService{
		removeUserFn: func(ctx *gin.Context, teamID, callerID, targetUserID int64) error {
			return nil
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1/users/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestTeamUserController_RemoveUser_NotFound(t *testing.T) {
	mockSvc := &mockTeamUserService{
		removeUserFn: func(ctx *gin.Context, teamID, callerID, targetUserID int64) error {
			return errors.New("el usuario no pertenece a este equipo")
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1/users/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "999"}}
	setAuthUserID(c, 1)

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamUserController_RemoveUser_Forbidden(t *testing.T) {
	mockSvc := &mockTeamUserService{
		removeUserFn: func(ctx *gin.Context, teamID, callerID, targetUserID int64) error {
			return errors.New("solo el entrenador puede quitar a otro usuario del equipo")
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1/users/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "2"}}
	setAuthUserID(c, 3)

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamUserController_AddUser_BadRequest_InvalidTeamID(t *testing.T) {
	controller := NewTeamUserController(&mockTeamUserService{})
	response := httptest.NewRecorder()
	body := `{"user_id":1,"role_in_team":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/abc/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.AddUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamUserController_AddUser_BadRequest_Body(t *testing.T) {
	controller := NewTeamUserController(&mockTeamUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/users", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.AddUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamUserController_RemoveUser_BadRequest_InvalidTeamID(t *testing.T) {
	controller := NewTeamUserController(&mockTeamUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/abc/users/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}, {Key: "user_id", Value: "1"}}

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamUserController_RemoveUser_BadRequest_InvalidUserID(t *testing.T) {
	controller := NewTeamUserController(&mockTeamUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/teams/1/users/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "abc"}}

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamUserController_GetUsersByTeam_Success(t *testing.T) {
	mockSvc := &mockTeamUserService{
		getUsersByTeamFn: func(ctx *gin.Context, teamID int64) ([]teamuser.TeamUserResponse, error) {
			return []teamuser.TeamUserResponse{
				{ID: 1, TeamID: 1, UserID: 10, RoleInTeam: "entrenador", Status: "active"},
				{ID: 2, TeamID: 1, UserID: 20, RoleInTeam: "corredor", Status: "active"},
			}, nil
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/1/users", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetUsersByTeam(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result []teamuser.TeamUserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestTeamUserController_GetUsersByTeam_BadRequest(t *testing.T) {
	controller := NewTeamUserController(&mockTeamUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/abc/users", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetUsersByTeam(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamUserController_GetUsersByTeam_NotFound(t *testing.T) {
	mockSvc := &mockTeamUserService{
		getUsersByTeamFn: func(ctx *gin.Context, teamID int64) ([]teamuser.TeamUserResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/999/users", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetUsersByTeam(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}
