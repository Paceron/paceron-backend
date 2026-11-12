package services

// Evaluador de logros (Task 5, D6): cada evento combina sus agregados en una
// sola lectura y agrupa los unlocks en un único INSERT.

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func seedAchievementFeedback(t *testing.T, db *gorm.DB, athleteID, sessionID int64, meters *float64) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		AssignedSessionID: sessionID, AssignedExerciseID: 1,
		AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID, ReportSource: "corredor",
		SessionDate: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), SetNumber: 1,
		DistanceMeters: meters, MediaURLs: media,
	}).Error)
}

func TestAchievementsService_EvaluateEvent_QueryCount(t *testing.T) {
	db := testutils.SetupTestDB(t)
	const athleteID int64 = 99001

	early := time.Date(2026, 10, 4, 10, 30, 0, 0, time.UTC)
	late := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
	for i := int64(0); i < 5; i++ {
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: 1000 + i, AthleteUserID: athleteID, Status: "finished", StartDate: early,
		}).Error)
	}
	for i := int64(0); i < 5; i++ {
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: 1100 + i, AthleteUserID: athleteID, Status: "finished", StartDate: late,
		}).Error)
	}
	d42, d10 := 42000.0, 10000.0
	seedAchievementFeedback(t, db, athleteID, 2001, &d42)
	seedAchievementFeedback(t, db, athleteID, 2002, &d10)

	queries := 0
	cdb := countingNestedDB(t, db, &queries)
	svc := NewAchievementsService(
		daos.NewAchievementDao(cdb), daos.NewGamificationDao(cdb),
		&fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{},
	)

	require.NoError(t, svc.EvaluateEventWithSnapshot(nil, athleteID, AchievementTriggerRunnerSession, &StreakSnapshot{}))
	assert.Equal(t, 1, queries, "runner: finished y early en una sola lectura; los INSERT no cuentan")

	queries = 0
	require.NoError(t, svc.EvaluateEvent(nil, athleteID, AchievementTriggerFeedback))
	assert.Equal(t, 1, queries, "feedback: total y máximo por sesión en una sola lectura")

	var progress int64
	require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", athleteID).Count(&progress).Error)
	assert.Equal(t, int64(5), progress, "first_session, sessions_5, early_5 y los 3 de distancia")

	var unseen int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?",
		athleteID, constants.GamificationSectionAchievements).Count(&unseen).Error)
	assert.Equal(t, progress, unseen, "cada unlock nuevo persiste su unseen en la misma sentencia")
}
