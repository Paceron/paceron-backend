package controllers

// --- Hooks de logros (gamificacion-core D5, task 3.5): negación/positivos ---
// PATCH de runner session hacia finished/interrupted; Create/Update de
// feedback (distance) y subida de points (gps); altas de asistencia (team).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/services"
)

type achEvent struct {
	userID  int64
	trigger services.AchievementTrigger
}

// stubAchievementsTrigger graba las invocaciones del hook de logros.
type stubAchievementsTrigger struct {
	events []achEvent
}

func (s *stubAchievementsTrigger) OnEvent(ctx *gin.Context, userID int64, trigger services.AchievementTrigger) {
	s.events = append(s.events, achEvent{userID: userID, trigger: trigger})
}

func achievementEventsOf(s *stubAchievementsTrigger) []achEvent {
	return s.events
}

// Bodies válidos de bind: el controller hace ShouldBindJSON antes del service,
// así que el hook se ejercita con payloads que pasan el bind del DTO.
const achievementFeedbackBody = `{"assigned_session_id":1,"assigned_exercise_id":2,"report_source":"corredor","session_date":"2026-01-15","set_number":1}`
const achievementPointsBody = `{"points":[{"order":0,"session_instance_id":1,"exercise_instance_id":2,"latitude":-31.4,"longitude":-64.2,"recorded_at":"2026-01-15T10:00:00Z"}]}`

// --- Runner session: mismo disparador post-write del PATCH de racha ---

func TestRunnerSessionController_Finish_DisparaLogros(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = runnersession.RunnerSessionStatusFinished
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)
	events := achievementEventsOf(&trigger)
	require.Len(t, events, 1)
	assert.Equal(t, int64(7), events[0].userID)
	assert.Equal(t, services.AchievementTriggerRunnerSession, events[0].trigger)
}

func TestRunnerSessionController_Create_WipNoDisparaLogros(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockRunnerSessionControllerService{
		createFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
			return fixtureRunnerSessionResponse(), nil, true, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	body := `{"start_date":"2026-09-24T09:00:00Z"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/session-instances/10/runner", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	controller.Create(c)

	require.Equal(t, http.StatusCreated, response.Code)
	assert.Empty(t, achievementEventsOf(&trigger), "el Create (wip) jamás dispara el hook de logros")
}

func TestRunnerSessionController_Finish_ErrorNoDisparaLogros(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			return nil, services.ErrRunnerSessionInvalid
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Empty(t, achievementEventsOf(&trigger))
}

func TestRunnerSessionController_Finish_SinHookNilSafe(t *testing.T) {
	mockSvc := &mockRunnerSessionControllerService{
		finishFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSessionResponse()
			rs.Status = runnersession.RunnerSessionStatusFinished
			return rs, nil
		},
	}
	controller := NewRunnerSessionController(mockSvc, nil, nil, nil, nil)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	runnerSessionPatchFinish(c, `{"status":"finished"}`)
	controller.Finish(c)

	require.Equal(t, http.StatusOK, response.Code)
}

// --- Feedback: distance (Create/Update) y gps (CreatePoints) ---

func achievementFeedbackContext(t *testing.T, verb, target, body string) *gin.Context {
	t.Helper()
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(verb, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	if target != "/api/v1/workout-feedback" {
		c.Params = []gin.Param{{Key: "id", Value: "5"}}
	}
	return c
}

func TestWorkoutFeedbackController_Create_DisparaLogrosDistance(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockWorkoutFeedbackService{
		createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedback(), nil
		},
	}
	controller := NewWorkoutFeedbackController(mockSvc, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", strings.NewReader(achievementFeedbackBody))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Create(c)

	require.Equal(t, http.StatusCreated, response.Code)
	events := achievementEventsOf(&trigger)
	require.Len(t, events, 1)
	assert.Equal(t, int64(7), events[0].userID, "a favor del atleta del feedback")
	assert.Equal(t, services.AchievementTriggerFeedback, events[0].trigger)
}

func TestWorkoutFeedbackController_Update_DisparaLogrosDistance(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockWorkoutFeedbackService{
		updateFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedback(), nil
		},
	}
	controller := NewWorkoutFeedbackController(mockSvc, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/workout-feedback/5", strings.NewReader(achievementFeedbackBody))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "5"}}
	controller.Update(c)

	require.Equal(t, http.StatusOK, response.Code)
	events := achievementEventsOf(&trigger)
	require.Len(t, events, 1)
	assert.Equal(t, services.AchievementTriggerFeedback, events[0].trigger)
}

func TestWorkoutFeedbackController_CreatePoints_DisparaLogrosGPS(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockWorkoutFeedbackService{
		createPointsFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*services.PointsResult, error) {
			// El atleta del feedback puede ser distinto del reportante.
			return &services.PointsResult{Created: 2, Skipped: 1, AthleteUserID: 42}, nil
		},
	}
	controller := NewWorkoutFeedbackController(mockSvc, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback/5/points", strings.NewReader(achievementPointsBody))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	c.Params = []gin.Param{{Key: "id", Value: "5"}}
	controller.CreatePoints(c)

	require.Equal(t, http.StatusCreated, response.Code)
	events := achievementEventsOf(&trigger)
	require.Len(t, events, 1)
	assert.Equal(t, int64(42), events[0].userID, "gps_N a favor del atleta, no del reportante")
	assert.Equal(t, services.AchievementTriggerGPSPoints, events[0].trigger)
}

func TestWorkoutFeedbackController_ErrorNoDisparaLogros(t *testing.T) {
	var trigger stubAchievementsTrigger
	mockSvc := &mockWorkoutFeedbackService{
		createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return nil, services.ErrWorkoutFeedbackInvalid
		},
	}
	controller := NewWorkoutFeedbackController(mockSvc, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", strings.NewReader(achievementFeedbackBody))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Create(c)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Empty(t, achievementEventsOf(&trigger))
}

// --- Asistencia: team_N en altas reales (created y bulk) ---

func attendanceRegisterWithContext(t *testing.T, ac AttendanceController, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/team/5/session/9", nil)
	setAuthUserID(c, userID)
	c.Params = []gin.Param{
		{Key: "team_id", Value: "5"},
		{Key: "training_session_id", Value: "9"},
	}
	ac.RegisterAttendance(c)
	return response
}

func TestAttendanceController_Register_CreadaDisparaLogros(t *testing.T) {
	var trigger stubAchievementsTrigger
	mock := &mockAttendanceService{
		registerFn: func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, *services.AttendanceEventRow, *daos.AttendanceSessionContext, error) {
			return true, &services.AttendanceEventRow{UserID: userID, TrainingSessionID: sessionID}, sessionCtxFor(teamID, 5, time.Now()), nil
		},
	}
	controller := NewAttendanceController(mock, nil, &trigger)

	response := attendanceRegisterWithContext(t, controller, 7)

	require.Equal(t, http.StatusCreated, response.Code)
	events := achievementEventsOf(&trigger)
	require.Len(t, events, 1)
	assert.Equal(t, int64(7), events[0].userID)
	assert.Equal(t, services.AchievementTriggerAttendance, events[0].trigger)
}

func TestAttendanceController_Register_IdempotenteNoDispara(t *testing.T) {
	var trigger stubAchievementsTrigger
	mock := &mockAttendanceService{
		registerFn: func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, *services.AttendanceEventRow, *daos.AttendanceSessionContext, error) {
			return false, nil, sessionCtxFor(teamID, 5, time.Now()), nil
		},
	}
	controller := NewAttendanceController(mock, nil, &trigger)

	response := attendanceRegisterWithContext(t, controller, 7)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, achievementEventsOf(&trigger), "la idempotencia no registra un hecho nuevo")
}

func TestAttendanceController_BulkSave_DisparaLogrosPorAsistente(t *testing.T) {
	var trigger stubAchievementsTrigger
	mock := &mockAttendanceService{
		bulkSaveFn: func(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, []services.AttendanceEventRow, error) {
			rows := []services.AttendanceEventRow{
				{UserID: 11, TrainingSessionID: sessionInstanceID},
				{UserID: 22, TrainingSessionID: sessionInstanceID},
			}
			return &attendance.BulkSaveResult{Created: 2}, rows, nil
		},
	}
	controller := NewAttendanceController(mock, nil, &trigger)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	body := `{"team_id":5,"training_session_id":9,"entries":[{"user_id":11},{"user_id":22}]}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/bulk", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 1)
	controller.BulkSaveAttendance(c)

	require.Equal(t, http.StatusOK, response.Code)
	events := achievementEventsOf(&trigger)
	require.Len(t, events, 2)
	assert.Equal(t, int64(11), events[0].userID)
	assert.Equal(t, int64(22), events[1].userID)
	for _, event := range events {
		assert.Equal(t, services.AchievementTriggerAttendance, event.trigger)
	}
}
