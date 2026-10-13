package controllers

// Competencias (E4.1): guards/400/404 + no-leak del controller.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
)

type mockCompetitionService struct {
	createFn  func(callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error)
	approveFn func(callerID, competitionID int64) (*competition.CompetitionResponse, error)
	rejectFn  func(callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error)
	cancelFn  func(callerID, competitionID int64) (*competition.CompetitionResponse, error)
	listFn    func(callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error)
	getFn     func(callerID, competitionID int64) (*competition.CompetitionDetailResponse, error)
	closeFn   func(callerID, competitionID int64) (*competition.CompetitionResponse, error)

	createCalls, approveCalls, rejectCalls, cancelCalls, listCalls, getCalls, closeCalls int
	lastReason                                                                           *string
	lastStatus                                                                           string
	lastGroupID                                                                          *int64
}

func (m *mockCompetitionService) Create(ctx *gin.Context, callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error) {
	m.createCalls++
	if m.createFn != nil {
		return m.createFn(callerID, teamID, req)
	}
	return &competition.CompetitionResponse{ID: 5, Status: competition.StatusActive}, nil
}

func (m *mockCompetitionService) Approve(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	m.approveCalls++
	if m.approveFn != nil {
		return m.approveFn(callerID, competitionID)
	}
	return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusActive}, nil
}

func (m *mockCompetitionService) Reject(ctx *gin.Context, callerID, competitionID int64, reason *string) (*competition.CompetitionResponse, error) {
	m.rejectCalls++
	m.lastReason = reason
	if m.rejectFn != nil {
		return m.rejectFn(callerID, competitionID, reason)
	}
	return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusRejected}, nil
}

func (m *mockCompetitionService) Cancel(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	m.cancelCalls++
	if m.cancelFn != nil {
		return m.cancelFn(callerID, competitionID)
	}
	return nil, nil
}

func (m *mockCompetitionService) ListByTeam(ctx *gin.Context, callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error) {
	m.listCalls++
	m.lastStatus = status
	m.lastGroupID = groupID
	if m.listFn != nil {
		return m.listFn(callerID, teamID, status, groupID)
	}
	return &competition.CompetitionsListResponse{Data: []competition.CompetitionResponse{}}, nil
}

func (m *mockCompetitionService) Get(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionDetailResponse, error) {
	m.getCalls++
	if m.getFn != nil {
		return m.getFn(callerID, competitionID)
	}
	return &competition.CompetitionDetailResponse{Data: competition.CompetitionResponse{ID: competitionID}}, nil
}

func (m *mockCompetitionService) Close(ctx *gin.Context, callerID, competitionID int64) (*competition.CompetitionResponse, error) {
	m.closeCalls++
	if m.closeFn != nil {
		return m.closeFn(callerID, competitionID)
	}
	return &competition.CompetitionResponse{ID: competitionID, Status: competition.StatusFinished}, nil
}

func setupCompetitionRouterS(t *testing.T, svc services.CompetitionServiceInterface, authUserID int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if authUserID > 0 {
			c.Set(utils.AuthUserIDKey, authUserID)
		}
		c.Next()
	})
	cc := NewCompetitionController(svc)
	router.POST("/api/v1/teams/:id/competitions", cc.Create)
	router.GET("/api/v1/teams/:id/competitions", cc.ListByTeam)
	router.PATCH("/api/v1/competitions/:id/approve", cc.Approve)
	router.PATCH("/api/v1/competitions/:id/reject", cc.Reject)
	router.PATCH("/api/v1/competitions/:id/close", cc.Close)
	router.DELETE("/api/v1/competitions/:id/cancel", cc.Cancel)
	router.GET("/api/v1/competitions/:id", cc.GetDetail)
	return router
}

func doCompetitionRequest(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func competitionErrorMessage(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	msg, ok := out["message"].(string)
	require.True(t, ok)
	return msg
}

func TestCompetitionController_Unauthorized(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 0)
	assert.Equal(t, http.StatusUnauthorized, doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions", "").Code)
	assert.Equal(t, http.StatusUnauthorized, doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/close", "").Code)
}

func TestCompetitionController_PathInvalido_DAONoTocado(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	require.Equal(t, http.StatusBadRequest, doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/abc/competitions", "").Code)
	require.Equal(t, http.StatusBadRequest, doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/-1/approve", "").Code)
	assert.Zero(t, svc.approveCalls)
	assert.Zero(t, svc.listCalls)
}

func TestCompetitionController_Create_BodyInvalido(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/teams/2/competitions", `{"type":...`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Zero(t, svc.createCalls)
}

func TestCompetitionService_Create_Success_MapeaDatos(t *testing.T) {
	svc := &mockCompetitionService{createFn: func(callerID, teamID int64, req competition.CreateCompetitionRequest) (*competition.CompetitionResponse, error) {
		metric := "distance"
		return &competition.CompetitionResponse{ID: 9, TeamID: teamID, Type: competition.TypeMetric, Metric: &metric,
			Status: competition.StatusActive, Participants: []int64{3, 4}}, nil
	}}
	router := setupCompetitionRouterS(t, svc, 1)
	resp := doCompetitionRequest(t, router, http.MethodPost, "/api/v1/teams/2/competitions",
		`{"type":"metric","metric":"distance","start_at":"2026-10-01T00:00:00Z","end_at":"2026-10-10T00:00:00Z","participants":[3,4]}`)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, 1, svc.createCalls)
	var out competition.CompetitionDetailResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	assert.Equal(t, int64(9), out.Data.ID)
	assert.Equal(t, "metric", out.Data.Type)
	require.Len(t, out.Data.Participants, 2)
}

func TestCompetitionController_ErroresMapeados(t *testing.T) {
	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{"body invalido", services.ErrCompetitionInvalid, http.StatusBadRequest, "Bad request"},
		{"transicion invalida", services.ErrCompetitionConflict, http.StatusBadRequest, "Bad request"},
		{"ajeno", services.ErrCompetitionForbidden, http.StatusForbidden, "Forbidden"},
		{"no existe", daos.ErrCompetitionNotFound, http.StatusNotFound, "Not Found"},
	}
	for _, tc := range cases {
		svc := &mockCompetitionService{closeFn: func(callerID, competitionID int64) (*competition.CompetitionResponse, error) {
			return nil, tc.serviceErr
		}}
		router := setupCompetitionRouterS(t, svc, 7)
		resp := doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/close", "")
		require.Equal(t, tc.wantStatus, resp.Code, tc.name)
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
		assert.Equal(t, tc.wantCode, out["code"], tc.name)
	}

	// Error crudo del DAO → 500 genérico sin leak.
	svc := &mockCompetitionService{listFn: func(callerID, teamID int64, status string, groupID *int64) (*competition.CompetitionsListResponse, error) {
		return nil, errors.New("SQLSTATE 23505 detalle interno")
	}}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions", "")
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "error interno", competitionErrorMessage(t, resp))
	assert.NotContains(t, resp.Body.String(), "SQLSTATE")
}

func TestCompetitionController_ListQueryParams(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions?status=active&group_id=9", "")
	require.Equal(t, 1, svc.listCalls)
	assert.Equal(t, "active", svc.lastStatus)
	require.NotNil(t, svc.lastGroupID)
	assert.Equal(t, int64(9), *svc.lastGroupID)
}

func TestCompetitionController_RejectBodyOpcional(t *testing.T) {
	// Con reason.
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/reject", `{"reason":"demasiado difícil"}`)
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, svc.lastReason)
	assert.Equal(t, "demasiado difícil", *svc.lastReason)

	// Sin body: igual rechaza con reason nil.
	svc = &mockCompetitionService{}
	router = setupCompetitionRouterS(t, svc, 7)
	resp = doCompetitionRequest(t, router, http.MethodPatch, "/api/v1/competitions/2/reject", "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Nil(t, svc.lastReason)
}

func TestCompetitionController_Cancel_Mensaje(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodDelete, "/api/v1/competitions/2/cancel", "")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, competition.MsgCompetitionCancelled, competitionErrorMessage(t, resp))
}

// FailingDB no-leak pin (D12): el DAO de guard falla con Postgres real y el
// endpoint de listado responde 500 genérico sin filtrar el error inyectado.
func TestCompetitionController_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	realSvc := services.NewCompetitionService(daos.NewCompetitionDao(failing), nil, nil)
	router := setupCompetitionRouterS(t, realSvc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/teams/2/competitions", "")
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "error interno", competitionErrorMessage(t, resp))
	assert.NotContains(t, resp.Body.String(), "inyectado", "sin leak del error de DB de test")
	assert.NotContains(t, resp.Body.String(), "FailingDB")
}

func TestCompetitionController_GetDetail_Success(t *testing.T) {
	svc := &mockCompetitionService{}
	router := setupCompetitionRouterS(t, svc, 7)
	resp := doCompetitionRequest(t, router, http.MethodGet, "/api/v1/competitions/2", "")
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, 1, svc.getCalls)
	var out competition.CompetitionDetailResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	assert.Equal(t, int64(2), out.Data.ID)
}
