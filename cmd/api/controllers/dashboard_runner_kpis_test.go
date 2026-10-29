package controllers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
)

func TestDashboardController_RunnerKpisQueryValida(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodGet,
		"/api/v1/dashboard/runner-kpis?period=week&reference_date=2026-10-28&tab=ritmo_continuo", "", true)

	NewDashboardController(service).RunnerKpis(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int64(42), service.lastCallerID)
	assert.Equal(t, services.RunnerKpisQuery{
		Period:        "week",
		ReferenceDate: "2026-10-28",
		Tab:           "ritmo_continuo",
	}, service.lastRunnerQuery)
}

func TestDashboardController_RunnerKpisDefaultsSeResuelvenEnElService(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/runner-kpis", "", true)

	NewDashboardController(service).RunnerKpis(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, services.RunnerKpisQuery{}, service.lastRunnerQuery)
}

func TestDashboardController_RunnerKpisIgnoraAthleteUserIDAjeno(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodGet,
		"/api/v1/dashboard/runner-kpis?athlete_user_id=999", "", true)

	NewDashboardController(service).RunnerKpis(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int64(42), service.lastCallerID, "el corredor sale del token, no del query")
	assert.Equal(t, services.RunnerKpisQuery{}, service.lastRunnerQuery)
}

func TestDashboardController_RunnerKpisSinTokenEs401(t *testing.T) {
	service := &stubDashboardService{}
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/runner-kpis", "", false)

	NewDashboardController(service).RunnerKpis(ctx)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Zero(t, service.lastCallerID, "sin token no se ejecuta el service")
}

func TestDashboardController_RunnerKpisParamErrorConSuCode(t *testing.T) {
	for _, code := range []string{"INVALID_PERIOD", "INVALID_DATE", "INVALID_TAB"} {
		t.Run(code, func(t *testing.T) {
			ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/runner-kpis", "", true)

			NewDashboardController(&stubDashboardService{
				err: &services.DashboardParamError{Code: code, Message: "parámetro inválido"},
			}).RunnerKpis(ctx)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, code, decodeAPIError(t, recorder).Code)
		})
	}
}

// El contrato estable: todas las claves de metrics presentes y units completo,
// con null serializado como null (nunca 0) cuando no hay dato.
func TestDashboardController_RunnerKpisContratoJSON(t *testing.T) {
	response := &dashboard.RunnerKpisResponse{
		Period:         "month",
		ReferenceDate:  "2026-10-28",
		From:           "2026-10-01",
		To:             "2026-10-31",
		DaysConsidered: 31,
		Tab:            "velocidad_tiempo",
		Units:          dashboard.RunnerKpisUnitsBlock,
		HasData:        false,
	}
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/runner-kpis", "", true)

	NewDashboardController(&stubDashboardService{response: response}).RunnerKpis(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	for _, key := range []string{
		"period", "reference_date", "from", "to", "days_considered", "tab", "units",
		"has_data", "series_count", "valid_series", "sessions", "metrics",
	} {
		assert.Contains(t, body, key)
	}

	units, ok := body["units"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"distance": "m", "speed": "km/h", "time": "ms"}, units)

	metrics, ok := body["metrics"].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{
		"total_distance_m", "best_day_distance_m", "best_day_date",
		"average_speed_kph", "best_speed_kph", "best_time_ms", "best_distance_m",
	} {
		value, present := metrics[key]
		assert.True(t, present, "%s tiene que estar presente", key)
		assert.Nil(t, value, "%s tiene que serializarse como null", key)
	}
}

func TestDashboardController_RunnerKpisError500(t *testing.T) {
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/runner-kpis", "", true)

	NewDashboardController(&stubDashboardService{err: assert.AnError}).RunnerKpis(ctx)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}
