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

	"simple-arq-golang/cmd/api/domains/invitation"
)

type mockInvitationService struct {
	inviteRunnerFn func(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error)
}

func (m *mockInvitationService) InviteRunner(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
	if m.inviteRunnerFn != nil {
		return m.inviteRunnerFn(ctx, teamID, req)
	}
	return nil, nil
}

func TestInvitationController_InviteRunner_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
			return &invitation.InviteRunnerResponse{Message: "Invitación enviada a " + req.Email}, nil
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"juan@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result invitation.InviteRunnerResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result.Message, "juan@test.com")
}

func TestInvitationController_InviteRunner_TeamNotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"juan@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/999/invite", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_InviteRunner_UserNotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
			return nil, errors.New("no se encontró un usuario con el email proporcionado")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"noexiste@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_InviteRunner_BadRequest(t *testing.T) {
	controller := NewInvitationController(&mockInvitationService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInvitationController_InviteRunner_InvalidID(t *testing.T) {
	controller := NewInvitationController(&mockInvitationService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/abc/invite", strings.NewReader(`{"email":"test@test.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}
