package services

// --- Task 7: post-finish unificado (D7) ---
// El snapshot de rachas se calcula una sola vez y lo comparten el trigger y
// los logros; el histórico no se relee.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// twelveFinishedDays marca los 12 días del escenario como finished: rachas
// 12/12 y logros de sesiones/early desbloqueados.
func twelveFinishedDays() map[int]bool {
	out := make(map[int]bool, 12)
	for i := 0; i < 12; i++ {
		out[i] = true
	}
	return out
}

func TestFinishHooks_UnifiedSnapshot_NoReleeHistorial(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)

	legacyUser, _ := seedStreakBatchScene(t, db, "legacy-finish", 12, twelveFinishedDays())
	unifiedUser, _ := seedStreakBatchScene(t, db, "unified-finish", 12, twelveFinishedDays())
	seedStreakStateRow(t, db, legacyUser.ID, 5, 2)
	seedStreakStateRow(t, db, unifiedUser.ID, 5, 2)

	counter := 0
	counting := txCountingDB(t, db, &counter)
	gamificationDao := daos.NewGamificationDao(counting)
	streakService := NewStreakService(gamificationDao)
	trigger := NewStreakTriggerService(gamificationDao, streakService)
	achievements := NewAchievementsService(daos.NewAchievementDao(counting), gamificationDao, streakService,
		&fakePushTokenDAO{}, &stubPushClient{})

	// Path previo: trigger y logros recalculan cada uno su propio histórico.
	counter = 0
	require.NoError(t, trigger.EvaluateRunnerCompletion(nil, legacyUser.ID))
	require.NoError(t, achievements.EvaluateEvent(nil, legacyUser.ID, AchievementTriggerRunnerSession))
	legacyQueries := counter

	// Path unificado: un único snapshot alimenta ambos hooks.
	counter = 0
	snapshot, err := streakService.ComputeSnapshot(nil, unifiedUser.ID)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, 12, snapshot.StrictCurrent)
	require.NoError(t, trigger.EvaluateRunnerCompletionWithSnapshot(nil, unifiedUser.ID, snapshot))
	require.NoError(t, achievements.EvaluateEventWithSnapshot(nil, unifiedUser.ID, AchievementTriggerRunnerSession, snapshot))
	unifiedQueries := counter

	// La diferencia exacta es el GetStreaks que hacía logros (4 queries:
	// EXISTS unseen + memberships + días + sesiones finished).
	assert.Equal(t, 4, legacyQueries-unifiedQueries,
		"el finish unificado no duplica loadStreakHistory")
}

func TestFinishHooks_UnifiedSnapshot_Idempotente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)

	runner, _ := seedStreakBatchScene(t, db, "idem-finish", 12, twelveFinishedDays())
	seedStreakStateRow(t, db, runner.ID, 5, 2)

	gamificationDao := daos.NewGamificationDao(db)
	streakService := NewStreakService(gamificationDao)
	trigger := NewStreakTriggerService(gamificationDao, streakService)
	achievements := NewAchievementsService(daos.NewAchievementDao(db), gamificationDao, streakService,
		&fakePushTokenDAO{}, &stubPushClient{})

	run := func() {
		snapshot, err := streakService.ComputeSnapshot(nil, runner.ID)
		require.NoError(t, err)
		require.NoError(t, trigger.EvaluateRunnerCompletionWithSnapshot(nil, runner.ID, snapshot))
		require.NoError(t, achievements.EvaluateEventWithSnapshot(nil, runner.ID, AchievementTriggerRunnerSession, snapshot))
	}

	run()
	var unseenAfterFirst, unlocksAfterFirst int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ?", runner.ID).Count(&unseenAfterFirst).Error)
	require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", runner.ID).Count(&unlocksAfterFirst).Error)

	run()
	var unseenAfterSecond, unlocksAfterSecond int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ?", runner.ID).Count(&unseenAfterSecond).Error)
	require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", runner.ID).Count(&unlocksAfterSecond).Error)

	assert.Equal(t, unseenAfterFirst, unseenAfterSecond, "re-evaluar no duplica unseen")
	assert.Equal(t, unlocksAfterFirst, unlocksAfterSecond, "re-evaluar no duplica logros")
	assert.NotZero(t, unlocksAfterFirst)

	var state dbs.GamificationStreakState
	require.NoError(t, db.Where("user_id = ?", runner.ID).First(&state).Error)
	assert.Equal(t, 12, state.StrictCurrent)
	assert.Equal(t, 12, state.FlexibleCurrent)
}
