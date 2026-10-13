package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"

	"simple-arq-golang/cmd/api/testutils"
)

// Agregaciones del leaderboard (D7) contra Postgres real. Escenario base:
// owner + 2 corredores en un grupo del team; fixtures fuera del team y del
// período para pinear los filtros.

type leaderboardFixture struct {
	owner   *dbs.User
	runnerA *dbs.User
	runnerB *dbs.User
	team    *dbs.Team
	group   *dbs.Group
}

func newLeaderboardFixture(t *testing.T, db *gorm.DB, suffix string) *leaderboardFixture {
	t.Helper()
	owner := persistUser(db, "lb-owner-"+suffix+"@test.com", "9500"+suffix+"1")
	runnerA := persistUser(db, "lb-runner-a-"+suffix+"@test.com", "9500"+suffix+"2")
	runnerB := persistUser(db, "lb-runner-b-"+suffix+"@test.com", "9500"+suffix+"3")
	team := testTeam(db, "equipo_lb_"+suffix, owner.ID)
	group := &dbs.Group{Name: "grupo_lb_" + suffix, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	for _, u := range []*dbs.User{runnerA, runnerB} {
		require.NoError(t, db.Create(&dbs.GroupUser{
			GroupID: group.ID, UserID: u.ID,
			DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}).Error)
	}
	return &leaderboardFixture{owner: owner, runnerA: runnerA, runnerB: runnerB, team: team, group: group}
}

func dateUTC(s string) time.Time {
	tm, _ := time.Parse("2006-01-02", s)
	return tm
}

func TestLeaderboardDAO_AggregateFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewLeaderboardDao(db)
	fx := newLeaderboardFixture(t, db, "01")

	// Runner A: 2 feedbacks con distance/duration (3km + 2km, 20min + 15min),
	// uno con peak. Runner B: 1 feedback con distance y peak.
	a1 := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 101, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(a1).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-05"), "distance_meters": 3000.0, "duration_ms": int64(1200000), "peak_speed_kph": 12.5,
	}).Error)
	a2 := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 102, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(a2).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-08"), "distance_meters": 2000.0, "duration_ms": int64(900000), "peak_speed_kph": 18.0,
	}).Error)
	b1 := testFeedback(t, db, fx.runnerB.ID, fx.owner.ID, 103, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(b1).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-06"), "distance_meters": 5000.0, "peak_speed_kph": 20.0,
	}).Error)
	// El owner también entrena: NO genera fila propia.
	own := testFeedback(t, db, fx.owner.ID, fx.owner.ID, 104, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(own).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-07"), "distance_meters": 42000.0,
	}).Error)
	// Feedback fuera del período y de otro team: excluidos.
	out := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 105, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(out).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-09-01"), "distance_meters": 9000.0,
	}).Error)
	otherTeam := testTeam(db, "equipo_lb_otro", fx.owner.ID)
	other := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 106, 1, &otherTeam.ID, 0)
	require.NoError(t, db.Model(other).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-09"), "distance_meters": 9000.0,
	}).Error)
	// Soft-deleteado: excluido.
	deleted := testFeedback(t, db, fx.runnerB.ID, fx.owner.ID, 107, 1, &fx.team.ID, 0)
	deletedAt := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Model(deleted).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-10"), "distance_meters": 7000.0, "deleted_at": deletedAt,
	}).Error)

	rows, err := dao.AggregateFeedback(nil, fx.team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byUser := map[int64]dbs.LeaderboardFeedbackAgg{}
	for _, r := range rows {
		byUser[r.UserID] = r
	}
	a := byUser[fx.runnerA.ID]
	require.NotNil(t, a.SumDistance)
	// Sin bordes, el "fuera del período" (10-09-01, 9km) entra igual: el
	// filtro de período solo aplica con rango (se prueba abajo).
	assert.InDelta(t, 14000.0, *a.SumDistance, 0.0001)
	require.NotNil(t, a.SumDurationMs)
	assert.Equal(t, int64(2100000), *a.SumDurationMs)
	assert.Equal(t, int64(3), a.DistanceCount)
	assert.Equal(t, int64(2), a.DurationCount)
	assert.Equal(t, int64(2), a.PaceCount)
	assert.Equal(t, int64(2), a.PeakCount)
	require.NotNil(t, a.MaxPeak)
	assert.InDelta(t, 18.0, *a.MaxPeak, 0.0001)
	require.NotNil(t, a.DistanceLastDate)
	assert.Equal(t, "2026-10-08", a.DistanceLastDate.UTC().Format("2006-01-02"))
	require.NotNil(t, a.PaceLastDate)
	assert.Equal(t, "2026-10-08", a.PaceLastDate.UTC().Format("2006-01-02"))

	b := byUser[fx.runnerB.ID]
	require.NotNil(t, b.SumDistance)
	assert.InDelta(t, 5000.0, *b.SumDistance, 0.0001)
	assert.Nil(t, b.SumDurationMs, "B nunca reportó duration")
	assert.Equal(t, int64(0), b.DurationCount)
	assert.Equal(t, int64(0), b.PaceCount)
	assert.Equal(t, int64(1), b.PeakCount)
	require.NotNil(t, b.MaxPeak)
	assert.InDelta(t, 20.0, *b.MaxPeak, 0.0001)

	_, ownerHasRow := byUser[fx.owner.ID]
	assert.False(t, ownerHasRow, "el owner no genera fila propia")

	// Período 10-06..10-07: solo b1 de B; el del 09-01 y el del 10-08 quedan fuera.
	from, to := "2026-10-06", "2026-10-07"
	rows, err = dao.AggregateFeedback(nil, fx.team.ID, &from, &to)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, fx.runnerB.ID, rows[0].UserID)

	// Rango que cubre todo octubre: el 9000 del 09-01 queda fuera.
	from, to = "2026-10-01", "2026-10-31"
	rows, err = dao.AggregateFeedback(nil, fx.team.ID, &from, &to)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	byUser = map[int64]dbs.LeaderboardFeedbackAgg{}
	for _, r := range rows {
		byUser[r.UserID] = r
	}
	require.NotNil(t, byUser[fx.runnerA.ID].SumDistance)
	assert.InDelta(t, 5000.0, *byUser[fx.runnerA.ID].SumDistance, 0.0001)
}

func TestLeaderboardDAO_AggregateFeedback_PeakArgmaxDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewLeaderboardDao(db)
	fx := newLeaderboardFixture(t, db, "02")

	// El max peak está en el feedback MÁS VIEJO: argmax devuelve esa fecha.
	old := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 201, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(old).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-01"), "peak_speed_kph": 25.0,
	}).Error)
	recent := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 202, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(recent).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-09"), "peak_speed_kph": 10.0,
	}).Error)

	rows, err := dao.AggregateFeedback(nil, fx.team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].PeakArgmaxDate)
	assert.Equal(t, "2026-10-01", rows[0].PeakArgmaxDate.UTC().Format("2006-01-02"))
}

func TestLeaderboardDAO_AggregateRunnerSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewLeaderboardDao(db)
	fx := newLeaderboardFixture(t, db, "03")

	mkSession := func(t *testing.T, groupID, athleteID int64, startDate time.Time, status string) {
		t.Helper()
		ins := &dbs.SessionInstance{Name: "instancia_lb"}
		require.NoError(t, db.Create(ins).Error)
		require.NoError(t, db.Create(&dbs.GroupCalendarDay{
			GroupID: groupID, Date: startDate, Kind: "training", SessionInstanceID: &ins.ID,
		}).Error)
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: ins.ID, AthleteUserID: athleteID, Status: status, StartDate: startDate,
		}).Error)
	}

	// Runner A: 2 finished (la última el 10-07 22:00 Cordoba = 10-08 01:00 UTC).
	mkSession(t, fx.group.ID, fx.runnerA.ID, time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC), runnersession.RunnerSessionStatusFinished)
	mkSession(t, fx.group.ID, fx.runnerA.ID, time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC), runnersession.RunnerSessionStatusFinished)
	// wip: no cuenta.
	mkSession(t, fx.group.ID, fx.runnerA.ID, time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC), "wip")
	// Runner B: 1 finished.
	mkSession(t, fx.group.ID, fx.runnerB.ID, time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC), runnersession.RunnerSessionStatusFinished)
	// Owner: finished pero excluido.
	mkSession(t, fx.group.ID, fx.owner.ID, time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC), runnersession.RunnerSessionStatusFinished)

	// Otro team con sesión del runner A: excluida.
	otherTeam := testTeam(db, "equipo_lb_otro_ses", fx.owner.ID)
	otherGroup := &dbs.Group{Name: "grupo_lb_otro_ses", TeamID: otherTeam.ID}
	require.NoError(t, db.Create(otherGroup).Error)
	mkSession(t, otherGroup.ID, fx.runnerA.ID, time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC), runnersession.RunnerSessionStatusFinished)

	rows, err := dao.AggregateRunnerSessions(nil, fx.team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byUser := map[int64]dbs.LeaderboardSessionsAgg{}
	for _, r := range rows {
		byUser[r.UserID] = r
	}
	a := byUser[fx.runnerA.ID]
	assert.Equal(t, int64(2), a.SessionsCount)
	require.NotNil(t, a.LastStart)
	// start_date 2026-10-08 01:00 UTC = 2026-10-07 22:00 Cordoba.
	assert.Equal(t, "2026-10-07", a.LastStart.UTC().Format("2006-01-02"))
	b := byUser[fx.runnerB.ID]
	assert.Equal(t, int64(1), b.SessionsCount)
	_, ownerRow := byUser[fx.owner.ID]
	assert.False(t, ownerRow)

	// Período con borde en Cordoba: 10-05..10-07 incluye la sesión que arrancó
	// 22:00 de Cordoba el 07 (01:00 UTC del 08) pero no la del 05? Sí incluye:
	// arrancó 10-05 07:00 Cordoba.
	from, to := "2026-10-06", "2026-10-07"
	rows, err = dao.AggregateRunnerSessions(nil, fx.team.ID, &from, &to)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.Equal(t, int64(1), r.SessionsCount)
	}
}

func TestLeaderboardDAO_AggregateAttendances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewLeaderboardDao(db)
	fx := newLeaderboardFixture(t, db, "04")

	mkAttendance := func(t *testing.T, teamID, trainingSessionID, userID int64, createdAt time.Time) {
		t.Helper()
		att := &dbs.Attendance{
			TeamID: teamID, TrainingSessionID: trainingSessionID, UserID: userID, Source: "qr",
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		require.NoError(t, db.Create(att).Error)
	}

	// Runner A: 2 asistencias; la última registrada 2026-10-07 23:30 Cordoba
	// (2026-10-08 02:30 UTC). Runner B: 1. Owner: 1 pero excluido.
	mkAttendance(t, fx.team.ID, 301, fx.runnerA.ID, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC))
	mkAttendance(t, fx.team.ID, 302, fx.runnerA.ID, time.Date(2026, 10, 8, 2, 30, 0, 0, time.UTC))
	mkAttendance(t, fx.team.ID, 303, fx.runnerB.ID, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	mkAttendance(t, fx.team.ID, 304, fx.owner.ID, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))

	otherTeam := testTeam(db, "equipo_lb_otro_att", fx.owner.ID)
	mkAttendance(t, otherTeam.ID, 305, fx.runnerA.ID, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))

	rows, err := dao.AggregateAttendances(nil, fx.team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byUser := map[int64]dbs.LeaderboardAttendanceAgg{}
	for _, r := range rows {
		byUser[r.UserID] = r
	}
	a := byUser[fx.runnerA.ID]
	assert.Equal(t, int64(2), a.AttendanceCount)
	require.NotNil(t, a.LastRegisteredAt)
	assert.Equal(t, "2026-10-07", a.LastRegisteredAt.UTC().Format("2006-01-02"))
	b := byUser[fx.runnerB.ID]
	assert.Equal(t, int64(1), b.AttendanceCount)
	_, ownerRow := byUser[fx.owner.ID]
	assert.False(t, ownerRow)

	// Período 10-06..10-06: solo la asistencia del 6 de cada uno.
	from, to := "2026-10-06", "2026-10-06"
	rows, err = dao.AggregateAttendances(nil, fx.team.ID, &from, &to)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, fx.runnerB.ID, rows[0].UserID)
}

func TestLeaderboardDAO_IsTeamGroupMember(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewLeaderboardDao(db)
	fx := newLeaderboardFixture(t, db, "05")

	ok, err := dao.IsTeamGroupMember(nil, fx.team.ID, fx.runnerA.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	// Owner sin grupo: no es miembro de grupo (pero sí owner).
	ok, err = dao.IsTeamOwner(nil, fx.team.ID, fx.owner.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = dao.IsTeamGroupMember(nil, fx.team.ID, fx.owner.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	// Membresía expirada y baja lógica: fuera.
	expiredEnd := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	deletedAt := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	expired := persistUser(db, "lb-expired-05@test.com", "95000509")
	require.NoError(t, db.Create(&dbs.GroupUser{
		GroupID: fx.group.ID, UserID: expired.ID,
		DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), DateEnd: &expiredEnd,
	}).Error)
	removed := persistUser(db, "lb-removed-05@test.com", "95000510")
	require.NoError(t, db.Create(&dbs.GroupUser{
		GroupID: fx.group.ID, UserID: removed.ID,
		DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), DeletedAt: &deletedAt,
	}).Error)
	ok, err = dao.IsTeamGroupMember(nil, fx.team.ID, expired.ID)
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = dao.IsTeamGroupMember(nil, fx.team.ID, removed.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	// Group del team soft-deleteado: no habilita.
	stranger := persistUser(db, "lb-stranger-05@test.com", "95000511")
	deadGroup := &dbs.Group{Name: "grupo_lb_muerto", TeamID: fx.team.ID, DeletedAt: &deletedAt}
	require.NoError(t, db.Create(deadGroup).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{
		GroupID: deadGroup.ID, UserID: stranger.ID,
		DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}).Error)
	ok, err = dao.IsTeamGroupMember(nil, fx.team.ID, stranger.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	// Team inexistente.
	ok, err = dao.IsTeamGroupMember(nil, 999999, fx.runnerA.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestLeaderboardDAO_UsersByIDs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewLeaderboardDao(db)
	u1 := persistUser(db, "lb-users-1@test.com", "95000601")
	u2 := persistUser(db, "lb-users-2@test.com", "95000602")

	users, err := dao.UsersByIDs(nil, []int64{u1.ID, u2.ID})
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, u1.ID, users[0].ID)

	empty, err := dao.UsersByIDs(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, empty)
}

// Ramas de error de las agregaciones con FailingDB (D12): 500 → error crudo
// del DAO que el service envuelve.
func TestLeaderboardDAO_Agregaciones_FailingDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	dao := NewLeaderboardDao(failing)

	_, err := dao.AggregateFeedback(nil, 1, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error aggregating leaderboard feedback")

	_, err = dao.AggregateRunnerSessions(nil, 1, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error aggregating leaderboard sessions")

	_, err = dao.AggregateAttendances(nil, 1, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error aggregating leaderboard attendances")

	_, err = dao.IsTeamGroupMember(nil, 1, 2)
	require.Error(t, err)
	_, err = dao.IsTeamOwner(nil, 1, 2)
	require.Error(t, err)
}
