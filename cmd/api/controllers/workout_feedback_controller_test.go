package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockWorkoutFeedbackService struct {
	createFn    func(ctx *gin.Context, authUserID int64, req workoutfeedback.CreateFeedbackRequest) (*dbs.WorkoutFeedback, error)
	getByIDFn   func(ctx *gin.Context, authUserID, feedbackID int64) (*dbs.WorkoutFeedback, error)
	searchFn    func(ctx *gin.Context, authUserID int64, filters workoutfeedback.SearchFilters) ([]dbs.WorkoutFeedback, error)
	updateFn    func(ctx *gin.Context, authUserID, feedbackID int64, req workoutfeedback.UpdateFeedbackRequest) (*dbs.WorkoutFeedback, error)
	softDeleteFn func(ctx *gin.Context, authUserID, feedbackID int64) error
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
	controller := NewWorkoutFeedbackController(&mockWorkoutFeedbackService{})

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
	controller := NewWorkoutFeedbackController(mockSvc)

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

func TestWorkoutFeedbackController_Create_MissingRequired(t *testing.T) {
	controller := NewWorkoutFeedbackController(&mockWorkoutFeedbackService{})

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
	controller := NewWorkoutFeedbackController(&mockWorkoutFeedbackService{})

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
			controller := NewWorkoutFeedbackController(mockSvc)

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
	controller := NewWorkoutFeedbackController(mockSvc)

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
	controller := NewWorkoutFeedbackController(&mockWorkoutFeedbackService{})

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
	controller := NewWorkoutFeedbackController(mockSvc)

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
	controller := NewWorkoutFeedbackController(&mockWorkoutFeedbackService{})

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
	controller := NewWorkoutFeedbackController(mockSvc)

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
	controller := NewWorkoutFeedbackController(&mockWorkoutFeedbackService{})

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
	ctrl := NewWorkoutFeedbackController(svc)
	r.POST("/workout-feedback", ctrl.Create)
	r.GET("/workout-feedback/:id", ctrl.GetByID)
	r.GET("/workout-feedback/search", ctrl.Search)
	r.PUT("/workout-feedback/:id", ctrl.Update)
	r.DELETE("/workout-feedback/:id", ctrl.Delete)
	return r
}