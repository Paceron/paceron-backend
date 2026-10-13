package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// stubDashboardService registra la query que arma el controller y devuelve lo que el
// test le pida. El controller no tiene lógica: su contrato es parsear y mapear.
type stubDashboardService struct {
	lastQuery      services.MetricsQuery
	lastCallerID   int64
	lastRequest    any
	lastExerciseID int64
	response       any
	err            error
	filtersTeamID  int64
}

func (s *stubDashboardService) Filters(c *gin.Context, callerID, teamID int64) (*dashboard.FiltersResponse, error) {
	s.lastCallerID, s.filtersTeamID = callerID, teamID
	if s.err != nil {
		return nil, s.err
	}
	if typed, ok := s.response.(*dashboard.FiltersResponse); ok {
		return typed, nil
	}
	return &dashboard.FiltersResponse{}, nil
}

func (s *stubDashboardService) Summary(c *gin.Context, callerID int64, query services.MetricsQuery) (*dashboard.SummaryResponse, error) {
	s.lastCallerID, s.lastQuery = callerID, query
	if s.err != nil {
		return nil, s.err
	}
	return &dashboard.SummaryResponse{}, nil
}

func (s *stubDashboardService) TimeSeries(c *gin.Context, callerID int64, query services.MetricsQuery) (*dashboard.TimeseriesResponse, error) {
	s.lastCallerID, s.lastQuery = callerID, query
	if s.err != nil {
		return nil, s.err
	}
	return &dashboard.TimeseriesResponse{}, nil
}

func (s *stubDashboardService) Leaderboard(c *gin.Context, callerID int64, query services.MetricsQuery) (*dashboard.LeaderboardResponse, error) {
	s.lastCallerID, s.lastQuery = callerID, query
	if s.err != nil {
		return nil, s.err
	}
	return &dashboard.LeaderboardResponse{}, nil
}

func (s *stubDashboardService) ExerciseSeries(c *gin.Context, callerID, catalogExerciseID int64, query services.MetricsQuery) (*dashboard.SeriesResponse, error) {
	s.lastCallerID, s.lastQuery, s.lastExerciseID = callerID, query, catalogExerciseID
	if s.err != nil {
		return nil, s.err
	}
	return &dashboard.SeriesResponse{}, nil
}

func (s *stubDashboardService) Comparisons(c *gin.Context, callerID int64, request dashboard.ComparisonsRequest) (*dashboard.ComparisonsResponse, error) {
	s.lastCallerID, s.lastRequest = callerID, request
	if s.err != nil {
		return nil, s.err
	}
	return &dashboard.ComparisonsResponse{}, nil
}

func (s *stubDashboardService) Periods(c *gin.Context, callerID int64, request dashboard.PeriodsRequest) (*dashboard.PeriodsResponse, error) {
	s.lastCallerID, s.lastRequest = callerID, request
	if s.err != nil {
		return nil, s.err
	}
	return &dashboard.PeriodsResponse{}, nil
}

func dashboardTestContext(t *testing.T, method, target, body string, withAuth bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if withAuth {
		ctx.Set(utils.AuthUserIDKey, int64(42))
	}
	return ctx, recorder
}

func decodeAPIError(t *testing.T, recorder *httptest.ResponseRecorder) apierror.APIError {
	t.Helper()
	var apiErr apierror.APIError
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &apiErr))
	return apiErr
}

// El 400 del dashboard lleva su code estable: el cliente no debe ofrecer reintentar
// la misma request.
func TestDashboardController_ParamErrorConSuCode(t *testing.T) {
	paramErr := &services.DashboardParamError{Code: "INVALID_TOLERANCE", Message: "tolerance fuera de rango"}

	cases := []struct {
		name string
		call func(DashboardController, *gin.Context)
	}{
		{"summary", func(dc DashboardController, c *gin.Context) { dc.Summary(c) }},
		{"timeseries", func(dc DashboardController, c *gin.Context) { dc.TimeSeries(c) }},
		{"leaderboard", func(dc DashboardController, c *gin.Context) { dc.Leaderboard(c) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := dashboardTestContext(t, http.MethodGet, "/?team_id=29&from=2026-09-01", "", true)

			tc.call(NewDashboardController(&stubDashboardService{err: paramErr}), ctx)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			apiErr := decodeAPIError(t, recorder)
			assert.Equal(t, "INVALID_TOLERANCE", apiErr.Code)
			assert.Equal(t, "tolerance fuera de rango", apiErr.Message)
		})
	}
}

func TestDashboardController_ForbiddenYNotFound(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"scope ajeno", services.ErrDashboardForbidden, http.StatusForbidden, "FORBIDDEN"},
		{"inexistente", services.ErrDashboardNotFound, http.StatusNotFound, "NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := dashboardTestContext(t, http.MethodGet, "/?team_id=29&from=2026-09-01", "", true)

			NewDashboardController(&stubDashboardService{err: tc.err}).Summary(ctx)

			assert.Equal(t, tc.status, recorder.Code)
			assert.Equal(t, tc.code, decodeAPIError(t, recorder).Code)
		})
	}
}

// El 403 de /comparisons nombra el corredor en `details`, para que el front pueda
// marcar la fila culpable sin parsear el mensaje.
func TestDashboardController_AthleteOutOfScopeEnDetails(t *testing.T) {
	ctx, recorder := dashboardTestContext(t, http.MethodPost, "/",
		`{"team_id":29,"from":"2026-09-01","to":"2026-09-30","athlete_user_ids":[1,9]}`, true)

	NewDashboardController(&stubDashboardService{
		err: &services.DashboardAthleteOutOfScope{AthleteUserID: 9},
	}).Comparisons(ctx)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	apiErr := decodeAPIError(t, recorder)
	assert.Equal(t, "ATHLETE_OUT_OF_SCOPE", apiErr.Code)
	assert.Equal(t, map[string]interface{}{"athlete_user_id": float64(9)}, apiErr.Details)
}

// team_id es obligatorio en los siete endpoints: el acceso es por pestaña del
// detalle de equipo.
func TestDashboardController_TeamIDObligatorio(t *testing.T) {
	service := &stubDashboardService{}
	dc := NewDashboardController(service)
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/?from=2026-09-01", "", true)

	dc.Filters(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "MISSING_TEAM", decodeAPIError(t, recorder).Code)
}

// El controller arma la query con los query params y deja los defaults al service.
func TestDashboardController_ArmaLaQuery(t *testing.T) {
	service := &stubDashboardService{}
	ctx, _ := dashboardTestContext(t, http.MethodGet,
		"/?team_id=29&group_id=39&exercise_id=7&kinds=ritmo_continuo&from=2026-09-01&to=2026-09-30&tolerance=0.1&granularity=week&page=2&page_size=50&sort=mean_speed_kph&order=desc", "", true)

	NewDashboardController(service).Leaderboard(ctx)

	assert.Equal(t, int64(42), service.lastCallerID)
	assert.Equal(t, int64(29), service.lastQuery.TeamID)
	require.NotNil(t, service.lastQuery.GroupID)
	assert.Equal(t, int64(39), *service.lastQuery.GroupID)
	require.NotNil(t, service.lastQuery.ExerciseID)
	assert.Equal(t, int64(7), *service.lastQuery.ExerciseID)
	assert.Equal(t, []string{"ritmo_continuo"}, service.lastQuery.Kinds)
	assert.Equal(t, "2026-09-01", service.lastQuery.From)
	assert.Equal(t, "2026-09-30", service.lastQuery.To)
	assert.Equal(t, "0.1", service.lastQuery.Tolerance)
	assert.Equal(t, "week", service.lastQuery.Granularity)
	assert.Equal(t, "2", service.lastQuery.Page)
	assert.Equal(t, "50", service.lastQuery.PageSize)
	assert.Equal(t, "mean_speed_kph", service.lastQuery.Sort)
	assert.Equal(t, "desc", service.lastQuery.Order)
}

// exercise_ids[] y kinds[] llegan como query params repetidos.
func TestDashboardController_QueryParamsRepetidos(t *testing.T) {
	service := &stubDashboardService{}
	ctx, _ := dashboardTestContext(t, http.MethodGet,
		"/?team_id=29&from=2026-09-01&exercise_ids=7&exercise_ids=8&kinds=ritmo_continuo&kinds=elongacion", "", true)

	NewDashboardController(service).Summary(ctx)

	assert.Equal(t, []int64{7, 8}, service.lastQuery.ExerciseIDs)
	assert.Equal(t, []string{"ritmo_continuo", "elongacion"}, service.lastQuery.Kinds)
}

// /exercises/{id}/series: el id del path es el de catálogo y llega como int64.
func TestDashboardController_ExerciseSeriesTomaElIDDelPath(t *testing.T) {
	service := &stubDashboardService{}
	ctx, _ := dashboardTestContext(t, http.MethodGet, "/?team_id=29&from=2026-09-01", "", true)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	NewDashboardController(service).ExerciseSeries(ctx)

	assert.Equal(t, int64(7), service.lastExerciseID)
	assert.Equal(t, int64(29), service.lastQuery.TeamID)
}

func TestDashboardController_ExerciseSeriesIDInvalido(t *testing.T) {
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/?team_id=29&from=2026-09-01", "", true)
	ctx.Params = gin.Params{{Key: "id", Value: "abc"}}

	NewDashboardController(&stubDashboardService{}).ExerciseSeries(ctx)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestDashboardController_BodyDeComparisons(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodPost, "/", `{
		"team_id": 29, "group_id": 39, "kinds": ["ritmo_continuo"],
		"from": "2026-09-01", "to": "2026-09-30", "tolerance": 0.1,
		"athlete_user_ids": [1, 2]
	}`, true)

	NewDashboardController(service).Comparisons(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	request, ok := service.lastRequest.(dashboard.ComparisonsRequest)
	require.True(t, ok)
	assert.Equal(t, int64(29), request.TeamID)
	require.NotNil(t, request.GroupID)
	assert.Equal(t, int64(39), *request.GroupID)
	assert.Equal(t, []string{"ritmo_continuo"}, request.Kinds)
	assert.Equal(t, []int64{1, 2}, request.AthleteUserIDs)
	require.NotNil(t, request.Tolerance)
	assert.InDelta(t, 0.1, *request.Tolerance, 0.0001)
}

func TestDashboardController_BodyDePeriods(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodPost, "/", `{
		"team_id": 29,
		"current": {"from": "2026-09-01", "to": "2026-09-30"}
	}`, true)

	NewDashboardController(service).Periods(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	request, ok := service.lastRequest.(dashboard.PeriodsRequest)
	require.True(t, ok)
	assert.Equal(t, "2026-09-01", request.Current.From)
	assert.Nil(t, request.Previous, "previous es opcional: sin él lo deriva el service")
}

// Sin token, 401 y nada de consulta.
func TestDashboardController_SinAutenticacion(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/?team_id=29&from=2026-09-01", "", false)

	NewDashboardController(service).Summary(ctx)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Nil(t, service.lastRequest)
}
