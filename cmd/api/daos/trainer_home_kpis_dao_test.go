package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestTrainerHomeKpisDao_SessionCountsUsesCalendarSessionsAndAttendance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTrainerHomeKpisDao(db)
	owner := persistUser(db, "trainer-home-session-owner@test.com", "81000001")
	otherOwner := persistUser(db, "trainer-home-session-other@test.com", "81000002")
	team := testTeam(db, "trainer home sessions", owner.ID)
	otherTeam := testTeam(db, "trainer home other", otherOwner.ID)
	group := testGroup(db, "trainer home group", team.ID)
	otherGroup := testGroup(db, "trainer home other group", otherTeam.ID)
	date := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)

	first := &dbs.SessionInstance{Name: "Presencial"}
	second := &dbs.SessionInstance{Name: "Programada"}
	cancelled := &dbs.SessionInstance{Name: "Cancelada"}
	foreign := &dbs.SessionInstance{Name: "Otro entrenador"}
	for _, session := range []*dbs.SessionInstance{first, second, cancelled, foreign} {
		require.NoError(t, db.Create(session).Error)
	}
	calendarDays := []dbs.GroupCalendarDay{
		{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &first.ID, IsPresencial: true},
		{GroupID: group.ID, Date: date.AddDate(0, 0, 1), Kind: "training", SessionInstanceID: &second.ID},
		{GroupID: group.ID, Date: date.AddDate(0, 0, 2), Kind: "cancelled", SessionInstanceID: &cancelled.ID, IsPresencial: true},
		{GroupID: otherGroup.ID, Date: date, Kind: "training", SessionInstanceID: &foreign.ID, IsPresencial: true},
	}
	for i := range calendarDays {
		require.NoError(t, db.Create(&calendarDays[i]).Error)
	}
	attendees := []*dbs.Attendance{
		{TeamID: team.ID, TrainingSessionID: first.ID, UserID: 990001, Source: "manual"},
		{TeamID: team.ID, TrainingSessionID: first.ID, UserID: 990002, Source: "qr"},
		{TeamID: otherTeam.ID, TrainingSessionID: foreign.ID, UserID: 990003, Source: "qr"},
	}
	for _, attendance := range attendees {
		require.NoError(t, db.Create(attendance).Error)
	}

	counts, err := dao.SessionCounts(nil, owner.ID, date, date.AddDate(0, 0, 6))

	require.NoError(t, err)
	assert.EqualValues(t, 2, counts.Scheduled)
	assert.EqualValues(t, 1, counts.Realized, "two attendance rows on one session still mean one realized session")
	assert.EqualValues(t, 1, counts.PresencialScheduled)
	assert.EqualValues(t, 1, counts.PresencialRealized)
}

func TestTrainerHomeKpisDao_StructureCountsUniqueActiveRunnersOfActiveTeams(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTrainerHomeKpisDao(db)
	owner := persistUser(db, "trainer-home-structure-owner@test.com", "82000001")
	otherOwner := persistUser(db, "trainer-home-structure-other@test.com", "82000002")
	teamA := testTeam(db, "trainer home structure A", owner.ID)
	teamB := testTeam(db, "trainer home structure B", owner.ID)
	teamB.Status = "inactive"
	require.NoError(t, db.Save(teamB).Error)
	otherTeam := testTeam(db, "trainer home structure foreign", otherOwner.ID)
	runnerA := persistUser(db, "trainer-home-runner-a@test.com", "82000003")
	runnerB := persistUser(db, "trainer-home-runner-b@test.com", "82000004")
	trainer := &dbs.TeamUser{TeamID: teamA.ID, UserID: owner.ID, RoleInTeam: "entrenador", Status: "active", AssignmentDate: time.Now()}
	sharedA := &dbs.TeamUser{TeamID: teamA.ID, UserID: runnerA.ID, RoleInTeam: "corredor", Status: "active", AssignmentDate: time.Now()}
	sharedB := &dbs.TeamUser{TeamID: teamA.ID, UserID: runnerB.ID, RoleInTeam: "corredor", Status: "active", AssignmentDate: time.Now()}
	duplicate := &dbs.TeamUser{TeamID: teamB.ID, UserID: runnerA.ID, RoleInTeam: "corredor", Status: "active", AssignmentDate: time.Now()}
	foreign := &dbs.TeamUser{TeamID: otherTeam.ID, UserID: runnerB.ID, RoleInTeam: "corredor", Status: "active", AssignmentDate: time.Now()}
	for _, membership := range []*dbs.TeamUser{trainer, sharedA, sharedB, duplicate, foreign} {
		require.NoError(t, db.Create(membership).Error)
	}
	inactive := &dbs.TeamUser{TeamID: teamA.ID, UserID: 82000999, RoleInTeam: "corredor", Status: "inactive", AssignmentDate: time.Now()}
	require.NoError(t, db.Create(inactive).Error)

	counts, err := dao.StructureCounts(nil, owner.ID)

	require.NoError(t, err)
	assert.EqualValues(t, 1, counts.ActiveTeams)
	assert.EqualValues(t, 2, counts.ActiveRunners)
}
