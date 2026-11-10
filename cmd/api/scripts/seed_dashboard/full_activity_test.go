package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func TestFullSeederSeedActivity_RequiresPreviousPhases(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullActivityTestConfig(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.July, 18, 15, 0, 0, 0, gametime.Location)

	err := s.seedActivity()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "seedIdentity")
	require.NoError(t, s.seedIdentity())
	err = s.seedActivity()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seedTeams")
	require.NoError(t, s.seedTeams())
	err = s.seedActivity()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seedCatalogCalendar")
	require.NoError(t, s.seedCatalogCalendar())
	todayID := s.ids.calendarDays["today_a_main"]
	delete(s.ids.calendarDays, "today_a_main")
	before := fullActivityCounts(t, db)
	err = s.seedActivity()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "today_a_main")
	assert.Equal(t, before, fullActivityCounts(t, db))
	s.ids.calendarDays["today_a_main"] = todayID
	assert.Empty(t, s.ids.runnerSessions)
	assert.Empty(t, s.ids.workoutFeedback)
	assert.Empty(t, s.ids.attendances)
	assert.Zero(t, fullActivityCount(t, db, &dbs.RunnerSession{}))
	assert.Zero(t, fullActivityCount(t, db, &dbs.WorkoutFeedback{}))
	assert.Zero(t, fullActivityCount(t, db, &dbs.Attendance{}))
}

func TestFullSeederSeedActivity_SeedsHistoryAndAPIContractsIdempotently(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullActivityTestConfig(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.July, 18, 15, 0, 0, 0, gametime.Location)
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	require.NoError(t, s.seedCatalogCalendar())

	require.NoError(t, s.seedActivity())
	before := fullActivityCounts(t, db)
	assert.Len(t, s.ids.runnerSessions, 4)
	assert.Len(t, s.ids.workoutFeedback, 5)
	assert.Len(t, s.ids.attendances, 6)
	assert.Positive(t, s.ids.sessionInstances["activity_a_secondary_minus7"])

	require.NoError(t, s.seedActivity())
	assert.Equal(t, before, fullActivityCounts(t, db))

	var runnerRows []dbs.RunnerSession
	require.NoError(t, db.Where("id IN ?", fullActivityIDs(s.ids.runnerSessions)).Find(&runnerRows).Error)
	minus14 := fullDate(s.now.AddDate(0, 0, -14))
	minus7 := fullDate(s.now.AddDate(0, 0, -7))
	assertActivityRunner(t, runnerRows, s.ids.runnerSessions["finished_r1_minus14"], "finished_r1_minus14", runnersession.RunnerSessionStatusFinished, s.ids.users["r1"], s.ids.sessionInstances[fullActivityStampedDayKey(minus14)], fullActivityClock(minus14, 8, 0), timePointer(fullActivityClock(minus14, 9, 0)))
	assertActivityRunner(t, runnerRows, s.ids.runnerSessions["finished_r4_minus7"], "finished_r4_minus7", runnersession.RunnerSessionStatusFinished, s.ids.users["r4"], s.ids.sessionInstances[fullActivitySecondaryDayKey], fullActivityClock(minus7, 8, 0), timePointer(fullActivityClock(minus7, 9, 0)))
	assertActivityRunner(t, runnerRows, s.ids.runnerSessions["interrupted_r3_minus14"], "interrupted_r3_minus14", runnersession.RunnerSessionStatusInterrupted, s.ids.users["r3"], s.ids.sessionInstances[fullActivityStampedDayKey(minus14)], fullActivityClock(minus14, 8, 15), timePointer(fullActivityClock(minus14, 8, 25)))
	today := fullDate(s.now)
	assertActivityRunner(t, runnerRows, s.ids.runnerSessions["wip_r1_today"], "wip_r1_today", runnersession.RunnerSessionStatusWip, s.ids.users["r1"], s.ids.sessionInstances["today_a_main"], fullActivityClock(today, 8, 0), nil)

	var feedbackRows []dbs.WorkoutFeedback
	require.NoError(t, db.Unscoped().Where("id IN ?", fullActivityIDs(s.ids.workoutFeedback)).Find(&feedbackRows).Error)
	statuses := map[string]bool{}
	var coachOwned, deletedID int64
	for _, feedback := range feedbackRows {
		if feedback.CompletionStatus != nil {
			statuses[*feedback.CompletionStatus] = true
		}
		if feedback.FeedbackOwnerUserID == s.ids.users["trainer_1"] {
			coachOwned = feedback.ID
		}
		if feedback.DeletedAt != nil {
			deletedID = feedback.ID
		}
	}
	assert.True(t, statuses[comparison.SetStatusCompleted])
	assert.True(t, statuses[comparison.SetStatusSkipped])
	assert.True(t, statuses[comparison.SetStatusUnregistered])
	assert.Positive(t, coachOwned)
	assert.Positive(t, deletedID)

	var gpsFeedbacks []dbs.WorkoutFeedback
	require.NoError(t, db.Where("id IN ?", []int64{s.ids.workoutFeedback["completed_r1_minus14_set1"], s.ids.workoutFeedback["completed_r4_minus7_set2"]}).Find(&gpsFeedbacks).Error)
	for _, feedback := range gpsFeedbacks {
		require.NotNil(t, feedback.PaceSecPerKM)
		require.NotNil(t, feedback.PeakSpeedKPH)
		assert.Positive(t, *feedback.PaceSecPerKM)
		assert.Positive(t, *feedback.PeakSpeedKPH)
	}

	feedbackService := services.NewWorkoutFeedbackService(daos.NewWorkoutFeedbackDao(db), daos.NewSessionInstanceDao(db))
	teamID, groupMainID := s.ids.teams["team_a"], s.ids.groups["group_a_main"]
	minus14Date := minus14.Format("2006-01-02")
	history, err := feedbackService.AdministeredHistory(nil, s.ids.users["trainer_1"], s.ids.users["trainer_1"], workoutfeedback.HistoryQuery{
		TeamID: &teamID, GroupID: &groupMainID, DateFrom: &minus14Date, DateTo: &minus14Date,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, history.Total)
	assert.NotContains(t, historyItemIDs(history.Items), deletedID)

	exerciseID := s.ids.exercises["full_speed_distance"]
	setNumber := 1
	filtered, err := feedbackService.AdministeredHistory(nil, s.ids.users["trainer_1"], s.ids.users["trainer_1"], workoutfeedback.HistoryQuery{
		TeamID: &teamID, GroupID: &groupMainID, DateFrom: &minus14Date, DateTo: &minus14Date,
		ExerciseID: &exerciseID, SetNumber: &setNumber,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, filtered.Total)
	require.Len(t, filtered.Items, 1)
	assert.Equal(t, s.ids.workoutFeedback["completed_r1_minus14_set1"], filtered.Items[0].ID)
	assert.Equal(t, comparison.SetStatusCompleted, *filtered.Items[0].CompletionStatus)

	groupSecondaryID := s.ids.groups["group_a_secondary"]
	minus7Date := minus7.Format("2006-01-02")
	secondaryHistory, err := feedbackService.AdministeredHistory(nil, s.ids.users["trainer_1"], s.ids.users["trainer_1"], workoutfeedback.HistoryQuery{
		TeamID: &teamID, GroupID: &groupSecondaryID, DateFrom: &minus7Date, DateTo: &minus7Date,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, secondaryHistory.Total)
	var trainerOwned dbs.WorkoutFeedback
	require.NoError(t, db.First(&trainerOwned, coachOwned).Error)
	assert.Equal(t, s.ids.users["r4"], trainerOwned.AthleteUserID)

	attendanceService := services.NewAttendanceService(
		daos.NewAttendanceDao(db), daos.NewGroupDao(db), daos.NewGroupUserDao(db), daos.NewUserDao(db), "",
	)
	todayGrid, err := attendanceService.GetSessionAttendance(nil, s.ids.users["trainer_1"], teamID, groupMainID, s.ids.sessionInstances["today_a_main"])
	require.NoError(t, err)
	assert.Greater(t, todayGrid.Summary.NotConfirmed, int64(0))
	rosterByUser := make(map[int64]attendance.SessionAttendanceRow, len(todayGrid.Roster))
	for _, row := range todayGrid.Roster {
		rosterByUser[row.UserID] = row
	}
	assert.Equal(t, attendance.SessionAttendanceStatusAttended, rosterByUser[s.ids.users["r1"]].Status)
	require.NotNil(t, rosterByUser[s.ids.users["r1"]].Source)
	assert.Equal(t, "qr", *rosterByUser[s.ids.users["r1"]].Source)
	assert.Equal(t, attendance.SessionAttendanceStatusAttended, rosterByUser[s.ids.users["r2"]].Status)
	require.NotNil(t, rosterByUser[s.ids.users["r2"]].Source)
	assert.Equal(t, "manual", *rosterByUser[s.ids.users["r2"]].Source)
	assert.Contains(t, rosterStatuses(rosterByUser), attendance.SessionAttendanceStatusNotConfirmed)

	var attendanceRows []dbs.Attendance
	require.NoError(t, db.Where("id IN ?", fullActivityIDs(s.ids.attendances)).Find(&attendanceRows).Error)
	assertAttendanceDateSources(t, attendanceRows, db, s.ids.sessionInstances, s.now)
}

func TestFullSeederSeedActivity_DoesNotOverwriteForeignRunnerSession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullActivityTestConfig(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.July, 18, 15, 0, 0, 0, gametime.Location)
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	require.NoError(t, s.seedCatalogCalendar())
	foreign := dbs.RunnerSession{
		SessionInstanceID: s.ids.sessionInstances["today_a_main"],
		AthleteUserID:     s.ids.users["r1"],
		Status:            runnersession.RunnerSessionStatusFinished,
		StartDate:         s.now.Add(-time.Hour), EndDate: timePtr(s.now.Add(-time.Minute)),
	}
	require.NoError(t, db.Create(&foreign).Error)
	before := fullActivityCounts(t, db)

	err := s.seedActivity()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "actividad ajena")
	assert.Equal(t, before, fullActivityCounts(t, db))
	assert.Empty(t, s.ids.runnerSessions)
	assert.Empty(t, s.ids.workoutFeedback)
	assert.Empty(t, s.ids.attendances)
	assert.NotContains(t, s.ids.sessionInstances, "activity_a_secondary_minus7")
	var unchanged dbs.RunnerSession
	require.NoError(t, db.First(&unchanged, foreign.ID).Error)
	assert.Equal(t, runnersession.RunnerSessionStatusFinished, unchanged.Status)
}

func seedFullActivityTestConfig(t *testing.T) {
	t.Helper()
	oldClientID, oldKey, oldSeeding := config.MyMP.OAuthClientID, config.TokenEncryptionKey, config.Seeding
	oldJWTSecret, oldJWTIssuer, oldJWTAudience := config.JWTSecret, config.JWTIssuer, config.JWTAudience
	config.MyMP.OAuthClientID, config.TokenEncryptionKey, config.Seeding = fullCalendarTestClientID, fullCalendarTestKey, true
	config.JWTSecret, config.JWTIssuer, config.JWTAudience = "test-full-seed-jwt-secret", "test-full-seed", "test-full-seed"
	t.Cleanup(func() {
		config.MyMP.OAuthClientID, config.TokenEncryptionKey, config.Seeding = oldClientID, oldKey, oldSeeding
		config.JWTSecret, config.JWTIssuer, config.JWTAudience = oldJWTSecret, oldJWTIssuer, oldJWTAudience
	})
}

func fullActivityCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Count(&count).Error)
	return count
}

func fullActivityCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	return map[string]int64{
		"calendar":       fullActivityCount(t, db, &dbs.GroupCalendarDay{}),
		"sessions":       fullActivityCount(t, db, &dbs.SessionInstance{}),
		"exerciseLinks":  fullActivityCount(t, db, &dbs.SessionExerciseInstance{}),
		"runner":         fullActivityCount(t, db, &dbs.RunnerSession{}),
		"feedback":       fullActivityCount(t, db, &dbs.WorkoutFeedback{}),
		"feedbackPoints": fullActivityCount(t, db, &dbs.WorkoutFeedbackPoint{}),
		"attendance":     fullActivityCount(t, db, &dbs.Attendance{}),
	}
}

func fullActivityIDs(ids map[string]int64) []int64 {
	values := make([]int64, 0, len(ids))
	for _, id := range ids {
		values = append(values, id)
	}
	return values
}

func assertActivityRunner(t *testing.T, rows []dbs.RunnerSession, id int64, key, status string, athleteID, instanceID int64, start time.Time, end *time.Time) {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			assert.Equal(t, status, row.Status, key)
			assert.Equal(t, athleteID, row.AthleteUserID, key)
			assert.Equal(t, instanceID, row.SessionInstanceID, key)
			assert.True(t, row.StartDate.Equal(start), key)
			assert.True(t, sameTimePointer(row.EndDate, end), key)
			return
		}
	}
	t.Fatalf("no se encontró runner_session %s con status %s", key, status)
}

func historyItemIDs(items []workoutfeedback.WorkoutFeedbackHistoryItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func rosterStatuses(rows map[int64]attendance.SessionAttendanceRow) []string {
	statuses := make([]string, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, row.Status)
	}
	return statuses
}

func assertAttendanceDateSources(t *testing.T, rows []dbs.Attendance, db *gorm.DB, instances map[string]int64, now time.Time) {
	t.Helper()
	seen := map[string]bool{}
	for _, row := range rows {
		var day dbs.GroupCalendarDay
		require.NoError(t, db.Where("session_instance_id = ?", row.TrainingSessionID).Take(&day).Error)
		switch day.Date.Format("2006-01-02") {
		case fullDate(now.AddDate(0, 0, -14)).Format("2006-01-02"):
			seen["minus14_"+row.Source] = true
		case fullDate(now.AddDate(0, 0, -7)).Format("2006-01-02"):
			seen["minus7_"+row.Source] = true
		case fullDate(now).Format("2006-01-02"):
			seen["today_"+row.Source] = true
		}
	}
	assert.True(t, seen["minus14_qr"])
	assert.True(t, seen["minus14_manual"])
	assert.True(t, seen["minus7_qr"])
	assert.True(t, seen["minus7_manual"])
	assert.True(t, seen["today_qr"])
	assert.True(t, seen["today_manual"])
	assert.Positive(t, instances["today_a_main"])
}
