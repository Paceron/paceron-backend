package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
)

type mockRunnerSessionControllerService struct {
	createFn func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error)
	finishFn func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error)
	getFn    func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error)
}

func (m *mockRunnerSessionControllerService) Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error) {
	if m.createFn != nil {
		return m.createFn(ctx, authUserID, sessionInstanceID, req)
	}
	return nil, false, nil
}

func (m *mockRunnerSessionControllerService) Finish(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
	if m.finishFn != nil {
		return m.finishFn(ctx, authUserID, sessionInstanceID, req)
	}
	return nil, nil
}

func (m *mockRunnerSessionControllerService) Get(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error) {
	if m.getFn != nil {
		return m.getFn(ctx, authUserID, sessionInstanceID, athleteUserID)
	}
	return nil, nil
}

func fixtureRunnerSessionResponse() *dbs.RunnerSession {
	start := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
	return &dbs.RunnerSession{
		ID:                1,
		SessionInstanceID: 10,
		AthleteUserID:     7,
		Status:            "wip",
		StartDate:         start,
	}
}

func TestRunnerSessionController_Unauthorized(t *testing.T) {
	controller := NewRunnerSessionController(&mockRunnerSessionControllerService{})

	for _, tc := range []struct {
		name  string
		call  func(c *gin.Context)
	}{
		{"create", func(c *gin.Context) { controller.Create(c) }},
		{"finish", func(c *gin.Context) { controller.Finish(c) }},
		{"get", func(c *gin.Context) { controller.Get(c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", nil)
			tc.call(c)
			assert.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}

func TestRunnerSessionController_Create_201(t *testing.T) {
	var gotAuth, gotSession int64
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error) {
			gotAuth = authUserID
			gotSession = sessionInstanceID
			return fixtureRunnerSessionResponse(), true, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc)

	response := httptest.NewRecorder()
	body := `{"start_date":"2026-09-24T09:00:00Z"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Create(c)

	require.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, int64(7), gotAuth)
	assert.Equal(t, int64(10), gotSession)

	var resp runnersession.MutationResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, runnersession.MsgRunnerSessionCreated, resp.Message)
	require.NotNil(t, resp.Data)
	assert.Equal(t, int64(1), resp.Data.ID)
	assert.Equal(t, "wip", resp.Data.Status)
}

func TestRunnerSessionController_Create_200_Existing(t *testing.T) {
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error) {
			return fixtureRunnerSessionResponse(), false, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc)

	response := httptest.NewRecorder()
	body := `{"start_date":"2026-09-24T09:00:00Z"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Create(c)

	require.Equal(t, http.StatusOK, response.Code)

	var resp runnersession.MutationResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, runnersession.MsgRunnerSessionExisted, resp.Message)
	assert.Equal(t, int64(1), resp.Data.ID)
}

func TestRunnerSessionController_Create_BadPathParam(t *testing.T) {
	controller := NewRunnerSessionController(&mockRunnerSessionControllerService{})

	response := httptest.NewRecorder()
	body := `{"start_date":"2026-09-24T09:00:00Z"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/abc/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}
	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRunnerSessionController_Create_BadJSON(t *testing.T) {
	controller := NewRunnerSessionController(&mockRunnerSessionControllerService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader("{not json"))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRunnerSessionController_Create_MissingStartDate(t *testing.T) {
	controller := NewRunnerSessionController(&mockRunnerSessionControllerService{})

	response := httptest.NewRecorder()
	body := `{}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRunnerSessionController_ErrorMapping(t *testing.T) {
	cases := []struct {
		name         string
		serviceErr   error
		expectedCode int
	}{
		{"invalid -> 400", services.ErrRunnerSessionInvalid, http.StatusBadRequest},
		{"forbidden -> 403", services.ErrRunnerSessionForbidden, http.StatusForbidden},
		{"session not found -> 404", services.ErrSessionInstanceNotFound, http.StatusNotFound},
		{"runner not found -> 404", daos.ErrRunnerSessionNotFound, http.StatusNotFound},
		{"unknown -> 500", assert.AnError, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := &mockRunnerSessionControllerService{
				createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error) {
					return nil, false, tc.serviceErr
				},
			}
			controller := NewRunnerSessionController(mockSvc)

			response := httptest.NewRecorder()
			body := `{"start_date":"2026-09-24T09:00:00Z"}`
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			setAuthUserID(c, 7)
			c.Params = []gin.Param{{Key: "id", Value: "10"}}
			controller.Create(c)

			assert.Equal(t, tc.expectedCode, response.Code)
		})
	}
}

func TestRunnerSessionController_Finish_200(t *testing.T) {
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = "finished"
			now := time.Now()
			rs.EndDate = &now
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc)

	response := httptest.NewRecorder()
	body := `{"status":"finished"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)

	var resp runnersession.MutationResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, runnersession.MsgRunnerSessionFinished, resp.Message)
	require.NotNil(t, resp.Data)
	assert.Equal(t, "finished", resp.Data.Status)
	require.NotNil(t, resp.Data.EndDate)
}

func TestRunnerSessionController_Finish_BadStatusBody(t *testing.T) {
	controller := NewRunnerSessionController(&mockRunnerSessionControllerService{})

	response := httptest.NewRecorder()
	body := `{"status":""}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Finish(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRunnerSessionController_Get_200(t *testing.T) {
	var gotAuth, gotSession int64
	mockSvc := &mockRunnerSessionControllerService{
		getFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error) {
			gotAuth = authUserID
			gotSession = sessionInstanceID
			require.Nil(t, athleteUserID)
			return fixtureRunnerSessionResponse(), nil
		},
	}
	controller := NewRunnerSessionController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/runner?a=b", nil)
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Get(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(7), gotAuth)
	assert.Equal(t, int64(10), gotSession)

	var resp runnersession.RunnerSessionListResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, int64(1), resp.Data.ID)
	assert.Equal(t, "wip", resp.Data.Status)
}

func TestRunnerSessionController_Get_WithAthleteQuery(t *testing.T) {
	athlete := int64(30)
	var gotAthlete *int64
	mockSvc := &mockRunnerSessionControllerService{
		getFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error) {
			gotAthlete = athleteUserID
			return fixtureRunnerSessionResponse(), nil
		},
	}
	controller := NewRunnerSessionController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/runner?athlete_user_id=30", nil)
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Get(c)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, gotAthlete)
	assert.Equal(t, athlete, *gotAthlete)
}

func TestRunnerSessionController_Get_InvalidQuery(t *testing.T) {
	controller := NewRunnerSessionController(&mockRunnerSessionControllerService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/runner?athlete_user_id=abc", nil)
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Get(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRunnerSessionController_Get_NotFound(t *testing.T) {
	mockSvc := &mockRunnerSessionControllerService{
		getFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error) {
			return nil, daos.ErrRunnerSessionNotFound
		},
	}
	controller := NewRunnerSessionController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/runner", nil)
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Get(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}