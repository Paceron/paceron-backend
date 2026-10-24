package services

// Progress del detalle route con el shape nuevo (Gap 36 D5) y los 3 campos
// caller-dependientes de CompetitionResponse (D7), con DAO mockeado.

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// progressRouteDao arma la competencia route active (misma geometría de
// closeRouteDao) con participantes 11/12 y estados del caller inyectados.
func progressRouteDao(status string, maxAttempts *int64) (*dbs.Competition, *mockCompetitionDAO) {
	comp := &dbs.Competition{
		ID: 5, TeamID: 10, Type: competition.TypeRoute, Status: status,
		MaxAttempts: maxAttempts,
		StartAt:     time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		RouteGeoJSON: strPtr(fmt.Sprintf(
			`{"type":"LineString","coordinates":[[0,0],[0,%v]]}`, 400.0/metrosPorGradoLat)),
		AcceptanceRadiusMeters: func() *float64 { r := 50.0; return &r }(),
	}
	dao := &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) { return comp, nil },
		isOwnerFn:  func(teamID, userID int64) (bool, error) { return userID == 1, nil },
		isMemberFn: func(teamID, userID int64) (bool, error) { return true, nil },
		listParticipantsFn: func(ids []int64) ([]dbs.CompetitionParticipant, error) {
			return []dbs.CompetitionParticipant{
				{CompetitionID: ids[0], UserID: 11}, {CompetitionID: ids[0], UserID: 12},
			}, nil
		},
	}
	return comp, dao
}

func TestCompetitionService_GetRouteProgress_MejorIntento(t *testing.T) {
	fixGamNow(t)
	_, dao := progressRouteDao(competition.StatusActive, nil)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{
			routeAttemptFixture(901, 11, 5*time.Minute), // mejor pct pero más lento
			routeAttemptFixture(900, 11, 3*time.Minute), // 100%, gana por duración
			routeAttemptFixture(912, 12, 7*time.Minute), // 100% de 12
			{SessionInstanceID: 930, AthleteUserID: 12}, // wip: no computa
		}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		points := attemptSpreadGPS(900, 11)
		points = append(points, attemptSpreadGPS(901, 11)...)
		points = append(points, attemptSpreadGPS(912, 12)...)
		return points, nil
	}
	// Caller 11: participante con 2 intentos, sin límite.
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return map[int64]int64{5: 2}, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.Get(nil, 11, 5)
	require.NoError(t, err)
	require.Len(t, resp.Data.Progress, 2)
	item11, ok := resp.Data.Progress[0].(competition.CompetitionRouteProgressItem)
	require.True(t, ok)
	item12, ok := resp.Data.Progress[1].(competition.CompetitionRouteProgressItem)
	require.True(t, ok)
	assert.Equal(t, int64(11), item11.UserID)
	require.NotNil(t, item11.ComplianceRatio)
	assert.InDelta(t, 100.0, *item11.ComplianceRatio, 1e-9)
	require.NotNil(t, item11.DurationMs)
	assert.Equal(t, int64(180000), *item11.DurationMs, "mejor duración de los intentos de 11")
	require.NotNil(t, item11.SessionInstanceID)
	assert.Equal(t, int64(900), *item11.SessionInstanceID, "instancia del intento ganador")
	assert.Equal(t, int64(12), item12.UserID)
	require.NotNil(t, item12.SessionInstanceID)
	assert.Equal(t, int64(912), *item12.SessionInstanceID)
	assert.Equal(t, int64(420000), *item12.DurationMs)

	// Shape rígido del JSON route: sin value (D5) y en el orden del struct.
	progJSON, err := json.Marshal(resp.Data.Progress)
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"user_id":11,"compliance_ratio":100,"duration_ms":180000,"session_instance_id":900},
		{"user_id":12,"compliance_ratio":100,"duration_ms":420000,"session_instance_id":912}]`,
		string(progJSON))

	// D7 con caller participante: intentos usados y puede intentar más.
	require.NotNil(t, resp.Data.AttemptsUsed)
	assert.Equal(t, int64(2), *resp.Data.AttemptsUsed)
	assert.True(t, resp.Data.CanAttempt)
}

func TestCompetitionService_GetRouteProgress_ParticipanteSinIntentos(t *testing.T) {
	fixGamNow(t)
	_, dao := progressRouteDao(competition.StatusActive, nil)
	dao.attemptSessionsFn = func(competitionID int64) ([]dbs.RunnerSession, error) {
		return []dbs.RunnerSession{routeAttemptFixture(900, 12, 5*time.Minute)}, nil
	}
	dao.attemptPointsFn = func(instanceIDs, athleteIDs []int64) ([]dbs.CompetitionAttemptPoint, error) {
		return attemptSpreadGPS(900, 12), nil
	}
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return map[int64]int64{5: 0}, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.Get(nil, 11, 5)
	require.NoError(t, err)
	require.Len(t, resp.Data.Progress, 2)
	item11, ok := resp.Data.Progress[0].(competition.CompetitionRouteProgressItem)
	require.True(t, ok)
	assert.Equal(t, int64(11), item11.UserID)
	assert.Nil(t, item11.ComplianceRatio)
	assert.Nil(t, item11.DurationMs)
	assert.Nil(t, item11.SessionInstanceID, "participante sin intentos: tres nulls")
	item12, ok := resp.Data.Progress[1].(competition.CompetitionRouteProgressItem)
	require.True(t, ok)
	require.NotNil(t, item12.SessionInstanceID)
	// used 0 con límite inexistente: can_attempt true.
	require.NotNil(t, resp.Data.AttemptsUsed)
	assert.Equal(t, int64(0), *resp.Data.AttemptsUsed)
	assert.True(t, resp.Data.CanAttempt)
}

func TestCompetitionService_GetAttemptFields_Reglas(t *testing.T) {
	fixGamNow(t)

	// Límite cumplido: can_attempt false (con used seteado).
	maxTwo := int64(2)
	_, dao := progressRouteDao(competition.StatusActive, &maxTwo)
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return map[int64]int64{5: 2}, nil
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.Get(nil, 11, 5)
	require.NoError(t, err)
	require.NotNil(t, resp.Data.AttemptsUsed)
	assert.Equal(t, int64(2), *resp.Data.AttemptsUsed)
	assert.False(t, resp.Data.CanAttempt)

	// Bajo el límite: true.
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return map[int64]int64{5: 1}, nil
	}
	resp, err = svc.Get(nil, 11, 5)
	require.NoError(t, err)
	assert.True(t, resp.Data.CanAttempt)

	// Competencia cerrada: used lleno pero can_attempt false.
	_, dao = progressRouteDao(competition.StatusFinished, nil)
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return map[int64]int64{5: 1}, nil
	}
	resp, err = newCompetitionServiceForTest(dao).Get(nil, 11, 5)
	require.NoError(t, err)
	assert.False(t, resp.Data.CanAttempt)
	require.NotNil(t, resp.Data.AttemptsUsed)

	// Caller no participante: null/false, sin tocar can_attempt.
	_, dao = progressRouteDao(competition.StatusActive, nil)
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return map[int64]int64{}, nil
	}
	resp, err = newCompetitionServiceForTest(dao).Get(nil, 77, 5)
	require.NoError(t, err)
	assert.Nil(t, resp.Data.AttemptsUsed)
	assert.False(t, resp.Data.CanAttempt)

	// Error del batch count: sube como 500 del detalle.
	_, dao = progressRouteDao(competition.StatusActive, nil)
	dao.countAttemptsFn = func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
		return nil, fmt.Errorf("DB down for testing")
	}
	_, err = newCompetitionServiceForTest(dao).Get(nil, 11, 5)
	require.Error(t, err)
}

func TestCompetitionService_GetMetricProgress_ByteIdentico(t *testing.T) {
	fixGamNow(t)
	comp := &dbs.Competition{
		ID: 5, TeamID: 10, Type: competition.TypeMetric, Metric: strPtr("distance"),
		Status:  competition.StatusActive,
		StartAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	dao := &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) { return comp, nil },
		isMemberFn: func(teamID, userID int64) (bool, error) { return true, nil },
		listParticipantsFn: func(ids []int64) ([]dbs.CompetitionParticipant, error) {
			return []dbs.CompetitionParticipant{
				{CompetitionID: ids[0], UserID: 11}, {CompetitionID: ids[0], UserID: 12},
			}, nil
		},
		aggregateWindowFn: func(participantIDs []int64, startAt, endAt time.Time) ([]dbs.CompetitionMetricAgg, error) {
			return []dbs.CompetitionMetricAgg{
				{UserID: 12, SumDistance: fptr(7000)},
			}, nil
		},
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.Get(nil, 3, 5)
	require.NoError(t, err)
	require.Len(t, resp.Data.Progress, 2)
	progJSON, err := json.Marshal(resp.Data.Progress)
	require.NoError(t, err)
	assert.Equal(t, `[{"user_id":11,"value":null},{"user_id":12,"value":7000}]`, string(progJSON),
		"metric no muta el shape histórico")

	// El detalle metric NO consulta intentos: campos null/false por default.
	assert.Zero(t, dao.countAttemptsCalls)
	assert.Nil(t, resp.Data.AttemptsUsed)
	assert.False(t, resp.Data.CanAttempt)
}

func TestCompetitionService_CreateAttemptFields(t *testing.T) {
	fixGamNow(t)

	// Corredor creador (participante): alta runner-creada arranca pending →
	// used 0 (participante sin intentos) y can_attempt false.
	capturedCompID := int64(0)
	dao := &mockCompetitionDAO{
		teamOwnerFn:    func(teamID int64) (int64, error) { return 1, nil },
		isMemberFn:     func(teamID, userID int64) (bool, error) { return userID == 11, nil },
		countMembersFn: func(teamID int64, userIDs []int64) (int64, error) { return int64(len(userIDs)), nil },
		createFn: func(comp *dbs.Competition, participantIDs []int64) error {
			capturedCompID = comp.ID
			return nil
		},
		countAttemptsFn: func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
			return map[int64]int64{capturedCompID: 0}, nil
		},
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.Create(nil, 11, 10, routeCreateReq())
	require.NoError(t, err)
	require.NotNil(t, resp.AttemptsUsed)
	assert.Equal(t, int64(0), *resp.AttemptsUsed)
	assert.False(t, resp.CanAttempt, "recién alta en pending_approval")

	// Trainer creador (no compite): null/false sin intento.
	daoTrainer := &mockCompetitionDAO{
		teamOwnerFn:    func(teamID int64) (int64, error) { return 1, nil },
		countMembersFn: func(teamID int64, userIDs []int64) (int64, error) { return int64(len(userIDs)), nil },
		createFn: func(comp *dbs.Competition, participantIDs []int64) error {
			return nil
		},
		countAttemptsFn: func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
			return map[int64]int64{}, nil
		},
	}
	resp, err = newCompetitionServiceForTest(daoTrainer).Create(nil, 1, 10, routeCreateReq())
	require.NoError(t, err)
	assert.Nil(t, resp.AttemptsUsed)
	assert.False(t, resp.CanAttempt)

	// Error del fill en el create: sube como error del alta.
	daoFail := &mockCompetitionDAO{
		teamOwnerFn:    func(teamID int64) (int64, error) { return 1, nil },
		isMemberFn:     func(teamID, userID int64) (bool, error) { return userID == 11, nil },
		countMembersFn: func(teamID int64, userIDs []int64) (int64, error) { return int64(len(userIDs)), nil },
		createFn: func(comp *dbs.Competition, participantIDs []int64) error {
			return nil
		},
		countAttemptsFn: func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
			return nil, fmt.Errorf("DB down for testing")
		},
	}
	_, err = newCompetitionServiceForTest(daoFail).Create(nil, 11, 10, routeCreateReq())
	require.Error(t, err)
}

func TestCompetitionService_ListadoAttemptFields_Batch(t *testing.T) {
	fixGamNow(t)
	metric := "distance"
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	comps := []dbs.Competition{
		{ID: 5, TeamID: 10, Type: competition.TypeRoute, Status: competition.StatusActive, StartAt: start},
		{ID: 6, TeamID: 10, Type: competition.TypeRoute, Status: competition.StatusFinished, StartAt: start},
		{ID: 7, TeamID: 10, Type: competition.TypeMetric, Metric: &metric, Status: competition.StatusActive, StartAt: start},
	}
	var capturedRouteIDs []int64
	dao := &mockCompetitionDAO{
		isMemberFn:  func(teamID, userID int64) (bool, error) { return true, nil },
		hasUnseenFn: func(userID, teamID int64) (bool, error) { return false, nil },
		listByTeamFn: func(teamID int64, status *string, groupID *int64) ([]dbs.Competition, error) {
			return comps, nil
		},
		countAttemptsFn: func(competitionIDs []int64, athleteUserID int64) (map[int64]int64, error) {
			capturedRouteIDs = append([]int64(nil), competitionIDs...)
			return map[int64]int64{5: 1}, nil
		},
	}
	svc := newCompetitionServiceForTest(dao)
	resp, err := svc.ListByTeam(nil, 11, 10, "", nil)
	require.NoError(t, err)
	require.Len(t, resp.Data, 3)

	// Solo las competencias route entran al batch (metric fuera).
	assert.Equal(t, []int64{5, 6}, capturedRouteIDs)

	// Participante y active: used + can_attempt.
	require.NotNil(t, resp.Data[0].AttemptsUsed)
	assert.Equal(t, int64(1), *resp.Data[0].AttemptsUsed)
	assert.True(t, resp.Data[0].CanAttempt)

	// Caller no participante de la comp 6 (el counts del mock solo la tiene
	// a la 5): null/false.
	assert.Nil(t, resp.Data[1].AttemptsUsed)
	assert.False(t, resp.Data[1].CanAttempt)

	// Metric: null/false siempre.
	assert.Nil(t, resp.Data[2].AttemptsUsed)
	assert.False(t, resp.Data[2].CanAttempt)
}
