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

	"simple-arq-golang/cmd/api/domains/groupuser"
)

type mockGroupUserService struct {
	addUserFn         func(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error)
	removeUserFn      func(ctx *gin.Context, groupID, callerID, targetUserID int64) error
	getUsersByGroupFn func(ctx *gin.Context, groupID int64) ([]groupuser.GroupUserResponse, error)
}

func (m *mockGroupUserService) AddUser(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error) {
	if m.addUserFn != nil {
		return m.addUserFn(ctx, teamID, groupID, callerID, req)
	}
	return nil, nil
}

func (m *mockGroupUserService) RemoveUser(ctx *gin.Context, groupID, callerID, targetUserID int64) error {
	if m.removeUserFn != nil {
		return m.removeUserFn(ctx, groupID, callerID, targetUserID)
	}
	return nil
}

func (m *mockGroupUserService) GetUsersByGroup(ctx *gin.Context, groupID int64) ([]groupuser.GroupUserResponse, error) {
	if m.getUsersByGroupFn != nil {
		return m.getUsersByGroupFn(ctx, groupID)
	}
	return nil, nil
}

func TestGroupUserController_AddUser_Success(t *testing.T) {
	mockSvc := &mockGroupUserService{
		addUserFn: func(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error) {
			return &groupuser.GroupUserResponse{ID: 1, GroupID: groupID, UserID: req.UserID}, nil
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/groups/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "group_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.AddUser(c)

	assert.Equal(t, http.StatusCreated, response.Code)
	var result groupuser.GroupUserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
}

func TestGroupUserController_AddUser_Conflict(t *testing.T) {
	mockSvc := &mockGroupUserService{
		addUserFn: func(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error) {
			return nil, errors.New("el usuario ya pertenece a este grupo")
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/groups/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "group_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.AddUser(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestGroupUserController_AddUser_NotFound(t *testing.T) {
	mockSvc := &mockGroupUserService{
		addUserFn: func(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error) {
			return nil, errors.New("grupo no encontrado en este equipo")
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/groups/999/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "group_id", Value: "999"}}
	setAuthUserID(c, 1)

	controller.AddUser(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGroupUserController_AddUser_Forbidden(t *testing.T) {
	mockSvc := &mockGroupUserService{
		addUserFn: func(ctx *gin.Context, teamID, groupID int64, callerID int64, req *groupuser.AddGroupUserRequest) (*groupuser.GroupUserResponse, error) {
			return nil, errors.New("solo el entrenador puede agregar usuarios al grupo")
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/groups/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "group_id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.AddUser(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestGroupUserController_RemoveUser_Success(t *testing.T) {
	mockSvc := &mockGroupUserService{
		removeUserFn: func(ctx *gin.Context, groupID, callerID, targetUserID int64) error {
			return nil
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1/users/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestGroupUserController_RemoveUser_NotFound(t *testing.T) {
	mockSvc := &mockGroupUserService{
		removeUserFn: func(ctx *gin.Context, groupID, callerID, targetUserID int64) error {
			return errors.New("el usuario no pertenece a este grupo")
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1/users/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "999"}}
	setAuthUserID(c, 1)

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGroupUserController_RemoveUser_Forbidden(t *testing.T) {
	mockSvc := &mockGroupUserService{
		removeUserFn: func(ctx *gin.Context, groupID, callerID, targetUserID int64) error {
			return errors.New("solo el entrenador puede quitar a otro usuario del grupo")
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1/users/2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "2"}}
	setAuthUserID(c, 3)

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestGroupUserController_AddUser_BadRequest_InvalidGroupID(t *testing.T) {
	controller := NewGroupUserController(&mockGroupUserService{})
	response := httptest.NewRecorder()
	body := `{"user_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/groups/abc/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "group_id", Value: "abc"}}

	controller.AddUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupUserController_AddUser_BadRequest_InvalidTeamID(t *testing.T) {
	controller := NewGroupUserController(&mockGroupUserService{})
	response := httptest.NewRecorder()
	body := `{"user_id":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/abc/groups/1/users", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}, {Key: "group_id", Value: "1"}}

	controller.AddUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupUserController_AddUser_BadRequest_Body(t *testing.T) {
	controller := NewGroupUserController(&mockGroupUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/groups/1/users", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "group_id", Value: "1"}}

	controller.AddUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupUserController_RemoveUser_BadRequest_InvalidGroupID(t *testing.T) {
	controller := NewGroupUserController(&mockGroupUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/abc/users/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}, {Key: "user_id", Value: "1"}}

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupUserController_RemoveUser_BadRequest_InvalidUserID(t *testing.T) {
	controller := NewGroupUserController(&mockGroupUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/groups/1/users/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "user_id", Value: "abc"}}

	controller.RemoveUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupUserController_GetUsersByGroup_Success(t *testing.T) {
	mockSvc := &mockGroupUserService{
		getUsersByGroupFn: func(ctx *gin.Context, groupID int64) ([]groupuser.GroupUserResponse, error) {
			return []groupuser.GroupUserResponse{
				{ID: 1, GroupID: 1, UserID: 10},
				{ID: 2, GroupID: 1, UserID: 20},
			}, nil
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups/1/users", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.GetUsersByGroup(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result []groupuser.GroupUserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result, 2)
}

func TestGroupUserController_GetUsersByGroup_BadRequest(t *testing.T) {
	controller := NewGroupUserController(&mockGroupUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups/abc/users", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetUsersByGroup(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGroupUserController_GetUsersByGroup_NotFound(t *testing.T) {
	mockSvc := &mockGroupUserService{
		getUsersByGroupFn: func(ctx *gin.Context, groupID int64) ([]groupuser.GroupUserResponse, error) {
			return nil, errors.New("grupo no encontrado")
		},
	}

	controller := NewGroupUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/groups/999/users", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.GetUsersByGroup(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}
