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
	inviteRunnerFn                  func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error)
	listPendingInvitationsFn        func(ctx *gin.Context, teamID int64, callerID int64) ([]invitation.InvitationResponse, error)
	listPendingInvitationsForUserFn func(ctx *gin.Context, userID int64) ([]invitation.InvitationResponse, error)
	getInvitationDetailFn           func(ctx *gin.Context, invitationID, userID int64) (*invitation.InvitationResponse, error)
	acceptInvitationFn              func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error)
	rejectInvitationFn              func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error)
}

func (m *mockInvitationService) InviteRunner(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
	if m.inviteRunnerFn != nil {
		return m.inviteRunnerFn(ctx, teamID, callerID, req)
	}
	return nil, nil
}

func (m *mockInvitationService) ListPendingInvitations(ctx *gin.Context, teamID int64, callerID int64) ([]invitation.InvitationResponse, error) {
	if m.listPendingInvitationsFn != nil {
		return m.listPendingInvitationsFn(ctx, teamID, callerID)
	}
	return nil, nil
}

func (m *mockInvitationService) ListPendingInvitationsForUser(ctx *gin.Context, userID int64) ([]invitation.InvitationResponse, error) {
	if m.listPendingInvitationsForUserFn != nil {
		return m.listPendingInvitationsForUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockInvitationService) GetInvitationDetail(ctx *gin.Context, invitationID, userID int64) (*invitation.InvitationResponse, error) {
	if m.getInvitationDetailFn != nil {
		return m.getInvitationDetailFn(ctx, invitationID, userID)
	}
	return nil, nil
}

func (m *mockInvitationService) AcceptInvitation(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
	if m.acceptInvitationFn != nil {
		return m.acceptInvitationFn(ctx, invitationID, userID)
	}
	return nil, nil
}

func (m *mockInvitationService) RejectInvitation(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
	if m.rejectInvitationFn != nil {
		return m.rejectInvitationFn(ctx, invitationID, userID)
	}
	return nil, nil
}

func TestInvitationController_InviteRunner_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
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
	setAuthUserID(c, 5)

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result invitation.InviteRunnerResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result.Message, "juan@test.com")
}

func TestInvitationController_InviteRunner_TeamNotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
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
	setAuthUserID(c, 5)

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_InviteRunner_UserNotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
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
	setAuthUserID(c, 5)

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_InviteRunner_Forbidden(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
			return nil, errors.New("solo el entrenador puede invitar usuarios al equipo")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"juan@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
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

func TestInvitationController_InviteRunner_DuplicateInvitation(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
			return nil, errors.New("ya existe una invitación pendiente para este usuario en este equipo")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"juan@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 5)

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestInvitationController_ListPendingInvitations_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		listPendingInvitationsFn: func(ctx *gin.Context, teamID int64, callerID int64) ([]invitation.InvitationResponse, error) {
			return []invitation.InvitationResponse{{ID: 1, TeamID: teamID}}, nil
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/1/invitations", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 5)

	controller.ListPendingInvitations(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestInvitationController_ListPendingInvitations_TeamNotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		listPendingInvitationsFn: func(ctx *gin.Context, teamID int64, callerID int64) ([]invitation.InvitationResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/999/invitations", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 5)

	controller.ListPendingInvitations(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_ListPendingInvitations_Forbidden(t *testing.T) {
	mockSvc := &mockInvitationService{
		listPendingInvitationsFn: func(ctx *gin.Context, teamID int64, callerID int64) ([]invitation.InvitationResponse, error) {
			return nil, errors.New("solo el entrenador puede ver las invitaciones del equipo")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/1/invitations", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.ListPendingInvitations(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestInvitationController_ListPendingInvitations_InvalidID(t *testing.T) {
	controller := NewInvitationController(&mockInvitationService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/teams/abc/invitations", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.ListPendingInvitations(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInvitationController_AcceptInvitation_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		acceptInvitationFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
			return &invitation.RespondInvitationResponse{Message: "Invitación aceptada"}, nil
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/1/accept", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.AcceptInvitation(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestInvitationController_AcceptInvitation_NotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		acceptInvitationFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
			return nil, errors.New("invitación no encontrada")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/999/accept", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 2)

	controller.AcceptInvitation(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_AcceptInvitation_WrongUser(t *testing.T) {
	mockSvc := &mockInvitationService{
		acceptInvitationFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
			return nil, errors.New("la invitación no pertenece a este usuario")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/1/accept", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 999)

	controller.AcceptInvitation(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestInvitationController_AcceptInvitation_InvalidID(t *testing.T) {
	controller := NewInvitationController(&mockInvitationService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/abc/accept", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.AcceptInvitation(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInvitationController_RejectInvitation_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		rejectInvitationFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
			return &invitation.RespondInvitationResponse{Message: "Invitación rechazada"}, nil
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/1/reject", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.RejectInvitation(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestInvitationController_RejectInvitation_NotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		rejectInvitationFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
			return nil, errors.New("invitación no encontrada")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/999/reject", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 2)

	controller.RejectInvitation(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_RejectInvitation_AlreadyResponded(t *testing.T) {
	mockSvc := &mockInvitationService{
		rejectInvitationFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error) {
			return nil, errors.New("la invitación ya fue respondida")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/invitations/1/reject", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.RejectInvitation(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestInvitationController_ListMyInvitations_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		listPendingInvitationsForUserFn: func(ctx *gin.Context, userID int64) ([]invitation.InvitationResponse, error) {
			return []invitation.InvitationResponse{{ID: 1, TeamID: 2}}, nil
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/invitations", nil)
	setAuthUserID(c, 1)

	controller.ListMyInvitations(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestInvitationController_GetInvitationByID_Success(t *testing.T) {
	mockSvc := &mockInvitationService{
		getInvitationDetailFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.InvitationResponse, error) {
			return &invitation.InvitationResponse{ID: invitationID, TeamID: 2}, nil
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/invitations/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.GetInvitationByID(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestInvitationController_GetInvitationByID_NotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		getInvitationDetailFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.InvitationResponse, error) {
			return nil, errors.New("invitación no encontrada")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/invitations/999", nil)
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 2)

	controller.GetInvitationByID(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestInvitationController_GetInvitationByID_WrongUser(t *testing.T) {
	mockSvc := &mockInvitationService{
		getInvitationDetailFn: func(ctx *gin.Context, invitationID, userID int64) (*invitation.InvitationResponse, error) {
			return nil, errors.New("la invitación no pertenece a este usuario")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/invitations/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 999)

	controller.GetInvitationByID(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestInvitationController_GetInvitationByID_InvalidID(t *testing.T) {
	controller := NewInvitationController(&mockInvitationService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/invitations/abc", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.GetInvitationByID(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInvitationController_InviteRunner_GroupNotFound(t *testing.T) {
	mockSvc := &mockInvitationService{
		inviteRunnerFn: func(ctx *gin.Context, teamID int64, callerID int64, req *invitation.InviteRunnerRequest) (*invitation.InviteRunnerResponse, error) {
			return nil, errors.New("el grupo no existe en este equipo")
		},
	}

	controller := NewInvitationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/teams/1/invite", strings.NewReader(`{"email":"juan@test.com","group_id":99}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 5)

	controller.InviteRunner(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}
