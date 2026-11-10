package controllers

// --- Task 7 / review IMPORTANT: el finish calcula el snapshot de rachas una
// sola vez y entrega el MISMO puntero al trigger y a los logros.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

func TestRunnerSessionController_Finish_ComputaSnapshotUnaVezYLoComparte(t *testing.T) {
	var streakTrigger mockStreakTrigger
	var achievementsTrigger stubAchievementsTrigger
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = runnersession.RunnerSessionStatusFinished
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, &streakTrigger, &achievementsTrigger)
	controller.(*runnerSessionController).asyncHook = func(fn func()) { fn() }

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 1, streakTrigger.computeCalls, "el snapshot se calcula exactamente una vez")
	require.NotNil(t, streakTrigger.computedSnapshot)
	require.NotNil(t, streakTrigger.receivedSnapshot, "el trigger de rachas recibe snapshot no-nil")
	require.NotNil(t, achievementsTrigger.receivedSnapshot, "logros recibe snapshot no-nil")
	assert.Same(t, streakTrigger.computedSnapshot, streakTrigger.receivedSnapshot,
		"el trigger de rachas recibe el mismo snapshot")
	assert.Same(t, streakTrigger.computedSnapshot, achievementsTrigger.receivedSnapshot,
		"logros recibe el mismo snapshot que el trigger")
	assert.Equal(t, int64(7), streakTrigger.receivedUserID)
	assert.Equal(t, int64(7), achievementsTrigger.receivedUserID)
}
