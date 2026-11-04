package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func TestRunFullSeed_CycleIsIdempotentAndCleanupStaysInFullScope(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullActivityTestConfig(t)
	decoy := createFullLifecycleDashboardDecoy(t, db)
	baseline := fullLifecycleCounts(t, db)

	originalNow := gametime.Now
	now := time.Date(2026, time.October, 13, 15, 0, 0, 0, gametime.Location)
	gametime.Now = func() time.Time { return now }
	t.Cleanup(func() { gametime.Now = originalNow })

	seedOpts := options{profile: fullProfile, seed: true}
	require.NoError(t, runFullSeed(db, seedOpts))
	require.NoError(t, runFullSeed(db, options{profile: fullProfile, verify: true}))
	seeded := fullLifecycleCounts(t, db)

	now = now.Add(24 * time.Hour)
	require.NoError(t, runFullSeed(db, seedOpts))
	assert.Equal(t, seeded, fullLifecycleCounts(t, db), "una invocación nueva no debe duplicar el perfil ni mover su ancla")
	foreignUser := dbs.User{
		Name: "Foreign", Surname: "Runner", Email: "full-cleanup-foreign@example.test", DNI: "990009902",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "test", Status: "active",
	}
	require.NoError(t, db.Create(&foreignUser).Error)
	var today dbs.GroupCalendarDay
	require.NoError(t, db.Where("group_id = ? AND date = ?", fullGroupAMainID, time.Date(2026, time.October, 13, 0, 0, 0, 0, gametime.Location)).Take(&today).Error)
	require.NotNil(t, today.SessionInstanceID)
	foreignRunner := dbs.RunnerSession{
		SessionInstanceID: *today.SessionInstanceID, AthleteUserID: foreignUser.ID,
		Status: "finished", StartDate: time.Date(2026, time.October, 13, 11, 0, 0, 0, gametime.Location),
	}
	require.NoError(t, db.Create(&foreignRunner).Error)
	withForeignRunner := fullLifecycleCounts(t, db)
	err := runFullSeed(db, options{profile: fullProfile, cleanup: true, yes: true})
	require.ErrorContains(t, err, "runner_session no atribuible")
	assert.Equal(t, withForeignRunner, fullLifecycleCounts(t, db), "un hijo no atribuible debe abortar antes de borrar")
	require.NoError(t, db.Delete(&dbs.RunnerSession{}, foreignRunner.ID).Error)
	require.NoError(t, db.Delete(&dbs.User{}, foreignUser.ID).Error)

	require.ErrorContains(t, runFullSeed(db, options{profile: fullProfile, cleanup: true}), "--yes")
	assert.Equal(t, seeded, fullLifecycleCounts(t, db), "cleanup sin confirmación no debe escribir")
	require.NoError(t, runFullSeed(db, options{profile: fullProfile, cleanup: true, yes: true}))
	require.NoError(t, runFullSeed(db, options{profile: fullProfile, verify: true}))
	assert.Equal(t, baseline, fullLifecycleCounts(t, db), "cleanup debe revertir todas las tablas del perfil y preservar el baseline ajeno")

	assert.Zero(t, fullLifecycleCount(t, db, "users", "email LIKE ?", "%@seed-full.local"))
	assert.Zero(t, fullLifecycleCount(t, db, "teams", "id BETWEEN ? AND ? AND description LIKE ?", fullTeamAID, fullTeamFID, "%"+fullSeedMarker+"%"))
	assert.Zero(t, fullLifecycleCount(t, db, "groups", "id BETWEEN ? AND ? AND description LIKE ?", fullGroupAMainID, fullGroupFID, "%"+fullSeedMarker+"%"))
	assert.Zero(t, fullLifecycleCount(t, db, "competition", "team_id IN ?", []int64{fullTeamAID, fullTeamBID}))
	assert.Equal(t, int64(1), fullLifecycleCount(t, db, "groups", "id = ?", decoy.groupID))
	assert.Equal(t, int64(1), fullLifecycleCount(t, db, "group_calendar_days", "id = ?", decoy.dayID))
	assert.Equal(t, int64(1), fullLifecycleCount(t, db, "runner_session", "id = ?", decoy.runnerID))
	assert.Equal(t, int64(1), fullLifecycleCount(t, db, "teams", "id = ?", seedTeamID))
	assert.Equal(t, int64(1), fullLifecycleCount(t, db, "groups", "id = ?", seedGroupID))
	assert.Positive(t, fullLifecycleCount(t, db, "roles", "name IN ?", []string{"entrenador", "corredor"}))
	assert.Positive(t, fullLifecycleCount(t, db, "platform_settings", "key = ?", "marketplace_fee_percent"))
}

type fullLifecycleDecoy struct {
	groupID, dayID, runnerID int64
}

func createFullLifecycleDashboardDecoy(t *testing.T, db *gorm.DB) fullLifecycleDecoy {
	t.Helper()
	owner := dbs.User{
		Name: "Dashboard", Surname: "Decoy", Email: "dashboard-decoy@example.test", DNI: "990009901",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "test", Status: "active",
	}
	require.NoError(t, db.Create(&owner).Error)
	team := dbs.Team{ID: seedTeamID, Name: "Dashboard decoy", OwnerID: owner.ID, MaxMembers: 10, Status: "active", Description: "[seed_dashboard] decoy"}
	require.NoError(t, db.Create(&team).Error)
	group := dbs.Group{ID: seedGroupID, TeamID: team.ID, Name: "Dashboard decoy", Description: "[seed_dashboard] decoy", IsMain: true}
	require.NoError(t, db.Create(&group).Error)
	instance := dbs.SessionInstance{Name: "Dashboard decoy instance"}
	require.NoError(t, db.Create(&instance).Error)
	date := time.Date(2026, time.October, 13, 0, 0, 0, 0, gametime.Location)
	day := dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &instance.ID}
	require.NoError(t, db.Create(&day).Error)
	start := date.Add(8 * time.Hour)
	runner := dbs.RunnerSession{SessionInstanceID: instance.ID, AthleteUserID: owner.ID, Status: "finished", StartDate: start}
	require.NoError(t, db.Create(&runner).Error)
	return fullLifecycleDecoy{groupID: group.ID, dayID: day.ID, runnerID: runner.ID}
}

func fullLifecycleCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	tables := []string{
		"users", "teams", "groups", "group_users", "team_users", "user_roles", "invitations", "join_requests",
		"user_role_tier_subscriptions", "refresh_tokens", "password_reset_tokens", "training_plans", "plan_days",
		"sessions", "session_exercises", "exercises", "group_calendar_days", "session_instances",
		"session_exercise_instances", "exercise_instances", "runner_session", "workout_feedback",
		"workout_feedback_points", "attendances", "gamification_streak_state", "user_achievement_progress",
		"gamification_unseen", "gamification_stamp", "competition", "competition_participant", "competition_winner",
		"installments", "payments", "session_messages", "session_message_recipients",
		"push_tokens", "seller_connections",
	}
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error, table)
		counts[table] = count
	}
	return counts
}

func fullLifecycleCount(t *testing.T, db *gorm.DB, table, where string, args ...any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where(where, args...).Count(&count).Error)
	return count
}

func TestFullRunnerKey_IdentifiesInstantNotOffset(t *testing.T) {
	cordoba := time.Date(2026, time.October, 13, 8, 0, 0, 0, gametime.Location)
	utc := cordoba.In(time.UTC)
	end := cordoba.Add(time.Hour)
	endUTC := end.In(time.UTC)

	base := fullRunnerKey(10, 20, "finished", cordoba, &end)
	assert.Equal(t, base, fullRunnerKey(10, 20, "finished", utc, &endUTC))
	assert.NotEqual(t, base, fullRunnerKey(11, 20, "finished", cordoba, &end))
	assert.NotEqual(t, base, fullRunnerKey(10, 21, "finished", cordoba, &end))
	assert.NotEqual(t, base, fullRunnerKey(10, 20, "wip", cordoba, &end))
	assert.NotEqual(t, base, fullRunnerKey(10, 20, "finished", cordoba.Add(time.Second), &end))
	assert.NotEqual(t, base, fullRunnerKey(10, 20, "finished", cordoba, nil))
	assert.NotEqual(t, base, fullRunnerKey(10, 20, "finished", cordoba, timePointer(end.Add(time.Microsecond))))
}
