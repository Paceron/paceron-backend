package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockWorkoutFeedbackService struct {
	createFn              func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error)
	getByIDFn             func(ctx *gin.Context, authUserID, feedbackID int64) (*dbs.WorkoutFeedback, error)
	searchFn              func(ctx *gin.Context, authUserID int64, filters workoutfeedback.SearchFilters) ([]dbs.WorkoutFeedback, error)
	updateFn              func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error)
	softDeleteFn          func(ctx *gin.Context, authUserID, feedbackID int64) error
	createPointsFn        func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*services.PointsResult, error)
	getPointsFn           func(ctx *gin.Context, authUserID, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error)
	getSessionFeedbackFn  func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error)
	athleteHistoryFn      func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error)
	administeredHistoryFn func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error)
}

func (m *mockWorkoutFeedbackService) Create(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
	if m.createFn != nil {
		return m.createFn(ctx, authUserID, req)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) GetByID(ctx *gin.Context, authUserID, feedbackID int64) (*dbs.WorkoutFeedback, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, authUserID, feedbackID)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) Search(ctx *gin.Context, authUserID int64, filters workoutfeedback.SearchFilters) ([]dbs.WorkoutFeedback, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, authUserID, filters)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) Update(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, authUserID, feedbackID, req)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) SoftDelete(ctx *gin.Context, authUserID, feedbackID int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, authUserID, feedbackID)
	}
	return nil
}

func (m *mockWorkoutFeedbackService) CreatePoints(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*services.PointsResult, error) {
	if m.createPointsFn != nil {
		return m.createPointsFn(ctx, authUserID, feedbackID, req)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) GetPoints(ctx *gin.Context, authUserID, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
	if m.getPointsFn != nil {
		return m.getPointsFn(ctx, authUserID, feedbackID)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) GetSessionFeedback(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
	if m.getSessionFeedbackFn != nil {
		return m.getSessionFeedbackFn(ctx, authUserID, sessionInstanceID, athleteUserID)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) AthleteHistory(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
	if m.athleteHistoryFn != nil {
		return m.athleteHistoryFn(ctx, callerID, targetID, query)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackService) AdministeredHistory(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
	if m.administeredHistoryFn != nil {
		return m.administeredHistoryFn(ctx, callerID, targetID, query)
	}
	return nil, nil
}

// newTestWorkoutFeedbackController arma el controller SIN notifier: los tests
// HTTP no deben ver frames (nil = comportamiento anterior intacto).
func newTestWorkoutFeedbackController(svc services.WorkoutFeedbackServiceInterface) WorkoutFeedbackController {
	return NewWorkoutFeedbackController(svc, nil)
}

// stubFeedbackNotifier registra las llamadas Emit del hook de Create (D7).
type stubFeedbackNotifier struct {
	mu    sync.Mutex
	calls []stubNotifierCall
}

type stubNotifierCall struct {
	channel string
	payload []byte
}

func (s *stubFeedbackNotifier) Emit(channel string, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, stubNotifierCall{channel: channel, payload: payload})
}

// fixtureFeedbackForSession clona el feedback genérico con otra sesión/atleta
// para tests de broadcast al canal de una sesión puntual.
func fixtureFeedbackForSession(sessionID, athleteID int64) *dbs.WorkoutFeedback {
	fb := fixtureFeedback()
	fb.AssignedSessionID = sessionID
	fb.AthleteUserID = athleteID
	return fb
}

// fixtureFeedback arma un feedback persistido con media_urls lista en []string,
// para verificar el shape plano de la respuesta.
func fixtureFeedback() *dbs.WorkoutFeedback {
	var media pgtype.TextArray
	_ = media.Set([]string{"https://media.foo/a.jpg"})
	return &dbs.WorkoutFeedback{
		ID:                  1,
		AssignedSessionID:   1,
		AssignedExerciseID:  2,
		AthleteUserID:       7,
		FeedbackOwnerUserID: 7,
		ReportSource:        "corredor",
		SetNumber:           1,
		MediaURLs:           media,
	}
}

func TestWorkoutFeedbackController_Unauthorized(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", nil)
	controller.Create(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestWorkoutFeedbackController_Create_Success(t *testing.T) {
	var gotAuth int64
	mockSvc := &mockWorkoutFeedbackService{
		createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			gotAuth = authUserID
			return fixtureFeedback(), nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	body := `{"assigned_session_id":1,"assigned_exercise_id":2,"report_source":"corredor","session_date":"2026-01-15","set_number":1}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Create(c)

	require.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, int64(7), gotAuth)

	var resp workoutfeedback.MutationResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, workoutfeedback.MsgFeedbackCreated, resp.Message)
	require.NotNil(t, resp.Data)
	assert.Equal(t, int64(1), resp.Data.ID)
	assert.Equal(t, []string{"https://media.foo/a.jpg"}, resp.Data.MediaURLs)
}

// setEventFrame es el frame server→cliente de update:set_event con el wrapper
// {message, data} replicando el body HTTP (D4/D7).
type setEventFrame struct {
	Type string `json:"type"`
	Data struct {
		Message string                                  `json:"message"`
		Data    workoutfeedback.WorkoutFeedbackResponse `json:"data"`
	} `json:"data"`
}

// setupFeedbackBroadcastRouter arma las rutas del módulo con auth inyectada y
// el controller configurable (notifier opcional), para probar el hook de D7
// por la ruta real y status codes flusheados por ServeHTTP.
func setupFeedbackBroadcastRouter(svc services.WorkoutFeedbackServiceInterface, notifier realtime.Notifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, int64(7))
		c.Next()
	})
	ctrl := NewWorkoutFeedbackController(svc, notifier)
	r.POST("/workout-feedback", ctrl.Create)
	r.PUT("/workout-feedback/:id", ctrl.Update)
	r.DELETE("/workout-feedback/:id", ctrl.Delete)
	r.POST("/workout-feedback/:id/points", ctrl.CreatePoints)
	return r
}

func createFeedbackRequest(sessionID int64) *strings.Reader {
	return strings.NewReader(`{"assigned_session_id":` + strconv.FormatInt(sessionID, 10) + `,"assigned_exercise_id":2,"report_source":"corredor","session_date":"2026-01-15","set_number":1}`)
}

func TestWorkoutFeedbackController_Create_EmitsUpdateSetEventToSessionChannel(t *testing.T) {
	notifier := &stubFeedbackNotifier{}
	mockSvc := &mockWorkoutFeedbackService{
		createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedbackForSession(42, 7), nil
		},
	}
	router := setupFeedbackBroadcastRouter(mockSvc, notifier)

	req := httptest.NewRequest(http.MethodPost, "/workout-feedback", createFeedbackRequest(42))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	// Canal canónico exacto: session:{assigned_session_id} decimal sin padding.
	require.Len(t, notifier.calls, 1)
	assert.Equal(t, "session:42", notifier.calls[0].channel)

	var frame setEventFrame
	require.NoError(t, json.Unmarshal(notifier.calls[0].payload, &frame))
	assert.Equal(t, realtime.UpdateSetEventType, frame.Type)
	assert.Equal(t, workoutfeedback.MsgFeedbackCreated, frame.Data.Message)
	require.Equal(t, int64(42), frame.Data.Data.AssignedSessionID)
	assert.Equal(t, int64(7), frame.Data.Data.AthleteUserID)

	// El payload WS y el body HTTP transportan el mismo objeto de respuesta.
	var httpResp workoutfeedback.MutationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &httpResp))
	require.NotNil(t, httpResp.Data)
	assert.Equal(t, *httpResp.Data, frame.Data.Data)
}

func TestWorkoutFeedbackController_Create_NilNotifierKeepsHTTPBehavior(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedbackForSession(42, 7), nil
		},
	}
	router := setupFeedbackBroadcastRouter(mockSvc, nil)

	req := httptest.NewRequest(http.MethodPost, "/workout-feedback", createFeedbackRequest(42))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var resp workoutfeedback.MutationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, workoutfeedback.MsgFeedbackCreated, resp.Message)
	require.NotNil(t, resp.Data)
	assert.Equal(t, int64(7), resp.Data.AthleteUserID)
}

func TestWorkoutFeedbackController_MutationsOtherThanCreateDoNotEmit(t *testing.T) {
	notifier := &stubFeedbackNotifier{}
	mockSvc := &mockWorkoutFeedbackService{
		updateFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedbackForSession(42, 7), nil
		},
		softDeleteFn: func(ctx *gin.Context, authUserID, feedbackID int64) error {
			return nil
		},
		createPointsFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*services.PointsResult, error) {
			return &services.PointsResult{Created: 1, Skipped: 0}, nil
		},
	}
	router := setupFeedbackBroadcastRouter(mockSvc, notifier)

	req := httptest.NewRequest(http.MethodPut, "/workout-feedback/1", strings.NewReader(`{"rpe":8}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/workout-feedback/1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/workout-feedback/1/points", strings.NewReader(
		`{"points":[{"order":0,"session_instance_id":3,"exercise_instance_id":4,"latitude":-34.6,"longitude":-58.4,"recorded_at":"2026-09-24T14:00:00Z"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	assert.Empty(t, notifier.calls)
}

func TestWorkoutFeedbackController_Create_MissingRequired(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	body := `{"report_source":"corredor"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_Create_BadJSON(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", strings.NewReader("{not json"))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Create(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_ErrorMapping(t *testing.T) {
	cases := []struct {
		name         string
		serviceErr   error
		expectedCode int
	}{
		{"invalid -> 400", services.ErrWorkoutFeedbackInvalid, http.StatusBadRequest},
		{"forbidden -> 403", services.ErrWorkoutFeedbackForbidden, http.StatusForbidden},
		{"not found -> 404", daos.ErrWorkoutFeedbackNotFound, http.StatusNotFound},
		{"team not found -> 404", services.ErrTeamNotFound, http.StatusNotFound},
		{"duplicate -> 409", daos.ErrWorkoutFeedbackDuplicate, http.StatusConflict},
		{"unknown -> 500", assert.AnError, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := &mockWorkoutFeedbackService{
				createFn: func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
					return nil, tc.serviceErr
				},
			}
			controller := newTestWorkoutFeedbackController(mockSvc)

			response := httptest.NewRecorder()
			body := `{"assigned_session_id":1,"assigned_exercise_id":2,"report_source":"corredor","session_date":"2026-01-15"}`
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			setAuthUserID(c, 7)
			controller.Create(c)

			assert.Equal(t, tc.expectedCode, response.Code)
		})
	}
}

func TestWorkoutFeedbackController_GetByID_Success(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		getByIDFn: func(ctx *gin.Context, authUserID, feedbackID int64) (*dbs.WorkoutFeedback, error) {
			assert.Equal(t, int64(1), feedbackID)
			return fixtureFeedback(), nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/workout-feedback/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 7)
	controller.GetByID(c)

	require.Equal(t, http.StatusOK, response.Code)
	var resp workoutfeedback.WorkoutFeedbackResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, "corredor", resp.ReportSource)
	assert.Equal(t, []string{"https://media.foo/a.jpg"}, resp.MediaURLs)
}

func TestWorkoutFeedbackController_GetByID_InvalidID(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/workout-feedback/abc", nil)
	setAuthUserID(c, 7)
	controller.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_Search_Success(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		searchFn: func(ctx *gin.Context, authUserID int64, filters workoutfeedback.SearchFilters) ([]dbs.WorkoutFeedback, error) {
			return []dbs.WorkoutFeedback{*fixtureFeedback()}, nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/workout-feedback/search", nil)
	setAuthUserID(c, 7)
	controller.Search(c)

	require.Equal(t, http.StatusOK, response.Code)
	var resp workoutfeedback.SearchResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, []string{"https://media.foo/a.jpg"}, resp.Data[0].MediaURLs)
}

func TestWorkoutFeedbackController_Search_InvalidParam(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/workout-feedback/search?team_id=abc", nil)
	setAuthUserID(c, 7)
	controller.Search(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_Update_Success(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		updateFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error) {
			return fixtureFeedback(), nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	body := `{"rpe":8}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/workout-feedback/1", strings.NewReader(body))
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Update(c)

	require.Equal(t, http.StatusOK, response.Code)
	var resp workoutfeedback.MutationResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, workoutfeedback.MsgFeedbackUpdated, resp.Message)
}

func TestWorkoutFeedbackController_Update_InvalidID(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/workout-feedback/0", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_Delete_Success(t *testing.T) {
	var deleted bool
	mockSvc := &mockWorkoutFeedbackService{
		softDeleteFn: func(ctx *gin.Context, authUserID, feedbackID int64) error {
			deleted = true
			return nil
		},
	}
	router := setupWorkoutFeedbackRouter(mockSvc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/workout-feedback/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, deleted)
}

func TestWorkoutFeedbackController_Delete_Forbidden(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		softDeleteFn: func(ctx *gin.Context, authUserID, feedbackID int64) error {
			return services.ErrWorkoutFeedbackForbidden
		},
	}
	router := setupWorkoutFeedbackRouter(mockSvc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/workout-feedback/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// setupWorkoutFeedbackRouter arma un router de prueba que inyecta auth_user_id y
// mapea las rutas del módulo (mismo patrón que setupSessionRouter).
func setupWorkoutFeedbackRouter(svc services.WorkoutFeedbackServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := newTestWorkoutFeedbackController(svc)
	r.POST("/workout-feedback", ctrl.Create)
	r.GET("/workout-feedback/:id/points", ctrl.GetPoints)
	r.POST("/workout-feedback/:id/points", ctrl.CreatePoints)
	r.GET("/workout-feedback/:id", ctrl.GetByID)
	r.GET("/workout-feedback/search", ctrl.Search)
	r.PUT("/workout-feedback/:id", ctrl.Update)
	r.DELETE("/workout-feedback/:id", ctrl.Delete)
	return r
}

func TestWorkoutFeedbackController_CreatePoints_Success(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		createPointsFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*services.PointsResult, error) {
			assert.Equal(t, int64(1), feedbackID)
			return &services.PointsResult{Created: 2, Skipped: 1}, nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	body := `{"points":[
		{"order":0,"session_instance_id":3,"exercise_instance_id":4,"latitude":-34.6,"longitude":-58.4,"recorded_at":"2026-09-24T14:00:00Z"},
		{"order":1,"session_instance_id":3,"exercise_instance_id":4,"latitude":-34.61,"longitude":-58.41,"recorded_at":"2026-09-24T14:00:01Z"}
	]}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback/1/points", strings.NewReader(body))
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.CreatePoints(c)

	require.Equal(t, http.StatusCreated, response.Code)
	var resp workoutfeedback.PointsMutationResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, workoutfeedback.MsgPointsCreated, resp.Message)
	assert.Equal(t, 2, resp.Data.Created)
	assert.Equal(t, 1, resp.Data.Skipped)
}

func TestWorkoutFeedbackController_CreatePoints_InvalidPayload(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback/1/points", strings.NewReader(`{"points":"x"}`))
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.CreatePoints(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_CreatePoints_InvalidID(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback/abc/points", strings.NewReader(`{"points":[{"order":0,"recorded_at":"2026-09-24T14:00:00Z"}]}`))
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}
	c.Request.Header.Set("Content-Type", "application/json")
	setAuthUserID(c, 7)
	controller.CreatePoints(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestWorkoutFeedbackController_CreatePoints_ErrorMapping(t *testing.T) {
	cases := []struct {
		name         string
		serviceErr   error
		expectedCode int
	}{
		{"invalid -> 400", services.ErrWorkoutFeedbackInvalid, http.StatusBadRequest},
		{"forbidden -> 403", services.ErrWorkoutFeedbackForbidden, http.StatusForbidden},
		{"not found -> 404", daos.ErrWorkoutFeedbackNotFound, http.StatusNotFound},
		{"unknown -> 500", assert.AnError, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := &mockWorkoutFeedbackService{
				createPointsFn: func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.CreatePointsRequest) (*services.PointsResult, error) {
					return nil, tc.serviceErr
				},
			}
			controller := newTestWorkoutFeedbackController(mockSvc)

			response := httptest.NewRecorder()
			body := `{"points":[{"order":0,"recorded_at":"2026-09-24T14:00:00Z"}]}`
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/workout-feedback/1/points", strings.NewReader(body))
			c.Params = []gin.Param{{Key: "id", Value: "1"}}
			c.Request.Header.Set("Content-Type", "application/json")
			setAuthUserID(c, 7)
			controller.CreatePoints(c)

			assert.Equal(t, tc.expectedCode, response.Code)
		})
	}
}

func TestWorkoutFeedbackController_GetPoints_Success(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		getPointsFn: func(ctx *gin.Context, authUserID, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
			assert.Equal(t, int64(1), feedbackID)
			return []dbs.WorkoutFeedbackPoint{
				{ID: 11, FeedbackID: 1, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)},
			}, nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/workout-feedback/1/points", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 7)
	controller.GetPoints(c)

	require.Equal(t, http.StatusOK, response.Code)
	var resp workoutfeedback.PointsListResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, 0, resp.Data[0].Order)
	assert.Equal(t, -34.6, resp.Data[0].Latitude)
}

func TestWorkoutFeedbackController_GetBySession_Success(t *testing.T) {
	var gotAuth, gotSession int64
	var gotAthlete *int64
	mockSvc := &mockWorkoutFeedbackService{
		getSessionFeedbackFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
			gotAuth = authUserID
			gotSession = sessionInstanceID
			gotAthlete = athleteUserID
			return []dbs.WorkoutFeedback{
				{ID: 1, AssignedSessionID: 10, AssignedExerciseID: 2, AthleteUserID: 7, FeedbackOwnerUserID: 7, ReportSource: "corredor", SetNumber: 1},
			}, nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/feedback", nil)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	setAuthUserID(c, 7)
	controller.GetBySession(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(7), gotAuth)
	assert.Equal(t, int64(10), gotSession)
	assert.Nil(t, gotAthlete)

	var resp workoutfeedback.SearchResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, int64(1), resp.Data[0].ID)
	assert.Equal(t, "corredor", resp.Data[0].ReportSource)
}

func TestWorkoutFeedbackController_GetBySession_ForwardsAthleteQuery(t *testing.T) {
	var gotAthlete *int64
	mockSvc := &mockWorkoutFeedbackService{
		getSessionFeedbackFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
			gotAthlete = athleteUserID
			return []dbs.WorkoutFeedback{}, nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/feedback?athlete_user_id=30", nil)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	setAuthUserID(c, 7)
	controller.GetBySession(c)

	require.Equal(t, http.StatusOK, response.Code)
	require.NotNil(t, gotAthlete)
	assert.Equal(t, int64(30), *gotAthlete)
}

func TestWorkoutFeedbackController_GetBySession_Unauthorized(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/feedback", nil)
	controller.GetBySession(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestWorkoutFeedbackController_GetBySession_InvalidParams(t *testing.T) {
	cases := []struct {
		name  string
		param string
		query string
	}{
		{"path no numerico", "abc", ""},
		{"path 0", "0", ""},
		{"query invalido", "10", "?athlete_user_id=abc"},
		{"query 0", "10", "?athlete_user_id=0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/feedback"+tc.query, nil)
			c.Params = []gin.Param{{Key: "id", Value: tc.param}}
			setAuthUserID(c, 7)
			controller.GetBySession(c)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestWorkoutFeedbackController_GetBySession_Forbidden(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		getSessionFeedbackFn: func(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
			return nil, services.ErrWorkoutFeedbackForbidden
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/session-instances/10/feedback", nil)
	c.Params = []gin.Param{{Key: "id", Value: "10"}}
	setAuthUserID(c, 7)
	controller.GetBySession(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestWorkoutFeedbackController_GetPoints_InvalidID(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/workout-feedback/0/points", nil)
	c.Params = []gin.Param{{Key: "id", Value: "0"}}
	setAuthUserID(c, 7)
	controller.GetPoints(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

// fixtureHistoryResponse arma la respuesta del service con un item y pools,
// para verificar el shape de los endpoints de historial.
func fixtureHistoryResponse() *workoutfeedback.WorkoutFeedbackHistoryResponse {
	return &workoutfeedback.WorkoutFeedbackHistoryResponse{
		Items: []workoutfeedback.WorkoutFeedbackHistoryItem{{
			ID:            1,
			AthleteUserID: 7,
			AthleteName:   "Anita",
			ExerciseID:    2,
			Date:          "2026-01-15",
			SetNumber:     1,
		}},
		Total:              1,
		Page:               1,
		PageSize:           20,
		AvailableAthletes:  []dbs.IDName{{ID: 7, Name: "Anita"}},
		AvailableExercises: []dbs.IDName{{ID: 2, Name: "Sentadilla"}},
	}
}

func TestWorkoutFeedbackController_AthleteHistory_Success(t *testing.T) {
	var gotCaller, gotTarget int64
	var gotQuery workoutfeedback.HistoryQuery
	mockSvc := &mockWorkoutFeedbackService{
		athleteHistoryFn: func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
			gotCaller, gotTarget, gotQuery = callerID, targetID, query
			return fixtureHistoryResponse(), nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/7/workout-feedback-history?team_id=4&group_id=6&date_from=2026-01-01&date_to=2026-01-31&exercise_id=2&set_number=1&sort=exercise_name&order=asc&page=2&page_size=10", nil)
	c.Params = []gin.Param{{Key: "id", Value: "7"}}
	setAuthUserID(c, 7)
	controller.AthleteHistory(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(7), gotCaller)
	assert.Equal(t, int64(7), gotTarget)
	require.NotNil(t, gotQuery.TeamID)
	assert.Equal(t, int64(4), *gotQuery.TeamID)
	require.NotNil(t, gotQuery.GroupID)
	assert.Equal(t, int64(6), *gotQuery.GroupID)
	require.NotNil(t, gotQuery.ExerciseID)
	assert.Equal(t, int64(2), *gotQuery.ExerciseID)
	require.NotNil(t, gotQuery.SetNumber)
	assert.Equal(t, 1, *gotQuery.SetNumber)
	require.NotNil(t, gotQuery.DateFrom)
	assert.Equal(t, "2026-01-01", *gotQuery.DateFrom)
	require.NotNil(t, gotQuery.DateTo)
	assert.Equal(t, "2026-01-31", *gotQuery.DateTo)
	assert.Equal(t, "exercise_name", gotQuery.Sort)
	assert.Equal(t, "asc", gotQuery.Order)
	require.NotNil(t, gotQuery.Page)
	assert.Equal(t, 2, *gotQuery.Page)
	require.NotNil(t, gotQuery.PageSize)
	assert.Equal(t, 10, *gotQuery.PageSize)

	var resp workoutfeedback.WorkoutFeedbackHistoryResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "Anita", resp.Items[0].AthleteName)
	assert.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.AvailableAthletes, 1)
	assert.Equal(t, "Anita", resp.AvailableAthletes[0].Name)
}

func TestWorkoutFeedbackController_AthleteHistory_IgnoresAthleteUserID(t *testing.T) {
	var gotQuery workoutfeedback.HistoryQuery
	mockSvc := &mockWorkoutFeedbackService{
		athleteHistoryFn: func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
			gotQuery = query
			return fixtureHistoryResponse(), nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/7/workout-feedback-history?athlete_user_id=5", nil)
	c.Params = []gin.Param{{Key: "id", Value: "7"}}
	setAuthUserID(c, 7)
	controller.AthleteHistory(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Nil(t, gotQuery.AthleteUserID)
}

func TestWorkoutFeedbackController_History_Unauthorized(t *testing.T) {
	controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/7/workout-feedback-history", nil)
	controller.AthleteHistory(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestWorkoutFeedbackController_AthleteHistory_OtherUserForbidden(t *testing.T) {
	mockSvc := &mockWorkoutFeedbackService{
		athleteHistoryFn: func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
			t.Fatal("el service no debió invocarse")
			return nil, nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/99/workout-feedback-history", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99"}}
	setAuthUserID(c, 7)
	controller.AthleteHistory(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestWorkoutFeedbackController_AdministeredHistory_Success(t *testing.T) {
	var gotCaller, gotTarget int64
	var gotQuery workoutfeedback.HistoryQuery
	mockSvc := &mockWorkoutFeedbackService{
		administeredHistoryFn: func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
			gotCaller, gotTarget, gotQuery = callerID, targetID, query
			return fixtureHistoryResponse(), nil
		},
	}
	controller := newTestWorkoutFeedbackController(mockSvc)

	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/7/administered-workout-feedback-history?team_id=4&athlete_user_id=9&set_number=2", nil)
	c.Params = []gin.Param{{Key: "id", Value: "7"}}
	setAuthUserID(c, 7)
	controller.AdministeredHistory(c)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(7), gotCaller)
	assert.Equal(t, int64(7), gotTarget)
	require.NotNil(t, gotQuery.TeamID)
	assert.Equal(t, int64(4), *gotQuery.TeamID)
	require.NotNil(t, gotQuery.AthleteUserID)
	assert.Equal(t, int64(9), *gotQuery.AthleteUserID)
	require.NotNil(t, gotQuery.SetNumber)
	assert.Equal(t, 2, *gotQuery.SetNumber)
}

func TestWorkoutFeedbackController_History_InvalidParams(t *testing.T) {
	cases := []struct {
		name       string
		endpoint   string
		query      string
		callsWrong string
	}{
		{"path no numerico", "AthleteHistory", "?", "abc"},
		{"path 0", "AthleteHistory", "?", "0"},
		{"team_id invalido", "AthleteHistory", "?team_id=abc", "7"},
		{"team_id 0", "AthleteHistory", "?team_id=0", "7"},
		{"exercise_id invalido", "AthleteHistory", "?exercise_id=x", "7"},
		{"set_number no numerico", "AthleteHistory", "?set_number=abc", "7"},
		{"page no numerico", "AthleteHistory", "?page=primera", "7"},
		{"page_size no numerico", "AthleteHistory", "?page_size=veinte", "7"},
		{"athlete_user_id invalido", "AdministeredHistory", "?athlete_user_id=x", "7"},
		{"athlete_user_id 0", "AdministeredHistory", "?athlete_user_id=0", "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := newTestWorkoutFeedbackController(&mockWorkoutFeedbackService{})

			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			var handler func(c *gin.Context)
			if tc.endpoint == "AdministeredHistory" {
				handler = controller.AdministeredHistory
			} else {
				handler = controller.AthleteHistory
			}
			path := "/api/v1/users/" + tc.callsWrong + "/workout-feedback-history"
			if tc.endpoint == "AdministeredHistory" {
				path = "/api/v1/users/" + tc.callsWrong + "/administered-workout-feedback-history"
			}
			c.Request, _ = http.NewRequest(http.MethodGet, path+tc.query, nil)
			c.Params = []gin.Param{{Key: "id", Value: tc.callsWrong}}
			setAuthUserID(c, 7)
			handler(c)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestWorkoutFeedbackController_History_ServiceErrorMapping(t *testing.T) {
	cases := []struct {
		name         string
		serviceErr   error
		expectedCode int
	}{
		{"forbidden -> 403", services.ErrWorkoutFeedbackForbidden, http.StatusForbidden},
		{"team not found -> 404", services.ErrTeamNotFound, http.StatusNotFound},
		{"invalid -> 400", services.ErrWorkoutFeedbackInvalid, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := &mockWorkoutFeedbackService{
				athleteHistoryFn: func(ctx *gin.Context, callerID, targetID int64, query workoutfeedback.HistoryQuery) (*workoutfeedback.WorkoutFeedbackHistoryResponse, error) {
					return nil, tc.serviceErr
				},
			}
			controller := newTestWorkoutFeedbackController(mockSvc)

			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/7/workout-feedback-history", nil)
			c.Params = []gin.Param{{Key: "id", Value: "7"}}
			setAuthUserID(c, 7)
			controller.AthleteHistory(c)

			assert.Equal(t, tc.expectedCode, response.Code)
		})
	}
}
