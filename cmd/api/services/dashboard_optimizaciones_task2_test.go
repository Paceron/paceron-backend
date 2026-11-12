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
	"simple-arq-golang/cmd/api/domains/dbs"
)

// task2QualityRows arma un result set con las cinco situaciones que mira el
// bloque de calidad: válidas con velocidad, inválida, legacy y suficientes
// velocidades para apagar insufficient_data.
func task2QualityRows() ([]daos.DashboardSeriesRow, int) {
	date := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	legacy := dashboardSeriesRow(1, 270000, 1000, date)
	legacy.HasRunnerSession = false
	return []daos.DashboardSeriesRow{
		dashboardSeriesRow(1, 240000, 1000, date),
		dashboardSeriesRow(2, 250000, 1000, date),
		dashboardSeriesRow(1, 260000, 1000, date),
		dashboardSeriesRow(2, 240000, 0, date),
		legacy,
	}, 4
}

func task2QualityService(t *testing.T, rows []daos.DashboardSeriesRow, open int) (*dashboardService, *mockDashboardDao, *mockDashboardScopeService) {
	t.Helper()
	service, dao, scopeService := dashboardFixture(t)
	dao.findSeriesFn = func(_ *gin.Context, _ daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		return rows, nil
	}
	dao.countOpenFn = func(_ *gin.Context, _ daos.DashboardScopeFilter) (daos.DashboardOpenSessions, error) {
		return daos.DashboardOpenSessions{Sessions: open}, nil
	}
	dao.unlinkedFn = func(_ *gin.Context, _ []int64, _, _ time.Time) ([]daos.UnlinkedExerciseInstance, error) {
		return nil, nil
	}
	dao.athletesFn = func(_ *gin.Context, _ []int64) ([]int64, error) {
		return []int64{1, 2}, nil
	}
	dao.catalogByIDFn = func(_ *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
		return &daos.DashboardCatalogExercises{
			ID: id, OwnerID: dashboardOwnerID, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo),
		}, nil
	}
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{
		{GroupID: 39, UserID: 1}, {GroupID: 39, UserID: 2},
	}}
	return service, dao, scopeService
}

// Los cinco callers que solo necesitan data_quality devuelven el mismo bloque que
// BuildSummary armaría para el mismo result set.
func TestDashboardService_DataQualityLigeraEnCallers(t *testing.T) {
	rows, open := task2QualityRows()
	expected := dashboard.BuildSummary(mapSeriesRows(rows), dashboard.DefaultTolerance, open).DataQuality
	require.Equal(t, 1, expected.InvalidSeries)
	require.Equal(t, 1, expected.LegacySessions)
	require.False(t, expected.InsufficientData)
	require.Equal(t, open, expected.OpenSessions)

	cases := map[string]func(*dashboardService) (dashboard.DataQuality, error){
		"leaderboard": func(s *dashboardService) (dashboard.DataQuality, error) {
			response, err := s.Leaderboard(nil, dashboardOwnerID, dashboardQuery())
			if err != nil {
				return dashboard.DataQuality{}, err
			}
			return response.DataQuality, nil
		},
		"exercise series": func(s *dashboardService) (dashboard.DataQuality, error) {
			response, err := s.ExerciseSeries(nil, dashboardOwnerID, 7, dashboardQuery())
			if err != nil {
				return dashboard.DataQuality{}, err
			}
			return response.DataQuality, nil
		},
		"comparisons": func(s *dashboardService) (dashboard.DataQuality, error) {
			response, err := s.Comparisons(nil, dashboardOwnerID, dashboard.ComparisonsRequest{
				TeamID: 29, From: "2026-09-01", To: "2026-09-30", AthleteUserIDs: []int64{1, 2},
			})
			if err != nil {
				return dashboard.DataQuality{}, err
			}
			return response.DataQuality, nil
		},
		"periods": func(s *dashboardService) (dashboard.DataQuality, error) {
			response, err := s.Periods(nil, dashboardOwnerID, dashboard.PeriodsRequest{
				TeamID: 29, Current: dashboard.PeriodRange{From: "2026-09-01", To: "2026-09-30"},
			})
			if err != nil {
				return dashboard.DataQuality{}, err
			}
			return response.DataQuality, nil
		},
		"timeseries": func(s *dashboardService) (dashboard.DataQuality, error) {
			response, err := s.TimeSeries(nil, dashboardOwnerID, dashboardQuery())
			if err != nil {
				return dashboard.DataQuality{}, err
			}
			return response.DataQuality, nil
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			service, _, _ := task2QualityService(t, rows, open)

			quality, err := call(service)

			require.NoError(t, err)
			assert.Equal(t, expected, quality)
		})
	}
}

// validate ya validó a los atletas: Comparisons no puede volver a leer el roster.
func TestDashboardService_ComparisonsUnaSolaLecturaDeRoster(t *testing.T) {
	rows, open := task2QualityRows()
	service, _, _ := task2QualityService(t, rows, open)
	groupUserDao := &mockDashboardGroupUserDao{members: []dbs.GroupUser{
		{GroupID: 39, UserID: 1}, {GroupID: 39, UserID: 2},
	}}
	service.groupUserDao = groupUserDao
	request := dashboard.ComparisonsRequest{
		TeamID: 29, From: "2026-09-01", To: "2026-09-30", AthleteUserIDs: []int64{1, 2},
	}

	response, err := service.Comparisons(nil, dashboardOwnerID, request)

	require.NoError(t, err)
	assert.Equal(t, 1, groupUserDao.findByGroupIDsCalls, "el roster se lee una sola vez")
	assert.Equal(t, 0, groupUserDao.findByGroupIDCalls)

	series := mapSeriesRows(rows)
	expected := dashboard.BuildComparisons(series, dashboard.DefaultTolerance, dashboard.BuildDataQuality(series, open))
	dashboard.ApplyAthleteNames(expected.Series, map[int64]dashboard.AthleteName{
		1: {Name: "Ana", Surname: "Gómez"},
		2: {Name: "Beto", Surname: "Ruiz"},
	})
	assert.Equal(t, expected, *response)
}

// resolveTeamGroup usa el grupo que ya está en el scope y solo consulta el DAO
// cuando el grupo no está, conservando 404 inexistente y 403 fuera de scope.
func TestDashboardService_ResolveTeamGroupReusaElScope(t *testing.T) {
	service, _, scopeService := dashboardFixture(t)

	inScope := int64(39)
	query := dashboardQuery()
	query.GroupID = &inScope
	_, err := service.Summary(nil, dashboardOwnerID, query)
	require.NoError(t, err)
	assert.Equal(t, 0, scopeService.findGroupCalls, "el grupo del scope no consulta el DAO")

	foreign := int64(99)
	query.GroupID = &foreign
	_, err = service.Summary(nil, dashboardOwnerID, query)
	require.ErrorIs(t, err, ErrDashboardForbidden)
	assert.Equal(t, 1, scopeService.findGroupCalls, "el grupo ajeno se resuelve para distinguir 403 de 404")

	scopeService.findGroupCalls = 0
	missing := int64(9999)
	query.GroupID = &missing
	_, err = service.Summary(nil, dashboardOwnerID, query)
	require.ErrorIs(t, err, ErrDashboardNotFound)
	assert.Equal(t, 1, scopeService.findGroupCalls)
}

// La resolución del grupo sigue ocurriendo antes que el resto de las
// validaciones, aunque el grupo ya esté en el scope.
func TestDashboardService_ResolveTeamGroupConservaElOrdenDeValidaciones(t *testing.T) {
	service, _, scopeService := dashboardFixture(t)

	foreign := int64(99)
	query := dashboardQuery()
	query.GroupID = &foreign
	query.Tolerance = "9"
	query.AthleteUserIDs = []int64{999}
	_, err := service.Summary(nil, dashboardOwnerID, query)
	require.ErrorIs(t, err, ErrDashboardForbidden, "el grupo ajeno gana sobre la tolerancia inválida")

	missing := int64(9999)
	query.GroupID = &missing
	query.Tolerance = "9"
	_, err = service.Summary(nil, dashboardOwnerID, query)
	require.ErrorIs(t, err, ErrDashboardNotFound, "el grupo inexistente gana sobre la tolerancia inválida")
	assert.Equal(t, 2, scopeService.findGroupCalls)
}

// SessionComparison mantiene A antes de B: si A falla, B ni se resuelve.
func TestDashboardService_SessionComparisonConservaABantesDeB(t *testing.T) {
	service, dao, scopeService := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 39, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, 1)}, nil
	}

	request := sessionComparisonRequest(1, 1)
	request.A.GroupID = 9999
	request.B.GroupID = 9999
	_, err := service.SessionComparison(nil, dashboardOwnerID, request)
	require.ErrorIs(t, err, ErrDashboardNotFound)
	assert.Equal(t, 1, scopeService.findGroupCalls, "B no se resuelve si A falla")

	scopeService.findGroupCalls = 0
	request = sessionComparisonRequest(1, 1)
	request.B.GroupID = 99
	_, err = service.SessionComparison(nil, dashboardOwnerID, request)
	require.ErrorIs(t, err, ErrDashboardForbidden)
	assert.Equal(t, 1, scopeService.findGroupCalls, "A está en el scope y no consulta el DAO")
}
