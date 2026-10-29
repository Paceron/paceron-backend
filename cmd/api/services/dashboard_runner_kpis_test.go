package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
)

func runnerKpisRow(athleteID, sessionID int64, kind string, date time.Time, distanceM float64, timeMs int64) daos.DashboardSeriesRow {
	return daos.DashboardSeriesRow{
		FeedbackID:          sessionID*10 + timeMs%10,
		SessionInstanceID:   sessionID,
		ExerciseInstanceID:  sessionID,
		SessionDate:         date,
		BucketStart:         date,
		AthleteUserID:       athleteID,
		FeedbackOwnerUserID: athleteID,
		SetNumber:           1,
		Kind:                kind,
		DurationMs:          &timeMs,
		ActiveDurationMs:    &timeMs,
		DistanceMeters:      &distanceM,
		HasRunnerSession:    true,
	}
}

func TestDashboardService_RunnerKpisDefaults(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 77, false)
	var gotAthleteID int64
	var gotFrom, gotTo time.Time
	dao.findRunnerFn = func(_ *gin.Context, athleteUserID int64, from, to time.Time) ([]daos.DashboardSeriesRow, error) {
		gotAthleteID, gotFrom, gotTo = athleteUserID, from, to
		return nil, nil
	}

	response, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{})

	require.NoError(t, err)
	assert.Equal(t, "month", response.Period)
	assert.Equal(t, string(dashboard.RunnerKpiTabGeneral), response.Tab)
	assert.Equal(t, dashboard.FormatDate(time.Now()), response.ReferenceDate)
	assert.Equal(t, int64(77), gotAthleteID)
	firstOfMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	assert.Equal(t, dashboard.FormatDate(firstOfMonth), dashboard.FormatDate(gotFrom))
	assert.Equal(t, dashboard.FormatDate(lastOfMonth), dashboard.FormatDate(gotTo))
	assert.False(t, response.HasData)
	assert.Zero(t, response.SeriesCount)
	assert.Zero(t, response.ValidSeries)
	assert.Zero(t, response.Sessions)
	assert.Nil(t, response.Metrics.TotalDistanceM)
}

func TestDashboardService_RunnerKpisPeriodoYFecha(t *testing.T) {
	cases := []struct {
		period string
		from   string
		to     string
		days   int
	}{
		{"month", "2026-10-01", "2026-10-31", 31},
		{"week", "2026-10-26", "2026-11-01", 7},
		{"day", "2026-10-28", "2026-10-28", 1},
	}
	for _, tc := range cases {
		t.Run(tc.period, func(t *testing.T) {
			service, _, _ := dashboardFixtureAs(t, 77, false)

			response, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{
				Period: tc.period, ReferenceDate: "2026-10-28", Tab: "general",
			})

			require.NoError(t, err)
			assert.Equal(t, tc.period, response.Period)
			assert.Equal(t, "2026-10-28", response.ReferenceDate)
			assert.Equal(t, tc.from, response.From)
			assert.Equal(t, tc.to, response.To)
			assert.Equal(t, tc.days, response.DaysConsidered)
		})
	}
}

// Los tres parámetros se rechazan con su code ANTES de tocar la base.
func TestDashboardService_RunnerKpisParametrosInvalidos(t *testing.T) {
	cases := []struct {
		name  string
		query RunnerKpisQuery
		code  string
	}{
		{
			name:  "period desconocido",
			query: RunnerKpisQuery{Period: "year"},
			code:  constants.ErrorCodeInvalidPeriod,
		},
		{
			name:  "fecha imposible",
			query: RunnerKpisQuery{ReferenceDate: "2026-02-30"},
			code:  constants.ErrorCodeInvalidDate,
		},
		{
			name:  "fecha con formato inválido",
			query: RunnerKpisQuery{ReferenceDate: "28/10/2026"},
			code:  constants.ErrorCodeInvalidDate,
		},
		{
			name:  "tab desconocido",
			query: RunnerKpisQuery{Tab: "comparacion"},
			code:  constants.ErrorCodeInvalidTab,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, dao, _ := dashboardFixtureAs(t, 77, false)

			_, err := service.RunnerKpis(nil, 77, tc.query)

			assert.Equal(t, tc.code, paramErrorCode(t, err))
			assert.Zero(t, dao.calls, "el 400 tiene que caer antes de la consulta")
		})
	}
}

// El alcance es el token: el service pasa el callerID y no existe ningún campo
// de athlete_user_id en la query.
func TestDashboardService_RunnerKpisUsaElCallerYNoAceptaOtroCorredor(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 77, false)
	calls := 0
	dao.findRunnerFn = func(_ *gin.Context, athleteUserID int64, _, _ time.Time) ([]daos.DashboardSeriesRow, error) {
		calls++
		assert.Equal(t, int64(77), athleteUserID)
		return nil, nil
	}

	_, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{Period: "day", ReferenceDate: "2026-10-28"})

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

// Dos grupos y dos equipos entran al mismo agregado sin team_id.
func TestDashboardService_RunnerKpisSumaDosEquipos(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 77, false)
	dao.findRunnerFn = func(_ *gin.Context, _ int64, _, _ time.Time) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{
			runnerKpisRow(77, 1, string(constants.ExerciseKindRitmoContinuo),
				time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC), 5000, 1800000),
			runnerKpisRow(77, 2, string(constants.ExerciseKindVelocidadDistancia),
				time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC), 2000, 600000),
		}, nil
	}

	response, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{
		Period: "month", ReferenceDate: "2026-10-28",
	})

	require.NoError(t, err)
	assert.True(t, response.HasData)
	assert.Equal(t, 2, response.SeriesCount)
	assert.Equal(t, 2, response.ValidSeries)
	assert.Equal(t, 2, response.Sessions)
	require.NotNil(t, response.Metrics.TotalDistanceM)
	assert.Equal(t, 7000.0, *response.Metrics.TotalDistanceM)
	require.NotNil(t, response.Metrics.AverageSpeedKph)
	assert.InDelta(t, 10.0, *response.Metrics.AverageSpeedKph, 0.0001,
		"general promedia solo ritmo_continuo")
	require.NotNil(t, response.Metrics.BestSpeedKph)
	assert.InDelta(t, 12.0, *response.Metrics.BestSpeedKph, 0.0001)
}

// series_count cuenta todas las filas de la pestaña; valid_series solo las
// calculables; sessions, los session_instance_id distintos.
func TestDashboardService_RunnerKpisContadores(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 77, false)
	dao.findRunnerFn = func(_ *gin.Context, _ int64, _, _ time.Time) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{
			runnerKpisRow(77, 1, string(constants.ExerciseKindRitmoContinuo),
				time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC), 5000, 1800000),
			// Ritmo continuo sin tiempo: cuenta como serie pero no es válida.
			runnerKpisRow(77, 1, string(constants.ExerciseKindRitmoContinuo),
				time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC), 4000, 0),
			runnerKpisRow(77, 2, string(constants.ExerciseKindVelocidadTiempo),
				time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC), 3000, 720000),
		}, nil
	}

	general, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{
		Period: "month", ReferenceDate: "2026-10-28", Tab: "general",
	})
	require.NoError(t, err)
	assert.Equal(t, 3, general.SeriesCount)
	assert.Equal(t, 2, general.ValidSeries)
	assert.Equal(t, 2, general.Sessions)

	ritmo, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{
		Period: "month", ReferenceDate: "2026-10-28", Tab: "ritmo_continuo",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, ritmo.SeriesCount)
	assert.Equal(t, 1, ritmo.ValidSeries)
	assert.Equal(t, 1, ritmo.Sessions)
}

// Una pestaña sin series propias no tiene datos aunque el período tenga otros
// tipos: los contadores quedan en cero y las métricas en null.
func TestDashboardService_RunnerKpisPestanaSinDatos(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 77, false)
	dao.findRunnerFn = func(_ *gin.Context, _ int64, _, _ time.Time) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{
			runnerKpisRow(77, 1, string(constants.ExerciseKindRitmoContinuo),
				time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC), 5000, 1800000),
		}, nil
	}

	response, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{
		Period: "month", ReferenceDate: "2026-10-28", Tab: "velocidad_tiempo",
	})

	require.NoError(t, err)
	assert.False(t, response.HasData)
	assert.Zero(t, response.SeriesCount)
	assert.Zero(t, response.ValidSeries)
	assert.Zero(t, response.Sessions)
	assert.Nil(t, response.Metrics.TotalDistanceM)
	assert.Nil(t, response.Metrics.BestTimeMs)
}

func TestDashboardService_RunnerKpisErrorDeDao(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 77, false)
	dao.findRunnerFn = func(_ *gin.Context, _ int64, _, _ time.Time) ([]daos.DashboardSeriesRow, error) {
		return nil, assert.AnError
	}

	_, err := service.RunnerKpis(nil, 77, RunnerKpisQuery{})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrDashboardInvalidQuery)
}
