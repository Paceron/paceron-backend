package services

// Leaderboard por team (gamificacion-core D7): service con mocks + TZ fija.

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/testutils"
)

type mockLeaderboardDAO struct {
	isTeamGroupMemberFn func(teamID, userID int64) (bool, error)
	isTeamOwnerFn       func(teamID, userID int64) (bool, error)
	aggregateFeedbackFn func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error)
	aggregateSessionsFn func(teamID int64, from, to *string) ([]dbs.LeaderboardSessionsAgg, error)
	aggregateAttendFn   func(teamID int64, from, to *string) ([]dbs.LeaderboardAttendanceAgg, error)

	groupMemberCalls int
	ownerCalls       int
	feedbackCalls    int
	sessionsCalls    int
	attendCalls      int
	usersCalls       int
}

func (m *mockLeaderboardDAO) IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error) {
	m.groupMemberCalls++
	if m.isTeamGroupMemberFn != nil {
		return m.isTeamGroupMemberFn(teamID, userID)
	}
	return false, nil
}

func (m *mockLeaderboardDAO) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	m.ownerCalls++
	if m.isTeamOwnerFn != nil {
		return m.isTeamOwnerFn(teamID, userID)
	}
	return false, nil
}

func (m *mockLeaderboardDAO) AggregateFeedback(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
	m.feedbackCalls++
	if m.aggregateFeedbackFn != nil {
		return m.aggregateFeedbackFn(teamID, from, to)
	}
	return nil, nil
}

func (m *mockLeaderboardDAO) AggregateRunnerSessions(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardSessionsAgg, error) {
	m.sessionsCalls++
	if m.aggregateSessionsFn != nil {
		return m.aggregateSessionsFn(teamID, from, to)
	}
	return nil, nil
}

func (m *mockLeaderboardDAO) AggregateAttendances(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardAttendanceAgg, error) {
	m.attendCalls++
	if m.aggregateAttendFn != nil {
		return m.aggregateAttendFn(teamID, from, to)
	}
	return nil, nil
}

func (m *mockLeaderboardDAO) UsersByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	m.usersCalls++
	out := make([]*dbs.User, 0, len(userIDs))
	for _, id := range userIDs {
		if id <= 9 {
			out = append(out, &dbs.User{ID: id, Name: fmt.Sprintf("user-%d", id)})
		}
	}
	return out, nil
}

func newLeaderboardDAOStub() *mockLeaderboardDAO {
	return &mockLeaderboardDAO{
		isTeamGroupMemberFn: func(int64, int64) (bool, error) { return true, nil },
	}
}

func TestLeaderboardService_Validacion_Query(t *testing.T) {
	svc := NewLeaderboardService(&mockLeaderboardDAO{})

	cases := []struct {
		name           string
		metric, period string
		from, to       *string
	}{
		{"metric fuera de whitelist", "hour", "week", nil, nil},
		{"period fuera de whitelist", "distance", "decade", nil, nil},
		{"custom sin from", "distance", "custom", nil, strPtr("2026-10-13")},
		{"custom sin to", "distance", "custom", strPtr("2026-10-01"), nil},
		{"custom from > to", "distance", "custom", strPtr("2026-10-13"), strPtr("2026-10-01")},
		{"custom formato invalido", "distance", "custom", strPtr("01/10/2026"), strPtr("2026-10-13")},
	}
	for _, tc := range cases {
		_, err := svc.GetLeaderboard(nil, 1, 2, tc.metric, tc.period, tc.from, tc.to)
		require.ErrorIs(t, err, ErrLeaderboardInvalid, tc.name)
	}

	// El query no llegó a tocar el DAO en ninguno de los 400.
	dao := &mockLeaderboardDAO{}
	svc = NewLeaderboardService(dao)
	_, err := svc.GetLeaderboard(nil, 1, 2, "hour", "week", nil, nil)
	require.ErrorIs(t, err, ErrLeaderboardInvalid)
	assert.Zero(t, dao.groupMemberCalls+dao.ownerCalls+dao.feedbackCalls)
}

func TestLeaderboardService_Guard(t *testing.T) {
	// Ajeno: ni miembro ni owner → 403 y agregación no ejecutada.
	dao := &mockLeaderboardDAO{}
	svc := NewLeaderboardService(dao)
	_, err := svc.GetLeaderboard(nil, 7, 2, "distance", "all", nil, nil)
	require.ErrorIs(t, err, ErrLeaderboardForbidden)
	assert.Equal(t, 1, dao.groupMemberCalls, "consulta miembro")
	assert.Equal(t, 1, dao.ownerCalls, "consulta owner como fallback")
	assert.Zero(t, dao.feedbackCalls, "la agregación no se ejecuta con 403")

	// Owner del team (sin grupo): pasa por el fallback y no consulta de nuevo.
	dao = &mockLeaderboardDAO{
		isTeamGroupMemberFn: func(int64, int64) (bool, error) { return false, nil },
		isTeamOwnerFn:       func(teamID, userID int64) (bool, error) { return userID == 9, nil },
	}
	svc = NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 9, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, dao.ownerCalls)

	// Error de DAO en el guard: 500 genérico sin leak.
	dao = &mockLeaderboardDAO{
		isTeamGroupMemberFn: func(int64, int64) (bool, error) { return false, errors.New("conexión rechazada") },
	}
	svc = NewLeaderboardService(dao)
	_, err = svc.GetLeaderboard(nil, 7, 2, "distance", "all", nil, nil)
	require.EqualError(t, err, "error al consultar el leaderboard")
}

func TestLeaderboardService_MetricaElegidaYRango(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13) // martes

	cases := []struct {
		metric string
		wantFn func(d *mockLeaderboardDAO) int
	}{
		{gamification.LeaderboardMetricDistance, func(d *mockLeaderboardDAO) int { return d.feedbackCalls }},
		{gamification.LeaderboardMetricDuration, func(d *mockLeaderboardDAO) int { return d.feedbackCalls }},
		{gamification.LeaderboardMetricPace, func(d *mockLeaderboardDAO) int { return d.feedbackCalls }},
		{gamification.LeaderboardMetricPeakSpeed, func(d *mockLeaderboardDAO) int { return d.feedbackCalls }},
		{gamification.LeaderboardMetricSessions, func(d *mockLeaderboardDAO) int { return d.sessionsCalls }},
		{gamification.LeaderboardMetricAttendance, func(d *mockLeaderboardDAO) int { return d.attendCalls }},
	}
	for _, tc := range cases {
		dao := newLeaderboardDAOStub()
		svc := NewLeaderboardService(dao)
		_, err := svc.GetLeaderboard(nil, 1, 2, tc.metric, "all", nil, nil)
		require.NoError(t, err, tc.metric)
		assert.Equal(t, 1, tc.wantFn(dao), tc.metric)
		// La agregación equivocada no corrió.
		if tc.metric == gamification.LeaderboardMetricDistance {
			assert.Zero(t, dao.sessionsCalls+dao.attendCalls)
		}
	}

	// week: lunes 10-12 .. domingo 10-18 derivados de hoy (martes 10-13).
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		require.NotNil(t, from)
		require.NotNil(t, to)
		assert.Equal(t, "2026-10-12", *from)
		assert.Equal(t, "2026-10-18", *to)
		return nil, nil
	}
	svc := NewLeaderboardService(dao)
	_, err := svc.GetLeaderboard(nil, 1, 2, "distance", "week", nil, nil)
	require.NoError(t, err)

	// month: 10-01 .. 10-31.
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		assert.Equal(t, "2026-10-01", *from)
		assert.Equal(t, "2026-10-31", *to)
		return nil, nil
	}
	svc = NewLeaderboardService(dao)
	_, err = svc.GetLeaderboard(nil, 1, 2, "distance", "month", nil, nil)
	require.NoError(t, err)

	// year: 01-01 .. 12-31.
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		assert.Equal(t, "2026-01-01", *from)
		assert.Equal(t, "2026-12-31", *to)
		return nil, nil
	}
	svc = NewLeaderboardService(dao)
	_, err = svc.GetLeaderboard(nil, 1, 2, "distance", "year", nil, nil)
	require.NoError(t, err)

	// custom: pasa tal cual.
	fromStr, toStr := "2026-03-01", "2026-03-31"
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		assert.Equal(t, fromStr, *from)
		assert.Equal(t, toStr, *to)
		return nil, nil
	}
	svc = NewLeaderboardService(dao)
	_, err = svc.GetLeaderboard(nil, 1, 2, "distance", "custom", &fromStr, &toStr)
	require.NoError(t, err)
}

func feedbackAggRow(userID int64, sumDistance float64, distanceCount int64, lastDate string) dbs.LeaderboardFeedbackAgg {
	row := dbs.LeaderboardFeedbackAgg{UserID: userID, SumDistance: &sumDistance, DistanceCount: distanceCount}
	if lastDate != "" {
		d := datePtr(lastDate)
		row.DistanceLastDate = d
	}
	return row
}

func datePtr(s string) *time.Time {
	tm, _ := time.Parse("2006-01-02", s)
	return &tm
}

func TestLeaderboardService_Distance_OrdenDesempateYRanks(t *testing.T) {
	// A y B empatan en 10km (rank 1 compartido), C tiene 8km (rank 3).
	// Desempate: A lo hizo en 2 sesiones, B en 5 → A arriba.
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		return []dbs.LeaderboardFeedbackAgg{
			feedbackAggRow(5, 10000, 5, "2026-10-05"),
			feedbackAggRow(3, 10000, 2, "2026-10-06"),
			feedbackAggRow(8, 8000, 1, "2026-10-07"),
		}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 3, 2, "distance", "all", nil, nil)
	require.NoError(t, err)

	require.Len(t, resp.Rows, 3)
	assert.Equal(t, int64(3), resp.Rows[0].UserID)
	assert.Equal(t, 1, resp.Rows[0].Rank)
	assert.Equal(t, 10000.0, resp.Rows[0].Value)
	assert.Equal(t, int64(5), resp.Rows[1].UserID)
	assert.Equal(t, 1, resp.Rows[1].Rank, "empate comparte rank")
	assert.Equal(t, int64(8), resp.Rows[2].UserID)
	assert.Equal(t, 3, resp.Rows[2].Rank, "siguiente posición saltea N")

	// Viewer es el corredor 3: rank 1.
	require.NotNil(t, resp.ViewerRank)
	assert.Equal(t, int64(3), resp.ViewerRank.UserID)
	assert.Equal(t, 1, resp.ViewerRank.Rank)
	// En all el record SÍ viene: empate A/B (10k) → achieved_at más reciente.
	require.NotNil(t, resp.Record)
	assert.Equal(t, int64(3), resp.Record.UserID)
	assert.Equal(t, "2026-10-06", resp.Record.AchievedAt)
}

func TestLeaderboardService_Pace_MenorEsMejor(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		mk := func(id int64, dist float64, dur int64, count int64) dbs.LeaderboardFeedbackAgg {
			return dbs.LeaderboardFeedbackAgg{
				UserID: id, SumDistance: &dist, SumDurationMs: &dur, DistanceCount: count,
				DurationCount: count, PaceCount: count,
			}
		}
		// pace: 300000/1000=300 vs 600000/3000=200 → el de 200 va primero.
		return []dbs.LeaderboardFeedbackAgg{mk(4, 1000, 300000, 1), mk(6, 3000, 600000, 1)}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 4, 2, "pace", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 2)
	assert.Equal(t, int64(6), resp.Rows[0].UserID)
	assert.InDelta(t, 200.0, resp.Rows[0].Value, 0.0001)
	assert.Equal(t, int64(4), resp.Rows[1].UserID)

	// Record de pace = el MEJOR (más bajo).
	require.NotNil(t, resp.Record)
	assert.Equal(t, int64(6), resp.Record.UserID)
}

func TestLeaderboardService_Sessions_DesempateEmpezoAntes(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateSessionsFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardSessionsAgg, error) {
		return []dbs.LeaderboardSessionsAgg{
			{UserID: 2, SessionsCount: 3, LastStart: timePtrDateHour("2026-10-08T10:00:00Z")},
			{UserID: 1, SessionsCount: 3, LastStart: timePtrDateHour("2026-10-07T10:00:00Z")},
		}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 1, 2, "sessions_count", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 2)
	assert.Equal(t, int64(1), resp.Rows[0].UserID, "empezó antes su última sesión")
	assert.Equal(t, int64(2), resp.Rows[1].UserID)
	assert.Equal(t, 1, resp.Rows[0].Rank)
	assert.Equal(t, 1, resp.Rows[1].Rank)
}

func TestLeaderboardService_Attendance_DesempateRegistroPrimero(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateAttendFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardAttendanceAgg, error) {
		return []dbs.LeaderboardAttendanceAgg{
			{UserID: 2, AttendanceCount: 4, LastRegisteredAt: timePtrDateHour("2026-10-09T12:00:00Z")},
			{UserID: 1, AttendanceCount: 4, LastRegisteredAt: timePtrDateHour("2026-10-08T12:00:00Z")},
		}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 1, 2, "attendance_count", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 2)
	assert.Equal(t, int64(1), resp.Rows[0].UserID, "registró antes su última asistencia")
}

func TestLeaderboardService_Top5YViewerFueraDelPodio(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		rows := make([]dbs.LeaderboardFeedbackAgg, 0, 9)
		for i := 1; i <= 9; i++ {
			rows = append(rows, feedbackAggRow(int64(i), float64(1000*i), 1, "2026-10-05"))
		}
		return rows, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 4, 2, "distance", "all", nil, nil)
	require.NoError(t, err)

	require.Len(t, resp.Rows, 5, "top 5")
	assert.Equal(t, int64(9), resp.Rows[0].UserID)
	assert.Equal(t, int64(5), resp.Rows[4].UserID)
	require.NotNil(t, resp.ViewerRank)
	assert.Equal(t, int64(4), resp.ViewerRank.UserID)
	assert.Equal(t, 6, resp.ViewerRank.Rank, "user 4 queda 6vo del ranking completo")

	// Viewer en posición 9.
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		rows := make([]dbs.LeaderboardFeedbackAgg, 0, 9)
		for i := 1; i <= 9; i++ {
			rows = append(rows, feedbackAggRow(int64(i), float64(1000*i), 1, "2026-10-05"))
		}
		return rows, nil
	}
	svc = NewLeaderboardService(dao)
	resp, err = svc.GetLeaderboard(nil, 2, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp.ViewerRank)
	assert.Equal(t, 8, resp.ViewerRank.Rank)
	assert.InDelta(t, 2000.0, resp.ViewerRank.Value, 0.0001)
}

func TestLeaderboardService_Record_AllConEmpateMasReciente(t *testing.T) {
	// A y B empatan el máximo histórico (10km); el achieved_at de B es más
	// reciente → record apunta a B.
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		return []dbs.LeaderboardFeedbackAgg{
			feedbackAggRow(1, 10000, 2, "2026-10-02"),
			feedbackAggRow(2, 10000, 3, "2026-10-09"),
			feedbackAggRow(3, 8000, 1, "2026-10-08"),
		}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 3, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp.Record)
	assert.Equal(t, int64(2), resp.Record.UserID)
	assert.InDelta(t, 10000.0, resp.Record.Value, 0.0001)
	assert.Equal(t, "2026-10-09", resp.Record.AchievedAt)
	assert.NotEmpty(t, resp.Record.Name)
}

func TestLeaderboardService_Record_PeakUsaFechaDelMax(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		maxPeak := 22.5
		oldDate := datePtr("2026-10-01")
		recentDate := datePtr("2026-10-09")
		return []dbs.LeaderboardFeedbackAgg{
			// El max peak está en la fila vieja: achieved_at = 10-01.
			{UserID: 1, MaxPeak: &maxPeak, PeakCount: 2, PeakArgmaxDate: oldDate, DistanceLastDate: recentDate, DistanceCount: 2, SumDistance: &maxPeak},
			feedbackAggRow(2, 5000, 1, "2026-10-08"),
		}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 1, 2, "peak_speed", "all", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp.Record)
	assert.Equal(t, int64(1), resp.Record.UserID)
	assert.InDelta(t, 22.5, resp.Record.Value, 0.0001)
	assert.Equal(t, "2026-10-01", resp.Record.AchievedAt)
}

func TestLeaderboardService_VacioYOwnerSinFila(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.isTeamGroupMemberFn = func(int64, int64) (bool, error) { return false, nil }
	dao.isTeamOwnerFn = func(teamID, userID int64) (bool, error) { return userID == 9, nil }
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 9, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Rows)
	assert.Nil(t, resp.ViewerRank, "el owner no puntúa, no tiene viewer_rank")
	assert.Nil(t, resp.Record)
}

func TestLeaderboardService_AgregacionError500Generico(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		return nil, errors.New("SQLSTATE detalle interno")
	}
	svc := NewLeaderboardService(dao)
	_, err := svc.GetLeaderboard(nil, 1, 2, "distance", "all", nil, nil)
	require.EqualError(t, err, "error al consultar el leaderboard")
	assert.NotContains(t, err.Error(), "SQLSTATE")
}

func TestLeaderboardService_UsuarioInexistenteEnBatchNoRompe(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		return []dbs.LeaderboardFeedbackAgg{feedbackAggRow(42, 5000, 1, "2026-10-05")}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 42, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 1)
	assert.Equal(t, "", resp.Rows[0].Name, "user huérfano: nombre vacío, no crash")
}

type leaderboardScene struct {
	owner      *dbs.User
	team       *dbs.Team
	firstRuner *dbs.User
}

// seedLeaderboardScene arma owner + team + grupo + n corredores con feedback
// de distancia dentro de la semana del "hoy" fijado (10-13 → week 10-12..18).
func seedLeaderboardScene(t *testing.T, db *gorm.DB, tag string, runnerCount int) *leaderboardScene {
	t.Helper()
	owner := &dbs.User{Name: "Owner", Surname: tag, Email: "lb-owner-" + tag + "@test.com", DNI: "6300" + tag[len(tag)-3:] + tag[:1], BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "LB scene " + tag, MaxMembers: 30, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "LB group " + tag, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	var first *dbs.User
	for i := 1; i <= runnerCount; i++ {
		runner := &dbs.User{Name: "Runner", Surname: fmt.Sprintf("%s-%d", tag, i), Email: fmt.Sprintf("lb-runner-%s-%d@test.com", tag, i), DNI: fmt.Sprintf("631%s%03d", tag[len(tag)-2:], i), BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
		require.NoError(t, db.Create(runner).Error)
		require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}).Error)
		feedback := &dbs.WorkoutFeedback{
			TeamID: &team.ID, AssignedSessionID: int64(9000 + i), AssignedExerciseID: 1,
			AthleteUserID: runner.ID, FeedbackOwnerUserID: owner.ID, ReportSource: "corredor",
			SessionDate: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), DistanceMeters: &[]float64{float64(1000 * i)}[0],
		}
		require.NoError(t, feedback.MediaURLs.Set([]string{"https://media.foo/a.jpg"}))
		require.NoError(t, db.Create(feedback).Error)
		if first == nil {
			first = runner
		}
	}
	return &leaderboardScene{owner: owner, team: team, firstRuner: first}
}

func TestLeaderboardService_CountingPin_SinNPlus1(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)

	fx := seedLeaderboardScene(t, db, "cnt6", 6)
	counter := 0
	counting := txCountingDB(t, db, &counter)
	svc := NewLeaderboardService(daos.NewLeaderboardDao(counting))

	resp, err := svc.GetLeaderboard(nil, fx.firstRuner.ID, fx.team.ID, "distance", "week", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, counter, "3 fijo: guard miembro + 1 agregación + batch users")
	assert.Len(t, resp.Rows, 5)

	// 12 corredores en vez de 6: el mismo read path (sin término lineal).
	big := seedLeaderboardScene(t, db, "cnt12", 12)
	counter = 0
	_, err = svc.GetLeaderboard(nil, big.firstRuner.ID, big.team.ID, "distance", "week", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, counter, "el batch de nombres no escala con corredores")
}

func timePtrDateHour(s string) *time.Time {
	tm, _ := time.Parse(time.RFC3339, s)
	return &tm
}

// Ramas del mapeo feedback→entries: duration y peak con datos, y los skips
// (count 0 / sums en 0) que dejan al usuario fuera del board.
func TestLeaderboardService_FeedbackAgg_Ramas(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)

	// duration: suma de duration_ms.
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		dur := int64(1500000)
		return []dbs.LeaderboardFeedbackAgg{{UserID: 4, SumDurationMs: &dur, DurationCount: 2}}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 4, 2, "duration", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 1)
	assert.InDelta(t, 1500000.0, resp.Rows[0].Value, 0.0001)

	// peak_speed: max directo.
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		peak := 21.5
		return []dbs.LeaderboardFeedbackAgg{{UserID: 4, MaxPeak: &peak, PeakCount: 3}}, nil
	}
	svc = NewLeaderboardService(dao)
	resp, err = svc.GetLeaderboard(nil, 4, 2, "peak_speed", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 1)
	assert.InDelta(t, 21.5, resp.Rows[0].Value, 0.0001)

	// pace con sums en 0 (duration sin distance): el usuario no entra.
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		dur := int64(600000)
		zero := 0.0
		return []dbs.LeaderboardFeedbackAgg{{UserID: 4, SumDurationMs: &dur, SumDistance: &zero, PaceCount: 1}}, nil
	}
	svc = NewLeaderboardService(dao)
	resp, err = svc.GetLeaderboard(nil, 4, 2, "pace", "all", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Rows)

	// distance sin feedbacks con distance > 0: fuera del board.
	dao = newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		return []dbs.LeaderboardFeedbackAgg{{UserID: 4, DistanceCount: 0}}, nil
	}
	svc = NewLeaderboardService(dao)
	resp, err = svc.GetLeaderboard(nil, 4, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Rows)
}

// Viewer sin actividad del período: viewer_rank null con filas presentes.
func TestLeaderboardService_ViewerSinActividad(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateFeedbackFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
		return []dbs.LeaderboardFeedbackAgg{feedbackAggRow(3, 5000, 1, "2026-10-05")}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 77, 2, "distance", "all", nil, nil)
	require.NoError(t, err)
	require.Len(t, resp.Rows, 1)
	assert.Nil(t, resp.ViewerRank)
}

// Record con achieved_at nil → string vacío, sin crash.
func TestLeaderboardService_Record_SinFecha(t *testing.T) {
	dao := newLeaderboardDAOStub()
	dao.aggregateSessionsFn = func(teamID int64, from, to *string) ([]dbs.LeaderboardSessionsAgg, error) {
		return []dbs.LeaderboardSessionsAgg{{UserID: 3, SessionsCount: 2}}, nil
	}
	svc := NewLeaderboardService(dao)
	resp, err := svc.GetLeaderboard(nil, 3, 2, "sessions_count", "all", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp.Record)
	assert.Equal(t, "", resp.Record.AchievedAt)
}

func TestLaterAchieved(t *testing.T) {
	early := datePtr("2026-10-01")
	late := datePtr("2026-10-09")
	assert.False(t, laterAchieved(nil, early), "nil nunca gana")
	assert.True(t, laterAchieved(late, nil), "la otra nil siempre pierde")
	assert.True(t, laterAchieved(late, early))
	assert.False(t, laterAchieved(early, late))
}
