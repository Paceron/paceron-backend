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

func TestDashboardService_SessionComparisonOwnerPuedeCompararMismoAtleta(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{
		{GroupID: 39, UserID: 1},
	}}
	configureSessionComparisonDAO(dao)
	request := sessionComparisonRequest(1, 1)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		require.Len(t, filter.GroupIDs, 1)
		assert.Equal(t, int64(39), filter.GroupIDs[0])
		require.NotNil(t, filter.SessionInstanceID)
		assert.Equal(t, []int64{1}, filter.AthleteUserIDs)
		assert.Equal(t, []int64{8}, filter.ExerciseIDs)
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, 1)}, nil
	}

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	require.NoError(t, err)
	assert.Equal(t, int64(501), response.A.SessionInstanceID)
	assert.Equal(t, int64(502), response.B.SessionInstanceID)
	assert.Equal(t, 12.0, *response.Delta.SpeedDeltaKph)
	assert.Equal(t, "Ana", response.A.AthleteName)
	assert.Equal(t, "Grupo A", response.A.GroupName)
}

func TestDashboardService_SessionComparisonOwnerPuedeCompararDosAtletas(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{
		{GroupID: 39, UserID: 1}, {GroupID: 39, UserID: 2},
	}}
	configureSessionComparisonDAO(dao)
	request := sessionComparisonRequest(1, 2)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, filter.AthleteUserIDs[0])}, nil
	}

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	require.NoError(t, err)
	assert.Equal(t, int64(1), response.A.AthleteUserID)
	assert.Equal(t, int64(2), response.B.AthleteUserID)
}

func TestDashboardService_SessionComparisonMemberSoloPuedeConsultarSusSeries(t *testing.T) {
	service, dao, _ := dashboardFixtureAs(t, 1, false)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 39, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, 1)}, nil
	}
	request := sessionComparisonRequest(1, 1)

	response, err := service.SessionComparison(nil, 1, request)

	require.NoError(t, err)
	assert.Equal(t, int64(1), response.A.AthleteUserID)
	request.B.AthleteUserID = 2
	response, err = service.SessionComparison(nil, 1, request)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
}

func TestDashboardService_SessionComparisonRechazaGrupoDeOtroTeam(t *testing.T) {
	service, dao, scopeService := dashboardFixture(t)
	scopeService.scope.Teams[30] = dbs.Team{ID: 30, Name: "Otro", OwnerID: dashboardOwnerID}
	scopeService.scope.Groups[40] = dbs.Group{ID: 40, TeamID: 30, Name: "Grupo B"}
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 40, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	request := sessionComparisonRequest(1, 1)
	request.B.GroupID = 40

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
}

func TestDashboardService_SessionComparisonBFueraDeScopeNoDevuelveA(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 39, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, 1)}, nil
	}
	request := sessionComparisonRequest(1, 1)
	request.B.GroupID = 99

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrDashboardForbidden)
}

func TestDashboardService_SessionComparisonValidaRangoAntesDeConsultarSeries(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	request := sessionComparisonRequest(1, 1)
	request.From = "2023-01-01"
	request.To = "2026-01-01"

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	assert.Nil(t, response)
	assert.Equal(t, constants.ErrorCodeInvalidRange, paramErrorCode(t, err))
	assert.Equal(t, 0, dao.calls, "un rango rechazado no consulta catalogo ni series")
}

func TestDashboardService_SessionComparisonReferenciaSinFeedbackEsNotFound(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 39, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	dao.findSeriesFn = func(_ *gin.Context, _ daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		return nil, nil
	}

	response, err := service.SessionComparison(nil, dashboardOwnerID, sessionComparisonRequest(1, 1))

	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

func TestDashboardService_SessionComparisonRechazaEjercicioQueNoCoincideConLaSesion(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 39, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		if *filter.SessionInstanceID == 502 && filter.ExerciseIDs[0] == 9 {
			return nil, nil
		}
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, 1)}, nil
	}
	request := sessionComparisonRequest(1, 1)
	request.B.ExerciseID = 9

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

func TestDashboardService_SessionComparisonRechazaSessionIDAjeno(t *testing.T) {
	service, dao, _ := dashboardFixture(t)
	service.groupUserDao = &mockDashboardGroupUserDao{members: []dbs.GroupUser{{GroupID: 39, UserID: 1}}}
	configureSessionComparisonDAO(dao)
	dao.findSeriesFn = func(_ *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
		if *filter.SessionInstanceID == 999 {
			return nil, nil
		}
		return []daos.DashboardSeriesRow{sessionComparisonDAOrow(*filter.SessionInstanceID, 1)}, nil
	}
	request := sessionComparisonRequest(1, 1)
	request.B.SessionInstanceID = 999

	response, err := service.SessionComparison(nil, dashboardOwnerID, request)

	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrDashboardNotFound)
}

func configureSessionComparisonDAO(dao *mockDashboardDao) {
	dao.catalogByIDFn = func(_ *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
		return &daos.DashboardCatalogExercises{
			ID: id, OwnerID: dashboardOwnerID, Name: "Ritmo", Kind: string(constants.ExerciseKindRitmoContinuo),
		}, nil
	}
	dao.unlinkedFn = func(_ *gin.Context, _ []int64) ([]daos.UnlinkedExerciseInstance, error) {
		return nil, nil
	}
}

func sessionComparisonRequest(athleteA, athleteB int64) dashboard.SessionComparisonRequest {
	return dashboard.SessionComparisonRequest{
		TeamID: 29,
		From:   "2026-09-01",
		To:     "2026-09-30",
		A: &dashboard.SessionExerciseRef{
			GroupID: 39, AthleteUserID: athleteA, SessionInstanceID: 501, ExerciseID: 8,
		},
		B: &dashboard.SessionExerciseRef{
			GroupID: 39, AthleteUserID: athleteB, SessionInstanceID: 502, ExerciseID: 8,
		},
	}
}

func sessionComparisonDAOrow(sessionID, athleteID int64) daos.DashboardSeriesRow {
	distance := 1000.0
	duration := int64(60_000)
	if sessionID == 502 {
		duration = 50_000
	}
	rpe := int16(6)
	return daos.DashboardSeriesRow{
		FeedbackID: sessionID, SessionInstanceID: sessionID, SessionDate: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
		AthleteUserID: athleteID, SetNumber: 1, Kind: string(constants.ExerciseKindRitmoContinuo),
		DurationMs: &duration, ActiveDurationMs: &duration, DistanceMeters: &distance, RPE: &rpe,
		HasRunnerSession: true,
	}
}
