package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/controllers"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const (
	fullCalendarTestClientID = "seed-full-calendar-test-client"
	fullCalendarTestKey      = "12345678901234567890123456789012"
)

func TestFullSeederSeedCatalogCalendar_SeedsCalendarAndAPIContracts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullCalendarPrerequisites(t)

	s := newFullSeeder(db, options{})
	s.now = gametime.Now().In(gametime.Location).Truncate(time.Second)
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	require.NoError(t, s.seedCatalogCalendar())
	assertFullCatalogContents(t, db, s)
	before := fullCalendarTableCounts(t, db)
	require.NoError(t, s.seedCatalogCalendar())
	assert.Equal(t, before, fullCalendarTableCounts(t, db))
	assert.Positive(t, s.ids.calendarDays["full_21_days_"+fullCalendarDate(s.now.AddDate(0, 0, -1)).Format("2006-01-02")])
	assert.Positive(t, s.ids.sessionInstances["full_same_team_"+fullCalendarDate(s.now.AddDate(0, 0, 1)).Format("2006-01-02")])

	rangeStart := fullCalendarDate(s.now.AddDate(0, 0, -21))
	rangeEnd := fullCalendarDate(s.now.AddDate(0, 0, 14))
	calendarService := fullCalendarService(db)
	days, err := calendarService.GetRange(nil, s.ids.groups["group_a_main"], s.ids.users["trainer_1"], rangeStart, rangeEnd)
	require.NoError(t, err)
	require.Len(t, days, 36)

	today := fullCalendarFindDay(t, days, s.now)
	require.Equal(t, string(constants.GroupCalendarDayKindTraining), today.Kind)
	require.True(t, today.IsPresencial)
	require.NotNil(t, today.SessionInstance)
	nowClock := fullClock(s.now)
	assert.LessOrEqual(t, *today.PresencialTimeFrom, nowClock.Format("15:04"))
	assert.GreaterOrEqual(t, *today.PresencialTimeTo, nowClock.Format("15:04"))
	roles := make(map[string]int, len(today.SessionInstance.Exercises))
	for _, exercise := range today.SessionInstance.Exercises {
		roles[exercise.Role]++
	}
	assert.Positive(t, roles[string(constants.SessionExerciseRoleWarmup)])
	assert.Positive(t, roles[string(constants.SessionExerciseRoleMain)])
	assert.Positive(t, roles[string(constants.SessionExerciseRoleCooldown)])
	bToday, err := calendarService.GetRange(nil, s.ids.groups["group_b"], s.ids.users["trainer_2"], fullCalendarDate(s.now), fullCalendarDate(s.now))
	require.NoError(t, err)
	require.Len(t, bToday, 1)
	assert.Equal(t, string(constants.GroupCalendarDayKindTraining), bToday[0].Kind)
	assert.False(t, bToday[0].IsPresencial)
	assert.NotNil(t, bToday[0].SessionInstance)

	for _, day := range days {
		if day.Kind == string(constants.GroupCalendarDayKindTraining) || day.Kind == string(constants.GroupCalendarDayKindCancelled) {
			assert.NotNil(t, day.SessionInstance, day.Date)
		}
	}
	presencialFuture := fullCalendarFindDay(t, days, s.now.AddDate(0, 0, 1))
	assert.Equal(t, string(constants.GroupCalendarDayKindTraining), presencialFuture.Kind)
	assert.True(t, presencialFuture.IsPresencial)
	asyncFuture := fullCalendarFindDay(t, days, s.now.AddDate(0, 0, 4))
	assert.Equal(t, string(constants.GroupCalendarDayKindTraining), asyncFuture.Kind)
	assert.False(t, asyncFuture.IsPresencial)
	assertCalendarKindOnOffset(t, days, s.now, 7, string(constants.GroupCalendarDayKindOther))
	cancelled := fullCalendarFindDay(t, days, s.now.AddDate(0, 0, 12))
	assert.Equal(t, string(constants.GroupCalendarDayKindCancelled), cancelled.Kind)
	require.NotNil(t, cancelled.CancelledReason)
	assert.NotEmpty(t, *cancelled.CancelledReason)
	assertCalendarKindOnOffset(t, days, s.now, 14, string(constants.GroupCalendarDayKindRest))

	var pastCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).
		Where("group_id = ? AND date BETWEEN ? AND ? AND source_plan_id = ?", s.ids.groups["group_a_main"], rangeStart, fullCalendarDate(s.now.AddDate(0, 0, -1)), s.ids.trainingPlans["full_21_days"]).
		Count(&pastCount).Error)
	assert.Equal(t, int64(21), pastCount)

	mainID := s.ids.sessionInstances["today_a_main"]
	secondaryID := s.ids.sessionInstances["today_a_secondary"]
	closedID := s.ids.sessionInstances["today_f_closed"]
	var closedDay dbs.GroupCalendarDay
	require.NoError(t, db.Where("id = ?", s.ids.calendarDays["today_f_closed"]).Take(&closedDay).Error)
	require.NotNil(t, closedDay.PresencialOpenedAt)
	require.NotNil(t, closedDay.PresencialClosedAt)
	assert.True(t, closedDay.PresencialOpenedAt.Before(*closedDay.PresencialClosedAt))
	assertRunnerSessionStatus(t, db, mainID, s.ids.users["r1"], s.now, http.StatusCreated, "")
	assertRunnerSessionStatus(t, db, secondaryID, s.ids.users["r4"], s.now, http.StatusConflict, "session_not_opened")
	assertRunnerSessionStatus(t, db, closedID, s.ids.users["r10"], s.now, http.StatusConflict, "session_closed")

	next, err := calendarService.NextSession(nil, s.ids.users["r1"])
	require.NoError(t, err)
	require.NotNil(t, next.NextTraining)
	require.NotNil(t, next.NextCancelled)
	assert.True(t, next.NextTraining.Date >= s.now.Format("2006-01-02"))
	nextPresencial, err := calendarService.NextPresencialSession(nil, s.ids.users["trainer_1"])
	require.NoError(t, err)
	require.NotNil(t, nextPresencial)
	assert.NotEmpty(t, nextPresencial.SessionName)

	secondaryDay := fullCalendarDate(s.now.AddDate(0, 0, 1))
	from, to := "18:00", "20:00"
	isPresencial := true
	sessionID := s.ids.sessions["full_interval"]
	location := trainingplan.Location{Lat: -31.4201, Lng: -64.1888, Label: strPtr("Parque del equipo")}
	updated, err := calendarService.UpsertDay(nil, s.ids.groups["group_a_secondary"], s.ids.users["trainer_1"], secondaryDay, calendar.CalendarDayRequest{
		Kind: string(constants.GroupCalendarDayKindTraining), SessionID: &sessionID, IsPresencial: &isPresencial,
		PresencialTimeFrom: &from, PresencialTimeTo: &to, PresencialLocation: &location,
	})
	require.NoError(t, err)
	require.NotEmpty(t, updated.SameTeamWarnings)
	assert.Equal(t, s.ids.groups["group_a_main"], updated.SameTeamWarnings[0].GroupID)

	crossDay := fullCalendarDate(s.now.AddDate(0, 0, 1))
	_, err = calendarService.UpsertDay(nil, s.ids.groups["group_f"], s.ids.users["trainer_1"], crossDay, calendar.CalendarDayRequest{
		Kind: string(constants.GroupCalendarDayKindTraining), SessionID: &sessionID, IsPresencial: &isPresencial,
		PresencialTimeFrom: &from, PresencialTimeTo: &to, PresencialLocation: &location,
	})
	assert.ErrorIs(t, err, services.ErrCalendarPresencialCollision)

	administered, err := calendarService.AdministeredCalendar(nil, s.ids.users["trainer_1"], rangeStart, rangeEnd)
	require.NoError(t, err)
	var crossCollision, sameTeamPair bool
	for _, day := range administered {
		if day.Date == s.now.Format("2006-01-02") && day.GroupID == s.ids.groups["group_a_main"] {
			if day.PresencialCollision != nil && day.PresencialCollision.Type == "cross_team" {
				for _, conflict := range day.PresencialCollision.Conflicts {
					crossCollision = crossCollision || conflict.TeamID == s.ids.teams["team_f"]
				}
			}
		}
		if day.Date == secondaryDay.Format("2006-01-02") && day.GroupID == s.ids.groups["group_a_secondary"] {
			sameTeamPair = day.TeamID == s.ids.teams["team_a"]
		}
	}
	assert.True(t, crossCollision)
	assert.True(t, sameTeamPair)
}

func TestFullCalendarClockRangeCapsAtEndOfDay(t *testing.T) {
	cases := []struct {
		name     string
		now      time.Time
		wantFrom string
		wantTo   string
	}{
		{
			name:     "normal hours",
			now:      time.Date(2026, time.November, 3, 18, 0, 0, 0, gametime.Location),
			wantFrom: "17:30",
			wantTo:   "20:00",
		},
		{
			name:     "late evening",
			now:      time.Date(2026, time.November, 3, 23, 0, 0, 0, gametime.Location),
			wantFrom: "22:30",
			wantTo:   "23:59",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to, err := fullCalendarClockRange(tc.now)
			require.NoError(t, err)
			assert.Equal(t, tc.wantFrom, from.UTC().Format("15:04"))
			assert.Equal(t, tc.wantTo, to.UTC().Format("15:04"))
		})
	}
}

func TestFullSeederSeedCatalogCalendar_ClampsTodayWindowAtLocalDayBoundaries(t *testing.T) {
	today := time.Now().In(gametime.Location)
	onToday := func(hour, minute int) time.Time {
		return time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, gametime.Location)
	}
	cases := []struct {
		name                 string
		now                  time.Time
		wantFrom             string
		wantTo               string
		wantOpened           string
		wantClosedTime       string
		allowEqualAtMidnight bool
	}{
		{
			name:           "normal hours",
			now:            onToday(18, 0),
			wantFrom:       "17:30",
			wantTo:         "20:00",
			wantOpened:     "17:40",
			wantClosedTime: "17:55",
		},
		{
			name:           "late evening",
			now:            onToday(22, 8),
			wantFrom:       "21:38",
			wantTo:         "23:59",
			wantOpened:     "21:48",
			wantClosedTime: "22:03",
		},
		{
			name:           "shortly after midnight",
			now:            onToday(0, 15),
			wantFrom:       "00:00",
			wantTo:         "02:15",
			wantOpened:     "00:00",
			wantClosedTime: "00:10",
		},
		{
			name:           "first minutes after midnight",
			now:            onToday(0, 2),
			wantFrom:       "00:00",
			wantTo:         "02:02",
			wantOpened:     "00:00",
			wantClosedTime: "00:02",
		},
		{
			name:                 "exact midnight",
			now:                  onToday(0, 0),
			wantFrom:             "00:00",
			wantTo:               "02:00",
			wantOpened:           "00:00",
			wantClosedTime:       "00:00",
			allowEqualAtMidnight: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			seedFullCalendarPrerequisites(t)

			s := newFullSeeder(db, options{})
			s.now = tc.now
			require.NoError(t, s.seedIdentity())
			require.NoError(t, s.seedTeams())
			require.NoError(t, s.seedCatalogCalendar())

			var day dbs.GroupCalendarDay
			require.NoError(t, db.Where("group_id = ? AND date = ?", s.ids.groups["group_a_main"], fullCalendarDate(s.now)).Take(&day).Error)
			require.NotNil(t, day.PresencialTimeFrom)
			require.NotNil(t, day.PresencialTimeTo)
			require.NotNil(t, day.PresencialOpenedAt)
			assert.Nil(t, day.PresencialClosedAt)
			assert.Equal(t, tc.wantFrom, day.PresencialTimeFrom.UTC().Format("15:04"))
			assert.Equal(t, tc.wantTo, day.PresencialTimeTo.UTC().Format("15:04"))
			assert.Equal(t, tc.wantOpened, day.PresencialOpenedAt.In(gametime.Location).Format("15:04"))

			var closedDay dbs.GroupCalendarDay
			require.NoError(t, db.Where("group_id = ? AND date = ?", s.ids.groups["group_f"], fullCalendarDate(s.now)).Take(&closedDay).Error)
			require.NotNil(t, closedDay.PresencialOpenedAt)
			require.NotNil(t, closedDay.PresencialClosedAt)
			dayStart := fullCalendarDate(s.now)
			nextDay := dayStart.AddDate(0, 0, 1)
			assert.False(t, closedDay.PresencialOpenedAt.Before(dayStart))
			assert.True(t, closedDay.PresencialOpenedAt.Before(nextDay))
			assert.False(t, closedDay.PresencialClosedAt.Before(dayStart))
			assert.True(t, closedDay.PresencialClosedAt.Before(nextDay))
			assert.False(t, closedDay.PresencialOpenedAt.After(s.now))
			assert.False(t, closedDay.PresencialClosedAt.After(s.now))
			if tc.allowEqualAtMidnight {
				assert.True(t, closedDay.PresencialOpenedAt.Equal(*closedDay.PresencialClosedAt))
			} else {
				assert.True(t, closedDay.PresencialOpenedAt.Before(*closedDay.PresencialClosedAt))
			}
			assert.Equal(t, tc.wantClosedTime, closedDay.PresencialClosedAt.In(gametime.Location).Format("15:04"))
		})
	}
}

func seedFullCalendarPrerequisites(t *testing.T) {
	t.Helper()
	oldClientID, oldKey, oldSeeding := config.MyMP.OAuthClientID, config.TokenEncryptionKey, config.Seeding
	config.MyMP.OAuthClientID = fullCalendarTestClientID
	config.TokenEncryptionKey = fullCalendarTestKey
	config.Seeding = true
	t.Cleanup(func() {
		config.MyMP.OAuthClientID, config.TokenEncryptionKey, config.Seeding = oldClientID, oldKey, oldSeeding
	})
}

func assertFullCatalogContents(t *testing.T, db *gorm.DB, s *fullSeeder) {
	t.Helper()
	var exercises []dbs.Exercise
	require.NoError(t, db.Where("owner_id IN ? AND description LIKE ?", []int64{s.ids.users["trainer_1"], s.ids.users["trainer_2"]}, "%"+fullSeedMarker+"%").Find(&exercises).Error)
	kinds := make(map[string]bool)
	for _, exercise := range exercises {
		kinds[exercise.Kind] = true
	}
	for _, kind := range constants.CurrentExerciseKinds() {
		assert.True(t, kinds[kind], "falta kind %s", kind)
	}
	assert.Len(t, s.ids.exercises, 9)
	assert.Len(t, s.ids.sessions, 3)
	assert.Len(t, s.ids.trainingPlans, 4)
	for _, id := range s.ids.exercises {
		var exercise dbs.Exercise
		require.NoError(t, db.First(&exercise, id).Error)
		assert.Contains(t, *exercise.Description, fullSeedMarker)
	}
	for _, id := range s.ids.sessions {
		var session dbs.Session
		require.NoError(t, db.First(&session, id).Error)
		assert.Contains(t, *session.Description, fullSeedMarker)
	}
	for _, id := range s.ids.trainingPlans {
		var plan dbs.TrainingPlan
		require.NoError(t, db.First(&plan, id).Error)
		assert.Contains(t, *plan.Description, fullSeedMarker)
	}

	for _, spec := range fullSessionSpecs() {
		var links []dbs.SessionExercise
		require.NoError(t, db.Where("session_id = ?", s.ids.sessions[spec.key]).Find(&links).Error)
		roles := make(map[string]int)
		for _, link := range links {
			roles[link.Role]++
		}
		assert.Positive(t, roles[string(constants.SessionExerciseRoleWarmup)], spec.key)
		assert.Positive(t, roles[string(constants.SessionExerciseRoleMain)], spec.key)
		assert.Positive(t, roles[string(constants.SessionExerciseRoleCooldown)], spec.key)
	}

	for _, spec := range fullTrainingPlanSpecs() {
		var days []dbs.PlanDay
		require.NoError(t, db.Where("plan_id = ?", s.ids.trainingPlans[spec.key]).Order("sequence_no").Find(&days).Error)
		require.Len(t, days, spec.days, spec.key)
		for index, day := range days {
			assert.Equal(t, index+1, day.SequenceNo, spec.key)
		}
	}
}

func fullCalendarTableCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	counts := make(map[string]int64)
	groupIDs := []int64{fullGroupAMainID, fullGroupASecondaryID, fullGroupBID, fullGroupCID, fullGroupDID, fullGroupEID, fullGroupFID}
	models := map[string]any{
		"calendar": &dbs.GroupCalendarDay{}, "session_instances": &dbs.SessionInstance{},
		"exercise_instances": &dbs.ExerciseInstance{}, "session_exercise_instances": &dbs.SessionExerciseInstance{},
	}
	for key, model := range models {
		query := db.Model(model)
		if key == "calendar" {
			query = query.Where("group_id IN ?", groupIDs)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			require.NoError(t, err)
		}
		counts[key] = count
	}
	return counts
}

func assertRunnerSessionStatus(t *testing.T, db *gorm.DB, instanceID, userID int64, now time.Time, status int, errorCode string) {
	t.Helper()
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	presencial := services.NewPresencialSessionService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db))
	runnerService := services.NewRunnerSessionService(daos.NewRunnerSessionDao(db), presencial)
	controller := controllers.NewRunnerSessionController(runnerService, presencial, nil, nil, nil)
	body, err := json.Marshal(runnersession.CreateRunnerSessionRequest{StartDate: now})
	require.NoError(t, err)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/session-instances/"+fmt.Sprint(instanceID)+"/runner", strings.NewReader(string(body)))
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(instanceID)}}
	ctx.Set(utils.AuthUserIDKey, userID)
	controller.Create(ctx)
	assert.Equal(t, status, response.Code, response.Body.String())
	if errorCode != "" {
		var payload map[string]any
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		assert.Equal(t, errorCode, payload["code"])
	}
}

func fullCalendarDate(value time.Time) time.Time {
	year, month, day := value.In(gametime.Location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, gametime.Location)
}

func fullCalendarFindDay(t *testing.T, days []calendar.CalendarDayResponse, date time.Time) calendar.CalendarDayResponse {
	t.Helper()
	for _, day := range days {
		if day.Date == date.Format("2006-01-02") {
			return day
		}
	}
	t.Fatalf("no hay día calendario para %s", date.Format("2006-01-02"))
	return calendar.CalendarDayResponse{}
}

func fullCalendarHHMM(value time.Time) string {
	return value.In(gametime.Location).Format("15:04")
}

func assertCalendarKindOnOffset(t *testing.T, days []calendar.CalendarDayResponse, now time.Time, offset int, kind string) {
	t.Helper()
	day := fullCalendarFindDay(t, days, now.AddDate(0, 0, offset))
	assert.Equal(t, kind, day.Kind)
}
