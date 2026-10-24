package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func cmpDate(day int) time.Time {
	return time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)
}

func cmpDatePtr(day int) *time.Time {
	d := cmpDate(day)
	return &d
}

type comparisonFixture struct {
	owner, otherOwner, athlete, stranger *dbs.User
	team, otherTeam                      *dbs.Team
	group, group2, otherGroup            *dbs.Group
	// s1 (5/1, group), s2 (15/1, group), s3 (25/1, group, después de salir),
	// s5 (8/1, group2, antes de entrar), s6 (18/1, group2), s7 (16/1, otherGroup),
	// s8 sin día (feedback del 17/1 con team).
	s1, s2, s3, s5, s6, s7, s8 *dbs.SessionInstance
	ex1a, ex1b, ex2            *dbs.ExerciseInstance
	fbOwn, fbOrphan            *dbs.WorkoutFeedback
}

func cmpSession(t *testing.T, db *gorm.DB, name string) *dbs.SessionInstance {
	t.Helper()
	s := &dbs.SessionInstance{Name: name}
	require.NoError(t, db.Create(s).Error)
	return s
}

func cmpDay(t *testing.T, db *gorm.DB, groupID int64, day int, kind string, sessionID *int64) {
	t.Helper()
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: groupID, Date: cmpDate(day), Kind: kind, SessionInstanceID: sessionID}).Error)
}

func cmpExerciseLink(t *testing.T, db *gorm.DB, sessionID int64, name string, minutes *int) *dbs.ExerciseInstance {
	t.Helper()
	ex := &dbs.ExerciseInstance{Name: name, Kind: "liviano", Minutes: minutes}
	require.NoError(t, db.Create(ex).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: sessionID, ExerciseInstanceID: ex.ID, Role: "main", RepeatCount: 2}).Error)
	return ex
}

func setupComparisonFixture(t *testing.T, db *gorm.DB) *comparisonFixture {
	t.Helper()
	f := &comparisonFixture{}
	f.owner = persistUser(db, "cmp-owner@test.com", "93000001")
	f.otherOwner = persistUser(db, "cmp-owner-2@test.com", "93000002")
	f.athlete = persistUser(db, "cmp-athlete@test.com", "93000003")
	f.stranger = persistUser(db, "cmp-stranger@test.com", "93000004")
	f.team = testTeam(db, "cmp_team", f.owner.ID)
	f.otherTeam = testTeam(db, "cmp_team_2", f.otherOwner.ID)
	f.group = testGroup(db, "cmp_group", f.team.ID)
	f.group2 = testGroup(db, "cmp_group_2", f.team.ID)
	f.otherGroup = testGroup(db, "cmp_group_other", f.otherTeam.ID)

	// En group del 1/1 al 20/1 (salió: deleted_at). En group2 desde el 10/1.
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.group.ID, UserID: f.athlete.ID, DateStart: cmpDate(1), DeletedAt: cmpDatePtr(20)}).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.group2.ID, UserID: f.athlete.ID, DateStart: cmpDate(10)}).Error)
	require.NoError(t, db.Create(&dbs.TeamUser{TeamID: f.team.ID, UserID: f.athlete.ID, RoleInTeam: "corredor", AssignmentDate: cmpDate(1)}).Error)

	f.s1 = cmpSession(t, db, "S1")
	f.s2 = cmpSession(t, db, "S2")
	f.s3 = cmpSession(t, db, "S3")
	f.s5 = cmpSession(t, db, "S5")
	f.s6 = cmpSession(t, db, "S6")
	f.s7 = cmpSession(t, db, "S7")
	f.s8 = cmpSession(t, db, "S8 sin día")

	cmpDay(t, db, f.group.ID, 5, "training", &f.s1.ID)
	cmpDay(t, db, f.group.ID, 12, "cancelled", nil)
	cmpDay(t, db, f.group.ID, 13, "rest", nil)
	cmpDay(t, db, f.group.ID, 15, "training", &f.s2.ID)
	cmpDay(t, db, f.group.ID, 25, "training", &f.s3.ID)
	cmpDay(t, db, f.group2.ID, 8, "training", &f.s5.ID)
	cmpDay(t, db, f.group2.ID, 18, "training", &f.s6.ID)
	cmpDay(t, db, f.otherGroup.ID, 16, "training", &f.s7.ID)

	ten := 10
	f.ex1a = cmpExerciseLink(t, db, f.s1.ID, "Primero", &ten)
	f.ex1b = cmpExerciseLink(t, db, f.s1.ID, "Segundo", nil)
	f.ex2 = cmpExerciseLink(t, db, f.s2.ID, "Caminata", &ten)

	f.fbOwn = historyFeedback(t, db, f.athlete.ID, f.s2.ID, f.ex2.ID, &f.team.ID, 1, cmpDate(15))
	f.fbOrphan = historyFeedback(t, db, f.athlete.ID, f.s8.ID, 999, &f.team.ID, 1, cmpDate(17))
	// Feedback borrado y de otro atleta: no tienen que aparecer.
	deleted := historyFeedback(t, db, f.athlete.ID, f.s7.ID, 998, nil, 1, cmpDate(16))
	require.NoError(t, db.Model(deleted).Update("deleted_at", time.Now()).Error)
	historyFeedback(t, db, f.stranger.ID, f.s2.ID, f.ex2.ID, nil, 1, cmpDate(15))

	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: f.s2.ID, AthleteUserID: f.athlete.ID, Status: "finished", StartDate: cmpDate(15)}).Error)
	return f
}

func sessionIDs(rows []ComparisonSessionDay) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.SessionInstanceID)
	}
	return ids
}

func TestActivityComparisonDao_ListPlannedDays_UsesMembershipOnEachDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	rows, err := dao.ListPlannedDays(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(1), To: cmpDate(31)})

	require.NoError(t, err)
	// s3 es posterior a la salida, s5 anterior a la entrada, s7 de otro equipo,
	// y los días cancelado y de descanso no cuentan.
	assert.Equal(t, []int64{f.s1.ID, f.s2.ID, f.s6.ID}, sessionIDs(rows))
	assert.Equal(t, "S2", rows[1].SessionName)
	assert.Equal(t, f.team.ID, *rows[1].TeamID)
	assert.Equal(t, "cmp_group", *rows[1].GroupName)
}

func TestActivityComparisonDao_ListPlannedDays_Filters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	byGroup, err := dao.ListPlannedDays(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(1), To: cmpDate(31), TeamID: &f.team.ID, GroupID: &f.group2.ID})
	require.NoError(t, err)
	assert.Equal(t, []int64{f.s6.ID}, sessionIDs(byGroup))

	otherTeam, err := dao.ListPlannedDays(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(1), To: cmpDate(31), TeamID: &f.otherTeam.ID})
	require.NoError(t, err)
	assert.Empty(t, otherTeam)

	narrow, err := dao.ListPlannedDays(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(14), To: cmpDate(16)})
	require.NoError(t, err)
	assert.Equal(t, []int64{f.s2.ID}, sessionIDs(narrow))
}

func TestActivityComparisonDao_ListFeedbackSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	rows, err := dao.ListFeedbackSessions(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(1), To: cmpDate(31)})

	require.NoError(t, err)
	require.Equal(t, []int64{f.s2.ID, f.s8.ID}, sessionIDs(rows))
	// Con día: toma el grupo y la fecha del día.
	assert.Equal(t, f.group.ID, *rows[0].GroupID)
	assert.True(t, rows[0].Date.Equal(cmpDate(15)))
	// Sin día: grupo nil, equipo del feedback y fecha del feedback.
	assert.Nil(t, rows[1].GroupID)
	assert.Equal(t, f.team.ID, *rows[1].TeamID)
	assert.True(t, rows[1].Date.Equal(cmpDate(17)))
	assert.Equal(t, "S8 sin día", rows[1].SessionName)

	byTeam, err := dao.ListFeedbackSessions(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(1), To: cmpDate(31), TeamID: &f.team.ID})
	require.NoError(t, err)
	assert.Equal(t, []int64{f.s2.ID, f.s8.ID}, sessionIDs(byTeam))

	byOtherTeam, err := dao.ListFeedbackSessions(nil, ComparisonRangeFilter{AthleteUserID: f.athlete.ID, From: cmpDate(1), To: cmpDate(31), TeamID: &f.otherTeam.ID})
	require.NoError(t, err)
	assert.Empty(t, byOtherTeam)
}

func TestActivityComparisonDao_FindSessionDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	withDay, err := dao.FindSessionDay(nil, f.s2.ID)
	require.NoError(t, err)
	require.NotNil(t, withDay)
	assert.Equal(t, f.group.ID, *withDay.GroupID)
	assert.True(t, withDay.Date.Equal(cmpDate(15)))

	withoutDay, err := dao.FindSessionDay(nil, f.s8.ID)
	require.NoError(t, err)
	require.NotNil(t, withoutDay)
	assert.Nil(t, withoutDay.GroupID)
	assert.Equal(t, "S8 sin día", withoutDay.SessionName)

	missing, err := dao.FindSessionDay(nil, 987654321)
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestActivityComparisonDao_ListPlannedExercises(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	rows, err := dao.ListPlannedExercises(nil, []int64{f.s1.ID, f.s2.ID})

	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, f.ex1a.ID, rows[0].ExerciseInstanceID)
	assert.Equal(t, f.ex1b.ID, rows[1].ExerciseInstanceID)
	assert.Equal(t, f.ex2.ID, rows[2].ExerciseInstanceID)
	assert.Equal(t, 10, *rows[0].Minutes)
	assert.Nil(t, rows[1].Minutes)
	assert.Equal(t, 2, rows[0].RepeatCount)
	assert.Equal(t, "liviano", rows[0].Kind)

	empty, err := dao.ListPlannedExercises(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestActivityComparisonDao_ListFeedbackAndRunnerSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	feedback, err := dao.ListFeedback(nil, f.athlete.ID, []int64{f.s2.ID, f.s7.ID, f.s8.ID})
	require.NoError(t, err)
	require.Len(t, feedback, 2)
	assert.Equal(t, f.fbOwn.ID, feedback[0].ID)
	assert.Equal(t, f.fbOrphan.ID, feedback[1].ID)

	runnerSessions, err := dao.ListRunnerSessions(nil, f.athlete.ID, []int64{f.s1.ID, f.s2.ID})
	require.NoError(t, err)
	require.Len(t, runnerSessions, 1)
	assert.Equal(t, f.s2.ID, runnerSessions[0].SessionInstanceID)

	emptyFeedback, err := dao.ListFeedback(nil, f.athlete.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, emptyFeedback)
	emptyRunner, err := dao.ListRunnerSessions(nil, f.athlete.ID, []int64{})
	require.NoError(t, err)
	assert.Empty(t, emptyRunner)
}

func TestActivityComparisonDao_OwnershipChecks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := setupComparisonFixture(t, db)
	dao := NewActivityComparisonDao(db)

	byDay, err := dao.IsSessionTeamOwner(nil, f.s2.ID, f.owner.ID)
	require.NoError(t, err)
	assert.True(t, byDay)

	byFeedback, err := dao.IsSessionTeamOwner(nil, f.s8.ID, f.owner.ID)
	require.NoError(t, err)
	assert.True(t, byFeedback)

	notOwner, err := dao.IsSessionTeamOwner(nil, f.s2.ID, f.otherOwner.ID)
	require.NoError(t, err)
	assert.False(t, notOwner)

	member, err := dao.IsTeamMemberOfOwner(nil, f.team.ID, f.athlete.ID, f.owner.ID)
	require.NoError(t, err)
	assert.True(t, member)

	wrongOwner, err := dao.IsTeamMemberOfOwner(nil, f.team.ID, f.athlete.ID, f.otherOwner.ID)
	require.NoError(t, err)
	assert.False(t, wrongOwner)

	notMember, err := dao.IsTeamMemberOfOwner(nil, f.team.ID, f.stranger.ID, f.owner.ID)
	require.NoError(t, err)
	assert.False(t, notMember)
}

func TestActivityComparisonDao_DBErrors(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewActivityComparisonDao(testutils.FailingDB(t, db, func(op string) bool { return op == "select" }))
	filter := ComparisonRangeFilter{AthleteUserID: 1, From: cmpDate(1), To: cmpDate(31)}

	_, err := dao.ListPlannedDays(nil, filter)
	assert.Error(t, err)
	_, err = dao.ListFeedbackSessions(nil, filter)
	assert.Error(t, err)
	_, err = dao.FindSessionDay(nil, 1)
	assert.Error(t, err)
	_, err = dao.ListPlannedExercises(nil, []int64{1})
	assert.Error(t, err)
	_, err = dao.ListFeedback(nil, 1, []int64{1})
	assert.Error(t, err)
	_, err = dao.ListRunnerSessions(nil, 1, []int64{1})
	assert.Error(t, err)
	_, err = dao.IsSessionTeamOwner(nil, 1, 1)
	assert.Error(t, err)
	_, err = dao.IsTeamMemberOfOwner(nil, 1, 1, 1)
	assert.Error(t, err)
}
