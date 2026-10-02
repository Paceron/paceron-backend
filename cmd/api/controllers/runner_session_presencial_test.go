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
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
)

// mockPresencialGateway implementa services.PresencialSessionServiceInterface
// con fns opcionales (nil → no-op) para probar hooks y gate desde el
// controller sin DB.
type mockPresencialGateway struct {
	checkEntryFn        func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error)
	onCreatedFn         func(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error)
	onFinishedFn        func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error)
	createdInvocations  int
	finishedInvocations int
}

func (m *mockPresencialGateway) CheckAthleteEntry(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error) {
	if m.checkEntryFn != nil {
		return m.checkEntryFn(ctx, sessionInstanceID, authUserID)
	}
	return nil, nil
}

func (m *mockPresencialGateway) OnRunnerCreated(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error) {
	m.createdInvocations++
	if m.onCreatedFn != nil {
		return m.onCreatedFn(ctx, sessionInstanceID, authUserID, gateDay)
	}
	return nil, false, nil
}

func (m *mockPresencialGateway) OnRunnerFinished(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
	m.finishedInvocations++
	if m.onFinishedFn != nil {
		return m.onFinishedFn(ctx, sessionInstanceID, authUserID)
	}
	return nil, false, nil
}

func newRunnerCtrl(mockSvc *mockRunnerSessionControllerService, presencial services.PresencialSessionServiceInterface, notifier realtime.Notifier) RunnerSessionController {
	return NewRunnerSessionController(mockSvc, presencial, notifier)
}

func runnerSessionPOST(c *gin.Context, body string) {
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
}

func runnerSessionPATCH(c *gin.Context, body string) {
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
}

// El gate corre DENTRO del service (orden 404→403→409): desde el controller
// el error llega envuelto, así que el mock del SERVICE lo devuelve y acá se
// prueba solo el mapping a 409 + Code slug.
func TestRunnerSessionController_Create_GateClosed_409WithSlug(t *testing.T) {
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return nil, nil, false, services.ErrRunnerSessionClosed
		},
	}
	controller := newRunnerCtrl(mockSvc, &mockPresencialGateway{}, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPOST(c, `{"start_date":"2026-09-24T09:00:00Z"}`)
	controller.Create(c)

	require.Equal(t, http.StatusConflict, response.Code)
	var resp apierror.APIError
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, "session_closed", resp.Code)
}

func TestRunnerSessionController_Create_GateNotOpen_409WithSlug(t *testing.T) {
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return nil, nil, false, services.ErrRunnerSessionNotOpen
		},
	}
	controller := newRunnerCtrl(mockSvc, &mockPresencialGateway{}, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPOST(c, `{"start_date":"2026-09-24T09:00:00Z"}`)
	controller.Create(c)

	require.Equal(t, http.StatusConflict, response.Code)
	var resp apierror.APIError
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, "session_not_opened", resp.Code)
}

func TestRunnerSessionController_Create_HookInvokedAfterSuccess(t *testing.T) {
	gateDay := &dbs.GroupCalendarDay{ID: 9}
	var gotDay *dbs.GroupCalendarDay
	called := false
	presencial := &mockPresencialGateway{onCreatedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error) {
		called = true
		gotDay = gateDay
		require.Equal(t, int64(10), sessionInstanceID)
		require.Equal(t, int64(7), authUserID)
		return nil, false, nil
	}}
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return fixtureRunnerSessionResponse(), gateDay, true, nil
		},
	}
	controller := newRunnerCtrl(mockSvc, presencial, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPOST(c, `{"start_date":"2026-09-24T09:00:00Z"}`)
	controller.Create(c)

	require.Equal(t, http.StatusCreated, response.Code)
	assert.True(t, called)
	assert.Same(t, gateDay, gotDay, "el día cargado por el gate viaja al hook sin re-consultar")
}

func TestRunnerSessionController_Create_HookNotInvokedOnServiceError(t *testing.T) {
	presencial := &mockPresencialGateway{}
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return nil, nil, false, daos.ErrRunnerSessionNotFound
		},
	}
	controller := newRunnerCtrl(mockSvc, presencial, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPOST(c, `{"start_date":"2026-09-24T09:00:00Z"}`)
	controller.Create(c)

	require.Equal(t, http.StatusNotFound, response.Code)
	assert.Zero(t, presencial.createdInvocations)
}

func TestRunnerSessionController_Create_HookErrorSurfaces(t *testing.T) {
	boom := assert.AnError
	presencial := &mockPresencialGateway{onCreatedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error) {
		return nil, false, boom
	}}
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return fixtureRunnerSessionResponse(), nil, true, nil
		},
	}
	controller := newRunnerCtrl(mockSvc, presencial, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPOST(c, `{"start_date":"2026-09-24T09:00:00Z"}`)
	controller.Create(c)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestRunnerSessionController_Finish_HookOnlyOnFinished(t *testing.T) {
	invokeAssert := func(status string, expectHook bool) *httptest.ResponseRecorder {
		presencial := &mockPresencialGateway{}
		mockSvc := &mockRunnerSessionControllerService{
			finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
				rs := fixtureRunnerSessionResponse()
				rs.Status = status
				now := time.Now()
				rs.EndDate = &now
				return rs, nil
			},
		}
		controller := newRunnerCtrl(mockSvc, presencial, nil)
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		runnerSessionPATCH(c, `{"status":"`+status+`"}`)
		controller.Finish(c)
		require.Equal(t, http.StatusOK, response.Code)
		expected := 0
		if expectHook {
			expected = 1
		}
		assert.Equal(t, expected, presencial.finishedInvocations)
		return response
	}
	invokeAssert("finished", true)
	invokeAssert("interrupted", false)
}

func TestRunnerSessionController_Finish_HookErrorSurfaces(t *testing.T) {
	boom := assert.AnError
	presencial := &mockPresencialGateway{onFinishedFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
		return nil, false, boom
	}}
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = "finished"
			return rs, nil
		},
	}
	controller := newRunnerCtrl(mockSvc, presencial, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPATCH(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}
