package services

// Winners del cierre (gamificacion-core E4.2, D9/D10) con DAO mockeado:
// metric por ventana, route con matching + tie-breaks, stamps, unseen y push.

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
	"simple-arq-golang/cmd/api/utils/routematch"
)

// metrosPorGradoLat replica los metros por grado de latitud del haversine del
// paquete (2*pi*6371000/360): la geometría de las fixtures se arma acá.
const metrosPorGradoLat = 111194.927

func fixGamNow(t *testing.T) {
	t.Helper()
	origNow := gametime.Now
	t.Cleanup(func() { gametime.Now = origNow })
	gametime.Now = func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, gametime.Location) }
}

// closeMetricDao arma la competencia metric active del escenario: creador
// corredor 77, participantes 11/12, ventana 10/1 a 10/10 con end_at pasado
// para el guard del cierre.
func closeMetricDao(metric string, aggRows []dbs.CompetitionMetricAgg) *mockCompetitionDAO {
	end := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	return &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) {
			return &dbs.Competition{
				ID: id, TeamID: 10, Type: competition.TypeMetric, Status: competition.StatusActive,
				Metric:          &metric,
				StartAt:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
				EndAt:           &end,
				CreatedByUserID: 77, CreatedByRole: competition.CreatorRoleRunner,
			}, nil
		},
		isOwnerFn: func(teamID, userID int64) (bool, error) { return userID == 1, nil },
		listParticipantsFn: func(ids []int64) ([]dbs.CompetitionParticipant, error) {
			return []dbs.CompetitionParticipant{
				{CompetitionID: ids[0], UserID: 11}, {CompetitionID: ids[0], UserID: 12},
			}, nil
		},
		aggregateWindowFn: func(participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
			return aggRows, nil
		},
	}
}

func TestCompetitionService_CloseMetric_WinnerUnico(t *testing.T) {
	fixGamNow(t)
	dao := closeMetricDao("distance", []dbs.CompetitionMetricAgg{
		{UserID: 11, SumDistance: fptr(7000)},
		{UserID: 12, SumDistance: fptr(3000)},
	})
	pushDAO := mockPushTokenDao{mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
		return []dbs.PushToken{{Token: "ExponentPushToken[test]"}}, nil
	}}
	client := &mockExpoPushClient{}
	svc := NewCompetitionService(dao, &pushDAO, client)

	resp, instances, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []int64{11}, resp.WinnerUserIDs)
	assert.Empty(t, instances) // sin intentos registrados no hay canales WS
	assert.Equal(t, []int64{11}, dao.lastFinishWinners)
	// Stamp solo al ganador; unseen para los dos participantes.
	assert.Equal(t, 1, dao.lastFinishStampsCount)
	assert.Equal(t, 2, dao.lastFinishUnseenCount)
	// Push best-effort a TODOS los participantes.
	assert.Equal(t, 2, client.sendCallCount)
	assert.Equal(t, "competition_finished", client.lastData["type"])
	assert.Equal(t, "/competencias/5", client.lastData["route"])
}

func TestCompetitionService_CloseMetric_EmpateExacto(t *testing.T) {
	fixGamNow(t)
	dao := closeMetricDao("duration", []dbs.CompetitionMetricAgg{
		{UserID: 11, SumDurationMs: ptrInt64Ms(900000)},
		{UserID: 12, SumDurationMs: ptrInt64Ms(900000)},
	})
	svc := newCompetitionServiceForTest(dao)
	resp, _, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12}, resp.WinnerUserIDs)
	assert.Equal(t, 2, dao.lastFinishStampsCount)
}

func TestCompetitionService_CloseMetric_SinIntentos(t *testing.T) {
	fixGamNow(t)
	// Sin filas de feedback: sin winner, sin stamps, sin push; unseen sí
	// (todos los participantes). peak_speed también deja winner vacía sin usos.
	dao := closeMetricDao("peak_speed", nil)
	pushDAO := mockPushTokenDao{mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
		t.Fatal("no debe haber push sin ganadores")
		return nil, nil
	}}
	svc := NewCompetitionService(dao, &pushDAO, &mockExpoPushClient{})
	resp, _, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Empty(t, resp.WinnerUserIDs)
	assert.Zero(t, dao.lastFinishStampsCount)
	assert.Equal(t, 2, dao.lastFinishUnseenCount)
}

func TestCompetitionService_CloseMetric_PaceMenorMejor(t *testing.T) {
	fixGamNow(t)
	// pace 5 min/km (uid12) le gana a 10 min/km (uid11): menor mejor.
	dao := closeMetricDao("pace", []dbs.CompetitionMetricAgg{
		{UserID: 11, SumDurationMs: ptrInt64Ms(1000000), SumDistance: fptr(1000)},
		{UserID: 12, SumDurationMs: ptrInt64Ms(1000000), SumDistance: fptr(2000)},
	})
	svc := newCompetitionServiceForTest(dao)
	resp, _, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, []int64{12}, resp.WinnerUserIDs)
}

func TestCompetitionService_CloseMetric_PeakYErrorDeAgregacion(t *testing.T) {
	fixGamNow(t)
	// peak_speed: max gana (cadena superior) y el error de agregación impide cerrar.
	dao := closeMetricDao("peak_speed", []dbs.CompetitionMetricAgg{
		{UserID: 11, MaxPeak: fptr(18)},
		{UserID: 12, MaxPeak: fptr(17)},
	})
	svc := newCompetitionServiceForTest(dao)
	resp, _, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, []int64{11}, resp.WinnerUserIDs)

	dao.aggregateWindowFn = func(participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
		return nil, fmt.Errorf("DB down for testing")
	}
	_, _, err = svc.Close(nil, 77, 5)
	require.Error(t, err)
	assert.Equal(t, 1, dao.finishCalls, "sin agregación el cierre no aplica la transición")
}

// routeAttemptFixture arma un runner session finished del intento.
func routeAttemptFixture(instanceID, athleteID int64, duration time.Duration) dbs.RunnerSession {
	base := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	end := base.Add(duration)
	return dbs.RunnerSession{
		SessionInstanceID: instanceID, AthleteUserID: athleteID,
		Status:    runnersession.RunnerSessionStatusFinished,
		StartDate: base, EndDate: &end,
	}
}

// closeRouteDao arma la competencia route con ruta meridional de ~400 m
// (samples cada 40 m: 11 posiciones exactas).
func closeRouteDao(radius float64) *mockCompetitionDAO {
	route := fmt.Sprintf(`{"type":"LineString","coordinates":[[0,0],[0,%v]]}`, 400.0/metrosPorGradoLat)
	return &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) {
			return &dbs.Competition{
				ID: id, TeamID: 10, Type: competition.TypeRoute, Status: competition.StatusActive,
				RouteGeoJSON:           strPtr(route),
				AcceptanceRadiusMeters: &radius,
				StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
				CreatedByUserID:        77, CreatedByRole: competition.CreatorRoleRunner,
			}, nil
		},
		isOwnerFn: func(teamID, userID int64) (bool, error) { return userID == 1, nil },
		listParticipantsFn: func(ids []int64) ([]dbs.CompetitionParticipant, error) {
			return []dbs.CompetitionParticipant{
				{CompetitionID: ids[0], UserID: 11}, {CompetitionID: ids[0], UserID: 12},
			}, nil
		},
	}
}

// attemptSpreadGPS puntos GPS en 11 posiciones exactas de los samples a lo
// largo de la ruta (40 m por sample).
func attemptSpreadGPS(instanceID, athleteID int64) []dbs.CompetitionAttemptPoint {
	points := make([]dbs.CompetitionAttemptPoint, 0, 11)
	for i := 0; i <= 10; i++ {
		points = append(points, dbs.CompetitionAttemptPoint{
			SessionInstanceID: instanceID, AthleteUserID: athleteID,
			Latitude: float64(i) * routematch.SampleStepMeters / metrosPorGradoLat, Longitude: 0,
		})
	}
	return points
}

func TestCompetitionService_CloseRoute_CanalesYMatch(t *testing.T) {
	fixGamNow(t)
	dao := closeRouteDao(50)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{
			routeAttemptFixture(900, 11, 5*time.Minute),
			routeAttemptFixture(912, 12, 7*time.Minute),
			{SessionInstanceID: 930, AthleteUserID: 11}, // wip: registra canal, no matchea
		}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		require.ElementsMatch(t, []int64{900, 912}, instanceIDs)
		return append(attemptSpreadGPS(900, 11), attemptSpreadGPS(912, 12)...), nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, instances, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	// Canales = intentos registrados, cualquier status, distinct.
	assert.ElementsMatch(t, []int64{900, 912, 930}, instances)
	// El estadístico ganador: empate en 100% match, menor duración de uid 11.
	assert.Equal(t, []int64{11}, resp.WinnerUserIDs)
	assert.Equal(t, 1, dao.lastFinishStampsCount)
	assert.Equal(t, 2, dao.lastFinishUnseenCount)
}

func TestCompetitionService_CloseRoute_TieIgualDuracion(t *testing.T) {
	fixGamNow(t)
	dao := closeRouteDao(50)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{
			routeAttemptFixture(900, 11, 5*time.Minute),
			routeAttemptFixture(912, 12, 5*time.Minute), // mis minutos exactos
		}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		return append(attemptSpreadGPS(900, 11), attemptSpreadGPS(912, 12)...), nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, _, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12}, resp.WinnerUserIDs)
	assert.Equal(t, 2, dao.lastFinishStampsCount)
}

func TestCompetitionService_CloseRoute_ParcialSinGPSyWip(t *testing.T) {
	fixGamNow(t)
	dao := closeRouteDao(50)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{
			routeAttemptFixture(900, 11, 5*time.Minute),  // sin GPS → 0%
			routeAttemptFixture(912, 12, 30*time.Minute), // GPS centrado en parte del recorrido
		}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		return []dbs.CompetitionAttemptPoint{
			// GPS solo en la mitad norte de la ruta (primeros samples).
			{SessionInstanceID: 912, AthleteUserID: 12, Latitude: 40.0 / metrosPorGradoLat, Longitude: 0},
			{SessionInstanceID: 912, AthleteUserID: 12, Latitude: 120.0 / metrosPorGradoLat, Longitude: 0},
			{SessionInstanceID: 912, AthleteUserID: 12, Latitude: 280.0 / metrosPorGradoLat, Longitude: 0},
		}, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, _, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, []int64{12}, resp.WinnerUserIDs)
	assert.Equal(t, 1, dao.lastFinishStampsCount)
}

func TestCompetitionService_CloseRoute_SinIntentos(t *testing.T) {
	fixGamNow(t)
	dao := closeRouteDao(50)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return nil, nil
	}
	pushDAO := mockPushTokenDao{mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
		t.Fatal("sin intentos no hay pushes de ganador")
		return nil, nil
	}}
	svc := NewCompetitionService(dao, &pushDAO, &mockExpoPushClient{})
	resp, instances, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Empty(t, resp.WinnerUserIDs)
	assert.Empty(t, instances)
	assert.Zero(t, dao.lastFinishStampsCount)
	assert.Equal(t, 2, dao.lastFinishUnseenCount)
}

func TestCompetitionService_CloseRoute_RutaInvalidaNoCierra(t *testing.T) {
	fixGamNow(t)
	dao := closeRouteDao(50)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{routeAttemptFixture(900, 11, 5*time.Minute)}, nil
	}
	dao.findByIDFn = func(id int64) (*dbs.Competition, error) {
		// Ruta ilegal en el load (alta vieja sin validación del parser).
		comp, _ := closeRouteDao(50).findByIDFn(id)
		comp.RouteGeoJSON = strPtr(`{"type":"Point","coordinates":[[0,0]]}`)
		return comp, nil
	}
	svc := newCompetitionServiceForTest(dao)
	_, _, err := svc.Close(nil, 77, 5)
	require.Error(t, err)
	assert.Zero(t, dao.finishCalls, "ruta mal cargada: el cierre no aplica la transición")
}

// routeDaoConEndAt replica closeRouteDao y agrega end_at explícito: el guard
// de ventana del close es metric-only (D3) y route se cierra igual con la
// ventana viva.
func routeDaoConEndAt(endAt time.Time) *mockCompetitionDAO {
	dao := closeRouteDao(50)
	dao.findByIDFn = func(id int64) (*dbs.Competition, error) {
		comp, _ := closeRouteDao(50).findByIDFn(id)
		comp.EndAt = &endAt
		return comp, nil
	}
	return dao
}

func TestCompetitionService_CloseRoute_FutureEndAtCierra(t *testing.T) {
	fixGamNow(t)
	// end_at en el futuro (10/25 vs now 10/20): metric daría 400, route cierra.
	endFuture := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	dao := routeDaoConEndAt(endFuture)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{
			routeAttemptFixture(900, 11, 5*time.Minute),
			routeAttemptFixture(912, 12, 7*time.Minute),
		}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		return append(attemptSpreadGPS(900, 11), attemptSpreadGPS(912, 12)...), nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, instances, err := svc.Close(nil, 77, 5)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusFinished, resp.Status)
	assert.Equal(t, []int64{11}, resp.WinnerUserIDs)
	assert.ElementsMatch(t, []int64{900, 912}, instances)
	assert.Equal(t, 1, dao.finishCalls)
}

func TestCompetitionService_CloseRoute_Ajeno403SinDAOEscritura(t *testing.T) {
	fixGamNow(t)
	// Caller ni creador ni owner: 403 ANTES de tocar escritura (regresión
	// del guard compartido con metric, pinado para route).
	dao := routeDaoConEndAt(time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC))
	svc := newCompetitionServiceForTest(dao)
	_, _, err := svc.Close(nil, 99, 5)
	require.ErrorIs(t, err, ErrCompetitionForbidden)
	assert.Zero(t, dao.finishCalls)
}
