package main

import (
	"testing"
	"time"

	appconstants "simple-arq-golang/cmd/api/constants"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFullSeederSeedGamification_RequiresPreviousPhases(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})

	err := s.seedGamification()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "seedIdentity")
	assert.Zero(t, fullGamificationCount(t, db, &dbs.GroupCalendarDay{}))
	assert.Zero(t, fullGamificationCount(t, db, &dbs.UserAchievementProgress{}))
}

func TestFullSeederSeedGamification_SeedsProfileContractsIdempotently(t *testing.T) {
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
	before := fullGamificationCounts(t, db)
	idSnapshot := copyInt64Map(s.ids.runnerSessions)
	require.NoError(t, s.seedGamification())
	assert.Equal(t, before, fullGamificationCounts(t, db))
	assert.Equal(t, idSnapshot, s.ids.runnerSessions)

	var plan dbs.TrainingPlan
	require.NoError(t, db.First(&plan, s.ids.trainingPlans[fullStreakPlanKey]).Error)
	assert.Contains(t, pointerValue(plan.Description), fullSeedMarker)
	var mainDays int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).
		Where("group_id = ? AND source_plan_id = ?", s.ids.groups["group_a_main"], plan.ID).Count(&mainDays).Error)
	assert.Zero(t, mainDays)
	var streakDays []dbs.GroupCalendarDay
	require.NoError(t, db.Where("group_id = ? AND source_plan_id = ?", s.ids.groups["group_a_secondary"], plan.ID).
		Order("date").Find(&streakDays).Error)
	require.Len(t, streakDays, 9)
	for i, day := range streakDays {
		assert.Equal(t, fullDate(s.now.AddDate(0, 0, -16+i)).Format("2006-01-02"), day.Date.Format("2006-01-02"))
		assert.Equal(t, "training", day.Kind)
		assert.NotNil(t, day.SessionInstanceID)
	}

	streakService := services.NewStreakService(daos.NewGamificationDao(db))
	r4, err := streakService.GetStreaks(nil, s.ids.users["r4"], 365)
	require.NoError(t, err)
	assert.Equal(t, 10, r4.Strict.Current)
	assert.Equal(t, 10, r4.Strict.Best)
	assert.Equal(t, 10, r4.Flexible.Current)
	assert.Equal(t, 10, r4.Flexible.Best)
	assert.True(t, r4.HasUnseen)
	for i := 0; i < 9; i++ {
		day := findFullStreakDay(t, r4.Days, s.now.AddDate(0, 0, -16+i))
		assert.Equal(t, gamification.DayStatusCompleted, day.StrictStatus)
		assert.Equal(t, gamification.DayStatusCompleted, day.FlexibleStatus)
	}
	minus7 := findFullStreakDay(t, r4.Days, s.now.AddDate(0, 0, -7))
	assert.Equal(t, gamification.DayStatusCompleted, minus7.StrictStatus)
	assert.Equal(t, gamification.DayStatusCompleted, minus7.FlexibleStatus)
	r5, err := streakService.GetStreaks(nil, s.ids.users["r5"], 365)
	require.NoError(t, err)
	assert.Equal(t, 0, r5.Strict.Current)
	assert.Equal(t, 4, r5.Strict.Best)
	assert.Equal(t, 0, r5.Flexible.Current)
	assert.Equal(t, 4, r5.Flexible.Best)
	assert.False(t, r5.HasUnseen)
	for i := 0; i < 4; i++ {
		day := findFullStreakDay(t, r5.Days, s.now.AddDate(0, 0, -16+i))
		assert.Equal(t, gamification.DayStatusCompleted, day.StrictStatus)
		assert.Equal(t, gamification.DayStatusCompleted, day.FlexibleStatus)
	}
	for i := 4; i < 9; i++ {
		day := findFullStreakDay(t, r5.Days, s.now.AddDate(0, 0, -16+i))
		assert.Equal(t, gamification.DayStatusFailed, day.StrictStatus)
		assert.Equal(t, gamification.DayStatusFailed, day.FlexibleStatus)
	}
	r9, err := streakService.GetStreaks(nil, s.ids.users["r9"], 365)
	require.NoError(t, err)
	for _, i := range []int{0, 1, 3, 4, 5, 6, 7} {
		day := findFullStreakDay(t, r9.Days, s.now.AddDate(0, 0, -16+i))
		assert.Equal(t, gamification.DayStatusSkipped, day.StrictStatus)
		assert.Equal(t, gamification.DayStatusSkipped, day.FlexibleStatus)
	}
	for _, i := range []int{2, 8} {
		day := findFullStreakDay(t, r9.Days, s.now.AddDate(0, 0, -16+i))
		assert.Equal(t, gamification.DayStatusFailed, day.StrictStatus)
		assert.Equal(t, gamification.DayStatusFailed, day.FlexibleStatus)
	}

	var states []dbs.GamificationStreakState
	require.NoError(t, db.Where("user_id IN ?", []int64{s.ids.users["r4"], s.ids.users["r5"], s.ids.users["r9"]}).Find(&states).Error)
	stateByUser := make(map[int64]dbs.GamificationStreakState, len(states))
	for _, state := range states {
		stateByUser[state.UserID] = state
	}
	assert.Equal(t, 10, stateByUser[s.ids.users["r4"]].StrictCurrent)
	assert.Equal(t, 10, stateByUser[s.ids.users["r4"]].FlexibleCurrent)
	assert.Zero(t, stateByUser[s.ids.users["r5"]].StrictCurrent)
	assert.Zero(t, stateByUser[s.ids.users["r5"]].FlexibleCurrent)
	assert.Zero(t, stateByUser[s.ids.users["r9"]].StrictCurrent)
	assert.Zero(t, stateByUser[s.ids.users["r9"]].FlexibleCurrent)

	achievements := services.NewAchievementsService(
		daos.NewAchievementDao(db), daos.NewGamificationDao(db), streakService, nil, nil,
	)
	r4Achievements, err := achievements.GetUserAchievements(nil, s.ids.users["r4"])
	require.NoError(t, err)
	assert.Len(t, r4Achievements.Data, len(appconstants.AchievementCatalog))
	assert.True(t, r4Achievements.HasUnseen)
	unlocked := make(map[string]bool)
	for _, item := range r4Achievements.Data {
		unlocked[item.AchievementID] = item.UnlockedAt != nil
	}
	for _, id := range []string{"first_session", "sessions_5", "streak_3", "streak_7", "team_first"} {
		assert.True(t, unlocked[id], id)
	}
	r3Achievements, err := achievements.GetUserAchievements(nil, s.ids.users["r3"])
	require.NoError(t, err)
	assert.False(t, r3Achievements.HasUnseen)
	for _, item := range r3Achievements.Data {
		assert.Nil(t, item.UnlockedAt, item.AchievementID)
	}
	r5Achievements, err := achievements.GetUserAchievements(nil, s.ids.users["r5"])
	require.NoError(t, err)
	assert.False(t, r5Achievements.HasUnseen)
	r4Stamps, err := achievements.GetUserStamps(nil, s.ids.users["r4"])
	require.NoError(t, err)
	assert.Empty(t, r4Stamps.Data)
	assert.Zero(t, fullGamificationCount(t, db, &dbs.GamificationStamp{}))
	var competitionUnseen int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("section = ?", "competitions").Count(&competitionUnseen).Error)
	assert.Zero(t, competitionUnseen)

	leaderboard := services.NewLeaderboardService(daos.NewLeaderboardDao(db))
	for _, metric := range gamification.GetValidLeaderboardMetrics() {
		resp, err := leaderboard.GetLeaderboard(nil, s.ids.users["r4"], s.ids.teams["team_a"], metric, gamification.LeaderboardPeriodAll, nil, nil)
		require.NoError(t, err, metric)
		require.NotEmpty(t, resp.Rows, metric)
		require.NotNil(t, resp.Record, metric)
		assert.Positive(t, resp.Record.UserID, metric)
		assert.NotEmpty(t, resp.Record.AchievedAt, metric)
	}
	beforeRun := fullGamificationCounts(t, db)
	err = runFullSeed(db, options{profile: fullProfile, seed: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompleto")
	assert.Equal(t, beforeRun, fullGamificationCounts(t, db))
}

func fullGamificationCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	return map[string]int64{
		"calendar":     fullGamificationCount(t, db, &dbs.GroupCalendarDay{}),
		"instances":    fullGamificationCount(t, db, &dbs.SessionInstance{}),
		"runner":       fullGamificationCount(t, db, &dbs.RunnerSession{}),
		"achievements": fullGamificationCount(t, db, &dbs.UserAchievementProgress{}),
		"unseen":       fullGamificationCount(t, db, &dbs.GamificationUnseen{}),
		"streakState":  fullGamificationCount(t, db, &dbs.GamificationStreakState{}),
		"stamps":       fullGamificationCount(t, db, &dbs.GamificationStamp{}),
	}
}

func fullGamificationCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Count(&count).Error)
	return count
}

func findFullStreakDay(t *testing.T, days []gamification.StreakDay, date time.Time) gamification.StreakDay {
	t.Helper()
	want := fullDate(date).Format("2006-01-02")
	for _, day := range days {
		if day.Date == want {
			return day
		}
	}
	t.Fatalf("no se encontró el día de racha %s", want)
	return gamification.StreakDay{}
}

func copyInt64Map(source map[string]int64) map[string]int64 {
	copy := make(map[string]int64, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
