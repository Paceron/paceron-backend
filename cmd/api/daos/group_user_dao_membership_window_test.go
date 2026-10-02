package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// TestGroupUserDao_MembershipWindowSameDay es el repro de Gap 25: una membresía
// con date_start el MISMO día de la sesión pero con hora (ej. el corredor se sumó
// a las 10:35 a clase de las 8) se excluye del roster porque timestamptz se
// compara contra la fecha (DATE medianoche) de la sesión. La ventana de membresía
// es por calendario, no por instante: la sesión que ocurre ese día debe contar
// como dentro de la ventana.
func TestGroupUserDao_MembershipWindowSameDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	attendanceDao := NewAttendanceDao(db)
	guDao := NewGroupUserDao(db)

	owner := persistUser(db, "gu-window-sameday-owner@test.com", "54000001")
	team := testTeam(db, "equipo_gu_window_sameday", owner.ID)
	group := testGroup(db, "grupo_gu_window_sameday", team.ID)
	sessionID := testSessionInstance(db, "sesion-gu-window-sameday")

	sessionDate := time.Now().UTC().Truncate(24 * time.Hour)

	startedToday := persistUser(db, "gu-window-sameday-today@test.com", "54000002")
	require.NoError(t, guDao.Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: startedToday.ID, DateStart: sessionDate.Add(12 * time.Hour), // mediodía UTC del día de sesión: evita flake de timezone (review T1)
	}))

	startedYesterday := persistUser(db, "gu-window-sameday-yesterday@test.com", "54000003")
	require.NoError(t, guDao.Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: startedYesterday.ID, DateStart: time.Now().AddDate(0, 0, -1),
	}))

	startedTomorrow := persistUser(db, "gu-window-sameday-tomorrow@test.com", "54000004")
	require.NoError(t, guDao.Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: startedTomorrow.ID, DateStart: time.Now().AddDate(0, 0, 1),
	}))

	// Deja hoy sobre el roster: su membresía vence ESE día y sigue asistiendo.
	endsToday := persistUser(db, "gu-window-sameday-ends-today@test.com", "54000005")
	endsTodayEnd := sessionDate.Add(23*time.Hour + 59*time.Minute)
	require.NoError(t, guDao.Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: endsToday.ID, DateStart: time.Now().AddDate(0, 0, -30), DateEnd: &endsTodayEnd,
	}))

	endedYesterday := persistUser(db, "gu-window-sameday-ended-yesterday@test.com", "54000006")
	endedYesterdayEnd := sessionDate.Add(-1 * time.Second)
	require.NoError(t, guDao.Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: endedYesterday.ID, DateStart: time.Now().AddDate(0, 0, -30), DateEnd: &endedYesterdayEnd,
	}))

	t.Run("(a) roster incluye al que se sumo el mismo dia de la sesion", func(t *testing.T) {
		rows, aggregates, err := attendanceDao.FindGroupRosterWithAttendance(nil, group.ID, team.ID, sessionID, sessionDate)

		require.NoError(t, err)
		assert.Equal(t, int64(3), aggregates.RosterSize)
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.UserID)
		}
		assert.Equal(t, int64(3), int64(len(ids)))
		assert.ElementsMatch(t, []int64{startedToday.ID, startedYesterday.ID, endsToday.ID}, ids,
			"date_start::date <= sessionDate::date: el de HOY CON HORA entra al roster")
		assert.NotContains(t, ids, startedTomorrow.ID)
		assert.NotContains(t, ids, endedYesterday.ID)
	})

	t.Run("(b) IsActiveGroupMember true para el que se sumo el mismo dia", func(t *testing.T) {
		got, err := guDao.IsActiveGroupMember(nil, group.ID, startedToday.ID, sessionDate)

		require.NoError(t, err)
		assert.True(t, got, "date_start con hora del mismo dia no excluye (Gap 25)")
	})

	t.Run("(c) MissingGroupMembers no lista al que se sumo el mismo dia", func(t *testing.T) {
		missing, err := guDao.MissingGroupMembers(nil, group.ID, []int64{startedToday.ID, startedTomorrow.ID}, sessionDate)

		require.NoError(t, err)
		assert.Equal(t, []int64{startedTomorrow.ID}, missing)
	})
}
