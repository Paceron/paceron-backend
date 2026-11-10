package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func TestFullCleanupSteps_VacioYCompleto(t *testing.T) {
	assert.Empty(t, fullCleanupSteps(fullCleanupState{}))

	state := fullCleanupState{
		users: []int64{1, 2}, teams: []int64{10}, groups: []int64{20},
		plans: []int64{30}, sessions: []int64{40}, exercises: []int64{50},
		calendarDays: []int64{60}, calendarInstances: []int64{61},
		competitionIDs: []int64{70}, competitionInstances: []int64{71},
		runnerIDs: []int64{80}, activityFeedbackIDs: []int64{81}, competitionFeedbackIDs: []int64{82},
		attendanceIDs: []int64{90}, messageIDs: []int64{100}, messageRecipientIDs: []int64{101},
		exerciseInstances: []int64{110}, subscriptionIDs: []int64{120}, installmentIDs: []int64{121},
		paymentIDs: []int64{130}, teamInstallmentIDs: []int64{131}, sellerConnectionIDs: []int64{140},
		fullUserIDs: map[string]int64{"r4": 1, "trainer_1": 2},
		fullTeamIDs: map[string]int64{"team_a": 10},
		fullGroupIDs: map[string]int64{
			"group_a_main": 20,
		},
	}

	steps := fullCleanupSteps(state)
	require.NotEmpty(t, steps)

	tables := make(map[string]bool, len(steps))
	for _, step := range steps {
		tables[step.Table] = true
	}
	for _, table := range []string{
		"runner_session", "workout_feedback", "attendances", "session_messages",
		"competition", "gamification_stamp", "payments", "installments",
		"session_instances", "group_calendar_days", "training_plans", "sessions",
		"exercises", "groups", "teams", "users", "seller_connections",
	} {
		assert.True(t, tables[table], "falta el paso de %s", table)
	}
}

func TestFullGroupIDListYContainsID(t *testing.T) {
	ids := fullGroupIDList(map[string]int64{"a": 1, "b": 1, "c": 2})
	assert.ElementsMatch(t, []int64{1, 2}, ids)
	assert.Empty(t, fullGroupIDList(nil))

	assert.True(t, containsID([]int64{1, 2, 3}, 2))
	assert.False(t, containsID([]int64{1, 2, 3}, 4))
	assert.False(t, containsID(nil, 1))
}

func TestFullCalendarKey_RoundTripYError(t *testing.T) {
	date := time.Date(2026, time.September, 28, 15, 30, 0, 0, time.UTC)
	key := fullCalendarKey(fullGroupAMainID, date)
	assert.Equal(t, "2001/2026-09-28", key)

	groupID, parsed, err := parseFullCalendarKey(key)
	require.NoError(t, err)
	assert.Equal(t, fullGroupAMainID, groupID)
	assert.Equal(t, "2026-09-28", parsed.Format(dateLayout))

	_, _, err = parseFullCalendarKey("sin-formato")
	require.Error(t, err)
}

func TestFullRunnerKey_IdentificaInstanteYFin(t *testing.T) {
	cordoba := time.Date(2026, time.October, 13, 8, 0, 0, 0, gametime.Location)
	utc := cordoba.In(time.UTC)
	end := cordoba.Add(time.Hour)
	endUTC := end.UTC()

	assert.Equal(t, fullRunnerKey(1, 2, "finished", cordoba, &end),
		fullRunnerKey(1, 2, "finished", utc, &endUTC))
	assert.NotEqual(t, fullRunnerKey(1, 2, "finished", cordoba, nil),
		fullRunnerKey(1, 2, "finished", cordoba, &end))
	assert.NotEqual(t, fullRunnerKey(1, 2, "finished", cordoba, nil),
		fullRunnerKey(1, 2, "wip", cordoba, nil))
}

func TestFullCleanupSeedTime_AnclaYFallback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	oldNow := gametime.Now
	now := time.Date(2026, time.October, 13, 15, 0, 0, 0, gametime.Location)
	gametime.Now = func() time.Time { return now }
	t.Cleanup(func() { gametime.Now = oldNow })

	assert.Equal(t, now, fullCleanupSeedTime(db, &fullCleanupState{}))

	anchor := time.Date(2026, time.September, 28, 0, 0, 0, 0, gametime.Location)
	date := anchor.AddDate(0, 0, 21)
	opened := time.Date(date.Year(), date.Month(), date.Day(), 7, 30, 0, 0, gametime.Location)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: fullGroupAMainID, Date: date, Kind: "training", PresencialOpenedAt: &opened,
	}).Error)

	got := fullCleanupSeedTime(db, &fullCleanupState{anchor: anchor})
	assert.Equal(t, 7, got.Hour())
	assert.Equal(t, 50, got.Minute())
}

func TestCollectFullCleanupState_BaseVaciaYErrorDeDB(t *testing.T) {
	db := testutils.SetupTestDB(t)

	state, err := collectFullCleanupState(db)
	require.NoError(t, err)
	assert.Empty(t, state.users)

	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	_, err = collectFullCleanupState(failing)
	require.Error(t, err)
}
