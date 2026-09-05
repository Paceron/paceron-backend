package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/joinrequest"
	"simple-arq-golang/cmd/api/services"
)

type mockJoinRequestService struct {
	createFn       func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error)
	cancelFn       func(ctx *gin.Context, requestID, callerID int64) error
	acceptFn       func(ctx *gin.Context, requestID, callerID int64) error
	rejectFn       func(ctx *gin.Context, requestID, callerID int64) error
	listMineFn     func(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error)
	listByTeamFn   func(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error)
	pendingCountFn func(ctx *gin.Context, ownerID int64) (int64, error)
}

func (m *mockJoinRequestService) Create(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, teamID, runnerID)
	}
	return nil, nil
}

func (m *mockJoinRequestService) Cancel(ctx *gin.Context, requestID, callerID int64) error {
	if m.cancelFn != nil {
		return m.cancelFn(ctx, requestID, callerID)
	}
	return nil
}

func (m *mockJoinRequestService) Accept(ctx *gin.Context, requestID, callerID int64) error {
	if m.acceptFn != nil {
		return m.acceptFn(ctx, requestID, callerID)
	}
	return nil
}

func (m *mockJoinRequestService) Reject(ctx *gin.Context, requestID, callerID int64) error {
	if m.rejectFn != nil {
		return m.rejectFn(ctx, requestID, callerID)
	}
	return nil
}

func (m *mockJoinRequestService) ListMine(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error) {
	if m.listMineFn != nil {
		return m.listMineFn(ctx, runnerID)
	}
	return nil, nil
}

func (m *mockJoinRequestService) ListByTeam(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
	if m.listByTeamFn != nil {
		return m.listByTeamFn(ctx, teamID, callerID)
	}
	return nil, nil
}

func (m *mockJoinRequestService) PendingCount(ctx *gin.Context, ownerID int64) (int64, error) {
	if m.pendingCountFn != nil {
		return m.pendingCountFn(ctx, ownerID)
	}
	return 0, nil
}

func TestJoinRequestController_Create_Success(t *testing.T) {
	svc := &mockJoinRequestService{createFn: func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
		return &joinrequest.JoinRequestResponse{ID: 1, TeamID: teamID, RunnerID: runnerID}, nil
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/teams/5/join-requests", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setAuthUserID(c, 7)

	ctrl.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestJoinRequestController_Create_TeamNotPublic(t *testing.T) {
	svc := &mockJoinRequestService{createFn: func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
		return nil, services.ErrTeamNotPublic
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/teams/5/join-requests", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setAuthUserID(c, 7)

	ctrl.Create(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestJoinRequestController_Create_TeamFull(t *testing.T) {
	svc := &mockJoinRequestService{createFn: func(ctx *gin.Context, teamID, runnerID int64) (*joinrequest.JoinRequestResponse, error) {
		return nil, services.ErrTeamFull
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/teams/5/join-requests", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setAuthUserID(c, 7)

	ctrl.Create(c)

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "TEAM_FULL")
}

func TestJoinRequestController_Cancel_NotFound(t *testing.T) {
	svc := &mockJoinRequestService{cancelFn: func(ctx *gin.Context, requestID, callerID int64) error {
		return services.ErrJoinRequestNotFound
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/join-requests/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setAuthUserID(c, 7)

	ctrl.Cancel(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestJoinRequestController_Accept_Success(t *testing.T) {
	svc := &mockJoinRequestService{acceptFn: func(ctx *gin.Context, requestID, callerID int64) error { return nil }}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/join-requests/1/accept", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	ctrl.Accept(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJoinRequestController_Accept_Forbidden(t *testing.T) {
	svc := &mockJoinRequestService{acceptFn: func(ctx *gin.Context, requestID, callerID int64) error {
		return services.ErrJoinRequestForbidden
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/join-requests/1/accept", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	ctrl.Accept(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestJoinRequestController_Reject_Success(t *testing.T) {
	svc := &mockJoinRequestService{rejectFn: func(ctx *gin.Context, requestID, callerID int64) error { return nil }}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/join-requests/1/reject", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	ctrl.Reject(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Solicitud rechazada")
}

func TestJoinRequestController_Reject_NotFound(t *testing.T) {
	svc := &mockJoinRequestService{rejectFn: func(ctx *gin.Context, requestID, callerID int64) error {
		return services.ErrJoinRequestNotFound
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/join-requests/1/reject", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	ctrl.Reject(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestJoinRequestController_Reject_Forbidden(t *testing.T) {
	svc := &mockJoinRequestService{rejectFn: func(ctx *gin.Context, requestID, callerID int64) error {
		return services.ErrJoinRequestForbidden
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/join-requests/1/reject", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	ctrl.Reject(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestJoinRequestController_ListMine_Success(t *testing.T) {
	svc := &mockJoinRequestService{listMineFn: func(ctx *gin.Context, runnerID int64) ([]joinrequest.JoinRequestResponse, error) {
		return []joinrequest.JoinRequestResponse{
			{ID: 1, TeamID: 5, RunnerID: runnerID, Status: "pending"},
		}, nil
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/join-requests/mine", nil)
	setAuthUserID(c, 7)

	ctrl.ListMine(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"team_id":5`)
}

func TestJoinRequestController_ListByTeam_Success(t *testing.T) {
	svc := &mockJoinRequestService{listByTeamFn: func(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
		return []joinrequest.JoinRequestResponse{
			{ID: 2, TeamID: teamID, RunnerID: 9, Status: "pending"},
		}, nil
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/teams/5/join-requests", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setAuthUserID(c, 1)

	ctrl.ListByTeam(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"runner_id":9`)
}

func TestJoinRequestController_ListByTeam_Forbidden(t *testing.T) {
	svc := &mockJoinRequestService{listByTeamFn: func(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
		return nil, services.ErrJoinRequestForbidden
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/teams/5/join-requests", nil)
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	setAuthUserID(c, 2)

	ctrl.ListByTeam(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestJoinRequestController_ListByTeam_NotFound(t *testing.T) {
	svc := &mockJoinRequestService{listByTeamFn: func(ctx *gin.Context, teamID, callerID int64) ([]joinrequest.JoinRequestResponse, error) {
		return nil, services.ErrTeamNotFound
	}}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/teams/999/join-requests", nil)
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	setAuthUserID(c, 1)

	ctrl.ListByTeam(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestJoinRequestController_PendingCount(t *testing.T) {
	svc := &mockJoinRequestService{pendingCountFn: func(ctx *gin.Context, ownerID int64) (int64, error) { return 4, nil }}
	ctrl := NewJoinRequestController(svc)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/join-requests/pending-count", nil)
	setAuthUserID(c, 1)

	ctrl.PendingCount(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"count":4`)
}
