package services

// ListAttempts (Gap 36 D4): guards 404/403, default/passthrough del atleta,
// atleta no-participante vacío y null-rules de los ítems, con DAO mockeado.

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// listAttemptsRouteDao arma la competencia route (misma geometría de las
// fixtures de winners: ruta de ~400 m, samples cada 40 m, radius 50) con
// owners/miembros/participantes fijos: owner = 1, miembro = 3, participantes
// 11 y 12.
func listAttemptsRouteDao(status string) (*dbs.Competition, *mockCompetitionDAO) {
	route := fmt.Sprintf(`{"type":"LineString","coordinates":[[0,0],[0,%v]]}`, 400.0/metrosPorGradoLat)
	comp := &dbs.Competition{
		ID: 5, TeamID: 10, Type: competition.TypeRoute, Status: status,
		RouteGeoJSON:           strPtr(route),
		AcceptanceRadiusMeters: fptr(50),
		StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	dao := &mockCompetitionDAO{
		findByIDFn:      func(id int64) (*dbs.Competition, error) { return comp, nil },
		isOwnerFn:       func(teamID, userID int64) (bool, error) { return userID == 1, nil },
		isMemberFn:      func(teamID, userID int64) (bool, error) { return userID == 3, nil },
		isParticipantFn: func(competitionID, userID int64) (bool, error) { return userID == 11 || userID == 12, nil },
	}
	return comp, dao
}

func TestCompetitionService_ListAttempts_Guards(t *testing.T) {
	// 404 antes de 403: competencia inexistente devuelve el sentinel del DAO
	// aunque el caller tampoco pertenezca al team.
	dao := &mockCompetitionDAO{}
	dao = &mockCompetitionDAO{findByIDFn: func(id int64) (*dbs.Competition, error) {
		return nil, daos.ErrCompetitionNotFound
	}}
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.ListAttempts(nil, 99, 5, 99)
	require.ErrorIs(t, err, daos.ErrCompetitionNotFound)

	// Caller ajeno al team (ni owner ni miembro): 403 y sin tocar el listado.
	_, dao = listAttemptsRouteDao(competition.StatusActive)
	var listCalls int
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		listCalls++
		return nil, nil
	}
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.ListAttempts(nil, 99, 5, 11)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Zero(t, listCalls)

	// Owner y miembro pasan; sin guard de tipo/status: una finished se lista
	// igual que una active.
	for _, tc := range []struct {
		callerID int64
		status   string
	}{
		{1, competition.StatusFinished},
		{3, competition.StatusActive},
	} {
		_, dao = listAttemptsRouteDao(tc.status)
		dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
			return nil, nil
		}
		svc = newCompetitionServiceForTest(dao)
		_, err = svc.ListAttempts(nil, tc.callerID, 5, 11)
		require.NoError(t, err, tc.status)
	}
}

// El atleta llega al DAO tal cual (el default = caller es del controller).
func TestCompetitionService_ListAttempts_AthletePassthrough(t *testing.T) {
	_, dao := listAttemptsRouteDao(competition.StatusActive)
	var captured listAttemptCapture
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		captured.competitionID, captured.athleteID = competitionID, athleteUserID
		return nil, nil
	}
	svc := newCompetitionServiceForTest(dao)

	_, err := svc.ListAttempts(nil, 3, 5, 11)
	require.NoError(t, err)
	assert.Equal(t, int64(5), captured.competitionID)
	assert.Equal(t, int64(11), captured.athleteID)

	// El owner consultando los intentos de otro participante.
	_, err = svc.ListAttempts(nil, 1, 5, 12)
	require.NoError(t, err)
	assert.Equal(t, int64(12), captured.athleteID)
}

type listAttemptCapture struct {
	competitionID, athleteID int64
}

// Atleta pedido no-participante: data vacía sin error y sin tocar el listado
// ni las agregaciones.
func TestCompetitionService_ListAttempts_NoParticipante(t *testing.T) {
	_, dao := listAttemptsRouteDao(competition.StatusActive)
	var touched []string
	dao.isParticipantFn = func(competitionID, userID int64) (bool, error) {
		return false, nil
	}
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		touched = append(touched, "list")
		return nil, nil
	}
	dao.sumDistanceFn = func(instanceIDs []int64, athleteUserID int64) ([]dbs.CompetitionInstanceDistanceAgg, error) {
		touched = append(touched, "sum")
		return nil, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.ListAttempts(nil, 3, 5, 44)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Data)
	assert.Empty(t, resp.Data)
	assert.Empty(t, touched)
}

// Null-rules del ítem: wip sin feedback → 4 campos null; finished con GPS y
// feedback → los 8 campos con valores; orden del DAO se conserva.
func TestCompetitionService_ListAttempts_NullRules(t *testing.T) {
	_, dao := listAttemptsRouteDao(competition.StatusFinished)
	base := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	wip := dbs.RunnerSession{ID: 1, SessionInstanceID: 900, AthleteUserID: 11,
		Status: runnersession.RunnerSessionStatusWip, StartDate: base, CompetitionID: ptrInt64(5)}
	fin := routeAttemptFixture(912, 11, 5*time.Minute)
	fin.ID = 2
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{wip, fin}, nil
	}
	var pointsInstances, pointsAthletes []int64
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		pointsInstances, pointsAthletes = instanceIDs, athleteIDs
		return attemptSpreadGPS(912, 11), nil
	}
	dao.sumDistanceFn = func(instanceIDs []int64, athleteUserID int64) ([]dbs.CompetitionInstanceDistanceAgg, error) {
		require.ElementsMatch(t, []int64{900, 912}, instanceIDs)
		assert.Equal(t, int64(11), athleteUserID)
		return []dbs.CompetitionInstanceDistanceAgg{{SessionInstanceID: 912, SumDistance: fptr(3000)}}, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.ListAttempts(nil, 3, 5, 11)
	require.NoError(t, err)
	require.Len(t, resp.Data, 2)

	// Wip: ended_at/duration_ms/compliance_ratio/distance null.
	wipItem := resp.Data[0]
	assert.Equal(t, int64(1), wipItem.ID)
	assert.Equal(t, int64(900), wipItem.SessionInstanceID)
	assert.Equal(t, "wip", wipItem.Status)
	assert.Equal(t, base, wipItem.StartedAt)
	assert.Nil(t, wipItem.EndedAt)
	assert.Nil(t, wipItem.DurationMs)
	assert.Nil(t, wipItem.ComplianceRatio)
	assert.Nil(t, wipItem.DistanceMeters)

	// Finished: los 8 campos con valores (matching on-demand 100%).
	finItem := resp.Data[1]
	assert.Equal(t, int64(2), finItem.ID)
	assert.Equal(t, int64(912), finItem.SessionInstanceID)
	assert.Equal(t, "finished", finItem.Status)
	require.NotNil(t, finItem.EndedAt)
	require.NotNil(t, finItem.DurationMs)
	assert.Equal(t, int64(5*time.Minute/time.Millisecond), *finItem.DurationMs)
	require.NotNil(t, finItem.DistanceMeters)
	assert.InDelta(t, 3000.0, *finItem.DistanceMeters, 1e-9)
	require.NotNil(t, finItem.ComplianceRatio)
	assert.InDelta(t, 100.0, *finItem.ComplianceRatio, 1e-6)

	// Los puntos GPS solo se piden para los intentos finished del atleta.
	assert.Equal(t, []int64{912}, pointsInstances)
	assert.Equal(t, []int64{11}, pointsAthletes)
}

// Finished sin puntos GPS: compliance_ratio 0 (no null).
func TestCompetitionService_ListAttempts_FinshedSinGPS(t *testing.T) {
	_, dao := listAttemptsRouteDao(competition.StatusActive)
	fin := routeAttemptFixture(900, 11, 5*time.Minute)
	fin.ID = 1
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{fin}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		return nil, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.ListAttempts(nil, 3, 5, 11)
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	require.NotNil(t, resp.Data[0].ComplianceRatio)
	assert.InDelta(t, 0.0, *resp.Data[0].ComplianceRatio, 1e-9)
	assert.Nil(t, resp.Data[0].DistanceMeters)
}

// Ruta mal cargada con intentos finished: el error sube (igual que el
// cierre/criterio de matching confiable) — no se lista un compliance nulo
// que mienta.
func TestCompetitionService_ListAttempts_RutaInvalidaSube(t *testing.T) {
	comp, dao := listAttemptsRouteDao(competition.StatusActive)
	comp.RouteGeoJSON = strPtr(`{"type":"Point","coordinates":[[0,0]]}`)
	fin := routeAttemptFixture(900, 11, 5*time.Minute)
	fin.ID = 1
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{fin}, nil
	}
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.ListAttempts(nil, 3, 5, 11)
	require.Error(t, err)
	require.ErrorContains(t, err, "error al consultar los intentos")

	// Con solo intentos wip no se pide la ruta: sin error aún con geojson
	// roto (el matching no aplica).
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		wip := dbs.RunnerSession{ID: 1, SessionInstanceID: 900, AthleteUserID: 11,
			Status: runnersession.RunnerSessionStatusWip, StartDate: time.Now()}
		return []dbs.RunnerSession{wip}, nil
	}
	resp, err := svc.ListAttempts(nil, 3, 5, 11)
	require.NoError(t, err)
	assert.Nil(t, resp.Data[0].ComplianceRatio)
}

// Errores crudos del DAO → error genérico (500 en controller) sin sentinels.
func TestCompetitionService_ListAttempts_ErroresDAO(t *testing.T) {
	_, dao := listAttemptsRouteDao(competition.StatusActive)
	dbErr := fmt.Errorf("SQLSTATE 23505 detalle interno")
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		return nil, dbErr
	}
	svc := newCompetitionServiceForTest(dao)
	_, err := svc.ListAttempts(nil, 3, 5, 11)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "SQLSTATE")

	_, dao = listAttemptsRouteDao(competition.StatusActive)
	fin := routeAttemptFixture(900, 11, 5*time.Minute)
	fin.ID = 1
	dao.listAttemptsFn = func(competitionID, athleteUserID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{fin}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		return nil, dbErr
	}
	svc = newCompetitionServiceForTest(dao)
	_, err = svc.ListAttempts(nil, 3, 5, 11)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "SQLSTATE")
}
