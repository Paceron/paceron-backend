package controllers

// --- Hook de gamificación (gamificacion-core D3, tasks 2.2/2.4) ---
// Negación: el Create (wip) NO dispara el trigger de racha; el PATCH hacia
// finished/interrupted sí (best-effort async, error no frena la respuesta).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
)

// mockStreakTrigger graba las invocaciones del hook de racha.
type mockStreakTrigger struct {
	completions []int64
}

func (m *mockStreakTrigger) OnRunnerSessionCompletion(ctx *gin.Context, userID int64) {
	m.completions = append(m.completions, userID)
}

func (m *mockStreakTrigger) EvaluateRunnerCompletion(ctx *gin.Context, userID int64) error {
	return nil
}

func runnerSessionPatchFinish(c *gin.Context, body string) {
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
}

func TestRunnerSessionController_Create_WipNoDisparaStreakTrigger(t *testing.T) {
	var trigger mockStreakTrigger
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return fixtureRunnerSessionResponse(), nil, true, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	body := `{"start_date":"2026-09-24T09:00:00Z"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Create(c)

	require.Equal(t, http.StatusCreated, response.Code)
	assert.Empty(t, trigger.completions, "el Create (wip) jamás dispara el trigger de racha")
}

func TestRunnerSessionController_Finish_DisparaStreakTrigger(t *testing.T) {
	var trigger mockStreakTrigger
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = "finished"
			end := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
			rs.EndDate = &end
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, []int64{7}, trigger.completions, "dispara una vez con el atleta resuelto del service")
}

func TestRunnerSessionController_Finish_Interrupted_DisparaStreakTrigger(t *testing.T) {
	var trigger mockStreakTrigger
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = "interrupted"
			end := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
			rs.EndDate = &end
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"interrupted"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, []int64{7}, trigger.completions)
}

func TestRunnerSessionController_Finish_SinTriggerNilSafe(t *testing.T) {
	// Sin trigger (nil): el PATCH responde igual (camino de producción sin el
	// módulo gamificado abajo).
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = "finished"
			end := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
			rs.EndDate = &end
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)
}

// El fallo del service NO debe dejar el hook en el aire: el error sube antes
// (404 acá) y el trigger queda sin invocar.
func TestRunnerSessionController_Finish_ErrorNoDispara(t *testing.T) {
	var trigger mockStreakTrigger
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			return nil, services.ErrRunnerSessionInvalid
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Empty(t, trigger.completions)
}
