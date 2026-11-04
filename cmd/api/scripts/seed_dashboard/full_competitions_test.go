package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
	"simple-arq-golang/cmd/api/utils/routematch"
)

func TestFullSeederSeedCompetitions_RequiresPreviousPhases(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})

	err := s.seedCompetitions()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "seedIdentity")
	assert.Zero(t, fullCompetitionCount(t, db, &dbs.Competition{}))
}

func TestFullSeederSeedCompetitions_SeedsCompetitionContractsIdempotently(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullActivityTestConfig(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.October, 13, 15, 0, 0, 0, gametime.Location)
	oldNow := gametime.Now
	gametime.Now = func() time.Time { return s.now }
	t.Cleanup(func() { gametime.Now = oldNow })
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	require.NoError(t, s.seedCatalogCalendar())
	require.NoError(t, s.seedActivity())
	require.NoError(t, s.seedGamification())
	teamID := s.ids.teams["team_a"]
	trainerID := s.ids.users["trainer_1"]
	leaderboardBefore := fullCompetitionLeaderboardSnapshot(t, db, s.ids.users["r4"], teamID)
	activeMetricBefore := fullCompetitionMetricSnapshot(t, db, teamID, s.ids.users["r1"], s.ids.users["r4"], s.now)
	achievementDistanceBefore := fullCompetitionAchievementDistanceSnapshot(t, db, s.ids.users)

	require.NoError(t, s.seedCompetitions())
	before := fullCompetitionCounts(t, db)
	competitionIDs := copyInt64Map(s.ids.competitions)
	require.NoError(t, s.seedCompetitions())
	assert.Equal(t, before, fullCompetitionCounts(t, db))
	assert.Equal(t, competitionIDs, s.ids.competitions)
	assert.Len(t, s.ids.competitions, 8)
	assert.Equal(t, leaderboardBefore, fullCompetitionLeaderboardSnapshot(t, db, s.ids.users["r4"], teamID),
		"la evidencia sintética de competencias no debe alterar el leaderboard del equipo")
	assert.Equal(t, achievementDistanceBefore, fullCompetitionAchievementDistanceSnapshot(t, db, s.ids.users),
		"la evidencia sintética de competencias no debe alterar los agregados de distancia de logros")

	service := services.NewCompetitionService(daos.NewCompetitionDao(db), nil, nil)
	r1ID, r2ID := s.ids.users["r1"], s.ids.users["r2"]

	for status, want := range map[string]int{
		competition.StatusPendingApproval: 1,
		competition.StatusActive:          3,
		competition.StatusFinished:        3,
		competition.StatusRejected:        1,
	} {
		listed, err := service.ListByTeam(nil, r1ID, teamID, status, nil)
		require.NoError(t, err)
		assert.Len(t, listed.Data, want, status)
	}
	all, err := service.ListByTeam(nil, s.ids.users["r6"], teamID, "", nil)
	require.NoError(t, err)
	assert.Len(t, all.Data, 8)
	assert.True(t, all.HasUnseen)
	pending, err := service.Get(nil, r1ID, s.ids.competitions["pending_metric"])
	require.NoError(t, err)
	assert.Equal(t, competition.StatusPendingApproval, pending.Data.Status)

	activeMetric, err := service.Get(nil, r1ID, s.ids.competitions["active_metric"])
	require.NoError(t, err)
	require.Len(t, activeMetric.Data.Progress, 2)
	metricProgress := make(map[int64]float64)
	for _, raw := range activeMetric.Data.Progress {
		item, ok := raw.(competition.CompetitionMetricProgressItem)
		require.True(t, ok)
		require.NotNil(t, item.Value)
		metricProgress[item.UserID] = *item.Value
	}
	assert.Positive(t, metricProgress[s.ids.users["r1"]])
	assert.Positive(t, metricProgress[s.ids.users["r4"]])
	assert.Equal(t, activeMetricBefore[s.ids.users["r1"]], metricProgress[s.ids.users["r1"]])
	assert.Equal(t, activeMetricBefore[s.ids.users["r4"]], metricProgress[s.ids.users["r4"]])
	assert.Nil(t, activeMetric.Data.AttemptsUsed)
	assert.False(t, activeMetric.Data.CanAttempt)

	finishedMetric, err := service.Get(nil, r1ID, s.ids.competitions["finished_metric_tie"])
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{s.ids.users["r6"], s.ids.users["r7"]}, finishedMetric.Data.WinnerUserIDs)
	tieProgress := make(map[int64]float64)
	for _, raw := range finishedMetric.Data.Progress {
		item, ok := raw.(competition.CompetitionMetricProgressItem)
		require.True(t, ok)
		require.NotNil(t, item.Value)
		tieProgress[item.UserID] = *item.Value
	}
	assert.Equal(t, 5000.0, tieProgress[s.ids.users["r6"]])
	assert.Equal(t, 5000.0, tieProgress[s.ids.users["r7"]])
	sourceFeedback := dbs.WorkoutFeedback{}
	require.NoError(t, db.First(&sourceFeedback, s.ids.workoutFeedback["completed_r1_minus14_set1"]).Error)
	for _, athlete := range []string{"r6", "r7"} {
		var synthetic dbs.WorkoutFeedback
		require.NoError(t, db.First(&synthetic, s.ids.workoutFeedback[fullCompetitionMetricFeedbackKey+"_"+athlete]).Error)
		assert.NotEqual(t, sourceFeedback.AssignedSessionID, synthetic.AssignedSessionID,
			"el feedback del empate debe vivir fuera de la instancia del calendario")
		assert.Contains(t, pointerValue(synthetic.Annotations), constants.CompetitionSeedFeedbackMarker)
	}
	var metricInstance dbs.SessionInstance
	require.NoError(t, db.First(&metricInstance, s.ids.competitionInstances["finished_metric_tie_metric"]).Error)
	assert.Equal(t, fmt.Sprintf("Competencia métrica %d", s.ids.competitions["finished_metric_tie"]), metricInstance.Name)
	assert.Contains(t, pointerValue(metricInstance.Description), fullSeedMarker)
	feedbackService := services.NewWorkoutFeedbackService(daos.NewWorkoutFeedbackDao(db))
	r6ID := s.ids.users["r6"]
	calendarFeedback, err := feedbackService.GetSessionFeedback(nil, trainerID, sourceFeedback.AssignedSessionID, &r6ID)
	require.NoError(t, err)
	assert.Empty(t, calendarFeedback, "el feedback métrico sintético no debe aparecer en el detalle de calendario")

	activeRoute, err := service.Get(nil, r2ID, s.ids.competitions["active_route"])
	require.NoError(t, err)
	require.NotNil(t, activeRoute.Data.AttemptsUsed)
	assert.Equal(t, int64(2), *activeRoute.Data.AttemptsUsed)
	assert.True(t, activeRoute.Data.CanAttempt)
	require.NotNil(t, activeRoute.Data.MaxAttempts)
	assert.Equal(t, int64(3), *activeRoute.Data.MaxAttempts)
	require.NotNil(t, activeRoute.Data.AcceptanceRadiusMeters)
	assert.Equal(t, 50.0, *activeRoute.Data.AcceptanceRadiusMeters)
	require.NotNil(t, activeRoute.Data.Route)
	vertices, err := routematch.ParseLineString(*activeRoute.Data.Route)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(vertices), 500)
	assert.Less(t, routematch.LineStringLength(vertices), 100000.0)
	routeProgress := fullCompetitionRouteProgress(t, activeRoute.Data.Progress)
	require.NotNil(t, routeProgress[r2ID].ComplianceRatio)
	assert.Greater(t, *routeProgress[r2ID].ComplianceRatio, 0.0)
	assert.Less(t, *routeProgress[r2ID].ComplianceRatio, 100.0)

	attempts, err := service.ListAttempts(nil, trainerID, s.ids.competitions["active_route"], r2ID)
	require.NoError(t, err)
	require.Len(t, attempts.Data, 2)
	assert.Equal(t, runnersession.RunnerSessionStatusFinished, attempts.Data[0].Status)
	assert.NotNil(t, attempts.Data[0].ComplianceRatio)
	assert.Equal(t, runnersession.RunnerSessionStatusWip, attempts.Data[1].Status)
	assert.Nil(t, attempts.Data[1].ComplianceRatio)

	r1Attempts, err := service.ListAttempts(nil, trainerID, s.ids.competitions["active_route"], r1ID)
	require.NoError(t, err)
	require.Len(t, r1Attempts.Data, 1)
	assert.Equal(t, 100.0, *r1Attempts.Data[0].ComplianceRatio)
	assert.Equal(t, r1Attempts.Data[0].SessionInstanceID, attempts.Data[0].SessionInstanceID,
		"el primer intento de varios atletas comparte la instancia sintética")
	assert.NotEqual(t, attempts.Data[0].SessionInstanceID, attempts.Data[1].SessionInstanceID,
		"los intentos posteriores usan una instancia propia")
	var attemptInstance dbs.SessionInstance
	require.NoError(t, db.First(&attemptInstance, attempts.Data[1].SessionInstanceID).Error)
	assert.Contains(t, attemptInstance.Name, "(intento 2)")
	assert.Contains(t, pointerValue(attemptInstance.Description), fullSeedMarker)
	fullCompetitionAssertGPSWithinRoute(t, db, s.ids.workoutFeedback["active_route_r1_1"], vertices, 10)
	fullCompetitionAssertGPSWithinRoute(t, db, s.ids.workoutFeedback["active_route_r2_1"], vertices, 10)
	var routeEvidence dbs.WorkoutFeedback
	require.NoError(t, db.First(&routeEvidence, s.ids.workoutFeedback["active_route_r1_1"]).Error)
	assert.Nil(t, routeEvidence.TeamID)
	assert.Contains(t, pointerValue(routeEvidence.Annotations), constants.CompetitionSeedFeedbackMarker)
	r1Route, err := service.Get(nil, r1ID, s.ids.competitions["active_route"])
	require.NoError(t, err)
	require.NotNil(t, r1Route.Data.AttemptsUsed)
	assert.Equal(t, int64(1), *r1Route.Data.AttemptsUsed)
	assert.True(t, r1Route.Data.CanAttempt)

	exhausted, err := service.Get(nil, s.ids.users["r3"], s.ids.competitions["active_route_exhausted"])
	require.NoError(t, err)
	require.NotNil(t, exhausted.Data.AttemptsUsed)
	assert.Equal(t, int64(3), *exhausted.Data.AttemptsUsed)
	assert.False(t, exhausted.Data.CanAttempt)

	finishedRoute, err := service.Get(nil, trainerID, s.ids.competitions["finished_route_winner"])
	require.NoError(t, err)
	assert.Equal(t, []int64{s.ids.users["r6"]}, finishedRoute.Data.WinnerUserIDs)
	finishedRouteAttempts, err := service.ListAttempts(nil, trainerID, s.ids.competitions["finished_route_winner"], s.ids.users["r6"])
	require.NoError(t, err)
	require.Len(t, finishedRouteAttempts.Data, 1)
	assert.Equal(t, 100.0, *finishedRouteAttempts.Data[0].ComplianceRatio)
	slowerRouteAttempts, err := service.ListAttempts(nil, trainerID, s.ids.competitions["finished_route_winner"], s.ids.users["r7"])
	require.NoError(t, err)
	require.Len(t, slowerRouteAttempts.Data, 1)
	require.NotNil(t, slowerRouteAttempts.Data[0].DurationMs)
	require.NotNil(t, finishedRouteAttempts.Data[0].DurationMs)
	assert.Greater(t, *slowerRouteAttempts.Data[0].DurationMs, *finishedRouteAttempts.Data[0].DurationMs)

	emptyRoute, err := service.Get(nil, trainerID, s.ids.competitions["finished_route_empty"])
	require.NoError(t, err)
	assert.Empty(t, emptyRoute.Data.WinnerUserIDs)
	emptyAttempts, err := service.ListAttempts(nil, trainerID, s.ids.competitions["finished_route_empty"], s.ids.users["r4"])
	require.NoError(t, err)
	assert.Empty(t, emptyAttempts.Data)

	rejected, err := service.Get(nil, r1ID, s.ids.competitions["rejected_metric"])
	require.NoError(t, err)
	assert.Equal(t, competition.StatusRejected, rejected.Data.Status)
	require.NotNil(t, rejected.Data.RejectionReason)
	assert.NotEmpty(t, *rejected.Data.RejectionReason)

	assert.EqualValues(t, 3, fullCompetitionCount(t, db, &dbs.GamificationStamp{}))
	var competitionUnseen int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("section = ? AND team_id = ?", "competitions", teamID).Count(&competitionUnseen).Error)
	assert.EqualValues(t, 6, competitionUnseen)
	var stamps []dbs.GamificationStamp
	require.NoError(t, db.Where("source_id IN ?", []int64{s.ids.competitions["finished_metric_tie"], s.ids.competitions["finished_route_winner"], s.ids.competitions["finished_route_empty"]}).Find(&stamps).Error)
	assert.Len(t, stamps, 3)
	for _, stamp := range stamps {
		assert.Equal(t, "competition", stamp.Type)
		assert.Equal(t, fmt.Sprintf("Competencia %d", stamp.SourceID), stamp.SourceName)
		assert.Equal(t, 1, stamp.Count)
	}
	var unseenRows []dbs.GamificationUnseen
	require.NoError(t, db.Where("section = ? AND team_id = ?", "competitions", teamID).Find(&unseenRows).Error)
	var unseenUsers []int64
	for _, row := range unseenRows {
		unseenUsers = append(unseenUsers, row.UserID)
	}
	assert.ElementsMatch(t, []int64{s.ids.users["r6"], s.ids.users["r7"], s.ids.users["r6"], s.ids.users["r7"], s.ids.users["r4"], s.ids.users["r5"]}, unseenUsers)
	var ownerParticipation int64
	require.NoError(t, db.Model(&dbs.CompetitionParticipant{}).
		Where("user_id = ? AND competition_id IN ?", trainerID, fullCompetitionIDs(s.ids.competitions)).Count(&ownerParticipation).Error)
	assert.Zero(t, ownerParticipation)
	assert.Len(t, s.ids.competitionAttempts, 8)
	assert.Len(t, s.ids.competitionInstances, 13)

	later := newFullSeeder(db, options{})
	later.now = s.now.Add(23 * time.Minute)
	later.ids = s.ids
	assert.Equal(t, fullDate(s.now), fullDate(later.now))
	laterIDs := copyInt64Map(later.ids.competitions)
	laterCounts := fullCompetitionCounts(t, db)
	gametime.Now = func() time.Time { return later.now }
	require.NoError(t, later.seedCompetitions())
	gametime.Now = func() time.Time { return s.now }
	assert.Equal(t, laterCounts, fullCompetitionCounts(t, db))
	assert.Equal(t, laterIDs, later.ids.competitions)
}

func fullCompetitionLeaderboardSnapshot(t *testing.T, db *gorm.DB, callerID, teamID int64) map[string]*gamification.LeaderboardResponse {
	t.Helper()
	service := services.NewLeaderboardService(daos.NewLeaderboardDao(db))
	snapshot := make(map[string]*gamification.LeaderboardResponse)
	for _, metric := range gamification.GetValidLeaderboardMetrics() {
		response, err := service.GetLeaderboard(nil, callerID, teamID, metric, gamification.LeaderboardPeriodAll, nil, nil)
		require.NoError(t, err, metric)
		snapshot[metric] = response
	}
	return snapshot
}

func fullCompetitionMetricSnapshot(t *testing.T, db *gorm.DB, teamID, r1ID, r4ID int64, now time.Time) map[int64]float64 {
	t.Helper()
	rows, err := daos.NewCompetitionDao(db).AggregateFeedbackWindow(nil, teamID, []int64{r1ID, r4ID}, fullDate(now.AddDate(0, 0, -21)), fullDate(now))
	require.NoError(t, err)
	values := make(map[int64]float64, len(rows))
	for _, row := range rows {
		if row.SumDistance != nil {
			values[row.UserID] = *row.SumDistance
		}
	}
	return values
}

type fullCompetitionAchievementSnapshot struct {
	distance, maxDistance float64
	finished, gpsFinished int64
	finishedStarts        []time.Time
}

func fullCompetitionAchievementDistanceSnapshot(t *testing.T, db *gorm.DB, users map[string]int64) map[string]fullCompetitionAchievementSnapshot {
	t.Helper()
	dao := daos.NewAchievementDao(db)
	snapshot := make(map[string]fullCompetitionAchievementSnapshot)
	for _, user := range []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		total, err := dao.SumDistanceMeters(nil, users[user])
		require.NoError(t, err)
		max, err := dao.MaxDistanceSingleSession(nil, users[user])
		require.NoError(t, err)
		finished, err := dao.CountFinishedSessions(nil, users[user])
		require.NoError(t, err)
		starts, err := dao.ListFinishedSessionStarts(nil, users[user])
		require.NoError(t, err)
		gpsFinished, err := dao.CountGPSFinishedSessions(nil, users[user])
		require.NoError(t, err)
		snapshot[user] = fullCompetitionAchievementSnapshot{
			distance: total, maxDistance: max, finished: finished, finishedStarts: starts, gpsFinished: gpsFinished,
		}
	}
	return snapshot
}

func fullCompetitionRouteProgress(t *testing.T, progress []competition.CompetitionProgress) map[int64]competition.CompetitionRouteProgressItem {
	t.Helper()
	items := make(map[int64]competition.CompetitionRouteProgressItem, len(progress))
	for _, raw := range progress {
		item, ok := raw.(competition.CompetitionRouteProgressItem)
		require.True(t, ok)
		items[item.UserID] = item
	}
	return items
}

func fullCompetitionAssertGPSWithinRoute(t *testing.T, db *gorm.DB, feedbackID int64, route []routematch.Point, maxMeters float64) {
	t.Helper()
	feedback := dbs.WorkoutFeedback{}
	require.NoError(t, db.First(&feedback, feedbackID).Error)
	require.NotNil(t, feedback.StartedAt)
	require.NotNil(t, feedback.EndedAt)
	var points []dbs.WorkoutFeedbackPoint
	require.NoError(t, db.Where("feedback_id = ?", feedbackID).Order(`"order"`).Find(&points).Error)
	require.NotEmpty(t, points)
	samples := routematch.SampleRoute(route)
	for _, point := range points {
		assert.False(t, point.RecordedAt.Before(*feedback.StartedAt))
		assert.False(t, point.RecordedAt.After(*feedback.EndedAt))
		nearest := 1e12
		for _, sample := range samples {
			distance := routematch.HaversineMeters(point.Latitude, point.Longitude, sample.Lat, sample.Lng)
			if distance < nearest {
				nearest = distance
			}
		}
		assert.LessOrEqual(t, nearest, maxMeters)
	}
}

func fullCompetitionCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Count(&count).Error)
	return count
}

func fullCompetitionCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	return map[string]int64{
		"competitions":  fullCompetitionCount(t, db, &dbs.Competition{}),
		"participants":  fullCompetitionCount(t, db, &dbs.CompetitionParticipant{}),
		"winners":       fullCompetitionCount(t, db, &dbs.CompetitionWinner{}),
		"instances":     fullCompetitionCount(t, db, &dbs.SessionInstance{}),
		"exerciseLinks": fullCompetitionCount(t, db, &dbs.SessionExerciseInstance{}),
		"exerciseInst":  fullCompetitionCount(t, db, &dbs.ExerciseInstance{}),
		"attempts":      fullCompetitionCount(t, db, &dbs.RunnerSession{}),
		"feedback":      fullCompetitionCount(t, db, &dbs.WorkoutFeedback{}),
		"points":        fullCompetitionCount(t, db, &dbs.WorkoutFeedbackPoint{}),
		"stamps":        fullCompetitionCount(t, db, &dbs.GamificationStamp{}),
		"unseen":        fullCompetitionCount(t, db, &dbs.GamificationUnseen{}),
	}
}

func fullCompetitionIDs(ids map[string]int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		result = append(result, id)
	}
	return result
}
