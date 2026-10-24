package services

// Conteos de queries del Gap 36 con DAO real (patrón countingConnPool): el
// listado de intentos y el batch de counts del ListByTeam no escalan con la
// cantidad de intentos ni con la de competencias.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/testutils"
)

func TestCompetitionService_ListAttempts_QueryCountConstante(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fx := attemptsFlowSetup(t, db, "qc1")

	compDao := daos.NewCompetitionDao(db)
	compSvc := NewCompetitionService(compDao, nil, nil)
	created, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
		Type:                   competition.TypeRoute,
		RouteGeoJSON:           strPtr(routeFlowGeoJSON()),
		AcceptanceRadiusMeters: fptr(50),
		StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		ParticipantIDs:         []int64{fx.runnerA.ID, fx.runnerB.ID},
	})
	require.NoError(t, err)
	comp, err := compDao.FindByID(nil, created.ID)
	require.NoError(t, err)
	runnerDao := daos.NewRunnerSessionDao(db)

	queries := 0
	countingSvc := NewCompetitionService(daos.NewCompetitionDao(countingDB(t, db, &queries)), nil, nil)

	createFinishedAttempt := func() {
		t.Helper()
		rs, err := compDao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
		require.NoError(t, err)
		require.NoError(t, runnerDao.UpdateStatus(nil, rs.ID, runnersession.RunnerSessionStatusFinished,
			[]string{runnersession.RunnerSessionStatusWip, runnersession.RunnerSessionStatusInterrupted}, time.Now()))
	}

	// N=1.
	createFinishedAttempt()
	_, err = countingSvc.ListAttempts(nil, fx.owner.ID, created.ID, fx.runnerA.ID)
	require.NoError(t, err)
	const expected = 6 // FindByID + IsTeamOwner + IsParticipant + list + sum distance + points
	assert.Equal(t, expected, queries)

	// N=3: mismo número, cada lectura extra es batch.
	createFinishedAttempt()
	createFinishedAttempt()
	queries = 0
	resp, err := countingSvc.ListAttempts(nil, fx.owner.ID, created.ID, fx.runnerA.ID)
	require.NoError(t, err)
	require.Len(t, resp.Data, 3)
	assert.Equal(t, expected, queries, "el número de queries no escala con la cantidad de intentos")
}

func TestCompetitionService_ListByTeam_CountsQueryUnica(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fx := attemptsFlowSetup(t, db, "qc2")

	compSvc := NewCompetitionService(daos.NewCompetitionDao(db), nil, nil)
	startAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	newRoute := func() {
		t.Helper()
		_, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
			Type:                   competition.TypeRoute,
			RouteGeoJSON:           strPtr(routeFlowGeoJSON()),
			AcceptanceRadiusMeters: fptr(50),
			StartAt:                startAt,
			ParticipantIDs:         []int64{fx.runnerA.ID, fx.runnerB.ID},
		})
		require.NoError(t, err)
	}
	newMetric := func() {
		t.Helper()
		metric := "distance"
		end := startAt.AddDate(0, 0, 5)
		_, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
			Type: competition.TypeMetric, Metric: &metric, StartAt: startAt, EndAt: &end,
			ParticipantIDs: []int64{fx.runnerA.ID, fx.runnerB.ID},
		})
		require.NoError(t, err)
	}

	// 1 metric + 2 route.
	newMetric()
	newRoute()
	newRoute()

	queries := 0
	countingSvc := NewCompetitionService(daos.NewCompetitionDao(countingDB(t, db, &queries)), nil, nil)
	resp, err := countingSvc.ListByTeam(nil, fx.runnerA.ID, fx.team.ID, "", nil)
	require.NoError(t, err)
	require.Len(t, resp.Data, 3)
	var sawRoute bool
	for _, it := range resp.Data {
		if it.Type != competition.TypeRoute {
			assert.Nil(t, it.AttemptsUsed)
			assert.False(t, it.CanAttempt)
			continue
		}
		sawRoute = true
		require.NotNil(t, it.AttemptsUsed)
		assert.Equal(t, int64(0), *it.AttemptsUsed)
		assert.True(t, it.CanAttempt, "sin límite: participante sin intentos puede intentar")
	}
	assert.True(t, sawRoute)

	first := queries

	// 1 metric + 4 route: el total de queries no se mueve — el counts de las
	// route es una ÚNICA query batch (fillAttemptFieldsBatch), no una por
	// competencia.
	newRoute()
	newRoute()
	queries = 0
	_, err = countingSvc.ListByTeam(nil, fx.runnerA.ID, fx.team.ID, "", nil)
	require.NoError(t, err)
	const expected = 6 // IsTeamGroupMember + HasUnseenTeam + ListByTeam + participants + winners + counts
	assert.Equal(t, expected, queries)
	assert.Equal(t, first, queries, "el número de queries no escala con la cantidad de competencias")
}
