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

// TestDateColumnScanProbe documenta el round-trip de la columna date de
// group_calendar_days: el driver la devuelve como medianoche UTC (el DSN fija
// TimeZone=UTC). Las claves de día de gamificación dependen de esto; si el
// driver cambia de comportamiento, explota acá.
func TestDateColumnScanProbe(t *testing.T) {
	db := testutils.SetupTestDB(t)
	group := setupCalendarGroup(t, db, "scanprobe")
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: group.ID,
		Date:    time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		Kind:    "training",
	}).Error)

	var day dbs.GroupCalendarDay
	require.NoError(t, db.Where("group_id = ?", group.ID).First(&day).Error)
	require.Equal(t, time.UTC, day.Date.Location())
	require.Equal(t, "2026-10-05", day.Date.UTC().Format("2006-01-02"))
}

// setupGamificationGroupWithRunner arma runner + equipo + grupo con
// membresía activa. El runner es owner del team a propósito: no importa para
// las queries de gamificación, así que no hace falta un entrenador aparte.
func setupGamificationGroupWithRunner(t *testing.T, db *gorm.DB, suffix string) (*dbs.User, *dbs.Group) {
	t.Helper()
	runner := persistUser(db, "gam-runner-"+suffix+"@test.com", "91000"+suffix)
	team := testTeam(db, "equipo_gam_"+suffix, runner.ID)
	group := &dbs.Group{Name: "grupo_gam_" + suffix, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}).Error)
	return runner, group
}

func TestGamificationDAO_FindActiveGroupMemberships(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, group := setupGamificationGroupWithRunner(t, db, "01")

	memberships, err := dao.FindActiveGroupMemberships(nil, runner.ID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Equal(t, group.ID, memberships[0].GroupID)

	// Usuario sin grupos: vacío.
	other := persistUser(db, "gam-nomembers-01@test.com", "91001901")
	memberships, err = dao.FindActiveGroupMemberships(nil, other.ID)
	require.NoError(t, err)
	assert.Empty(t, memberships)
}

func TestGamificationDAO_FindActiveGroupMemberships_ExcluyeBajasYExpiradas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, group := setupGamificationGroupWithRunner(t, db, "02")

	expiredEnd := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	deletedAt := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), DeletedAt: &deletedAt}).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC), DateEnd: &expiredEnd}).Error)

	memberships, err := dao.FindActiveGroupMemberships(nil, runner.ID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Nil(t, memberships[0].DateEnd)
	assert.Nil(t, memberships[0].DeletedAt)
}

func TestGamificationDAO_FindTrainingDaysUntil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, group := setupGamificationGroupWithRunner(t, db, "03")
	team2 := testTeam(db, "equipo_otro_gam", runner.ID)
	otherGroup := &dbs.Group{Name: "grupo_otro_gam", TeamID: team2.ID}
	require.NoError(t, db.Create(otherGroup).Error)

	ins := &dbs.SessionInstance{Name: "instancia"}
	require.NoError(t, db.Create(ins).Error)
	insID := ins.ID
	seed := []dbs.GroupCalendarDay{
		{GroupID: group.ID, Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &insID},
		{GroupID: group.ID, Date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Kind: "training"},
		{GroupID: group.ID, Date: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), Kind: "rest"},
		{GroupID: group.ID, Date: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), Kind: "cancelled"},
		{GroupID: group.ID, Date: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &insID},
		{GroupID: otherGroup.ID, Date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Kind: "training"},
	}
	for i := range seed {
		require.NoError(t, db.Create(&seed[i]).Error)
	}

	until := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC) // medianoche de ayer en Cordoba
	days, err := dao.FindTrainingDaysUntil(nil, []int64{group.ID}, until)
	require.NoError(t, err)
	require.Len(t, days, 2)
	assert.Equal(t, "2026-10-01", days[0].Date.UTC().Format("2006-01-02"))
	assert.Equal(t, "2026-10-02", days[1].Date.UTC().Format("2006-01-02"))

	// Sin grupos: vacío, evita IN () inválido.
	days, err = dao.FindTrainingDaysUntil(nil, []int64{}, until)
	require.NoError(t, err)
	assert.Empty(t, days)
}

func TestGamificationDAO_FindFinishedRunnerSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, _ := setupGamificationGroupWithRunner(t, db, "04")
	other := persistUser(db, "gam-runner-other-04@test.com", "91004901")

	insFinished := &dbs.SessionInstance{Name: "fin"}
	insWip := &dbs.SessionInstance{Name: "wip"}
	insInterrupted := &dbs.SessionInstance{Name: "int"}
	insOutside := &dbs.SessionInstance{Name: "out"}
	for _, inst := range []*dbs.SessionInstance{insFinished, insWip, insInterrupted, insOutside} {
		require.NoError(t, db.Create(inst).Error)
	}
	start := time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: insFinished.ID, AthleteUserID: runner.ID, Status: "finished", StartDate: start, EndDate: &end}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: insWip.ID, AthleteUserID: runner.ID, Status: "wip", StartDate: start}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: insInterrupted.ID, AthleteUserID: runner.ID, Status: "interrupted", StartDate: start, EndDate: &end}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: insOutside.ID, AthleteUserID: runner.ID, Status: "finished", StartDate: start}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: insFinished.ID, AthleteUserID: other.ID, Status: "finished", StartDate: start}).Error)

	ids := []int64{insFinished.ID, insWip.ID, insInterrupted.ID}
	sessions, err := dao.FindFinishedRunnerSessions(nil, ids, runner.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, insFinished.ID, sessions[0].SessionInstanceID)

	sessions, err = dao.FindFinishedRunnerSessions(nil, []int64{}, runner.ID)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestGamificationDAO_HasUnseenSection(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, _ := setupGamificationGroupWithRunner(t, db, "05")

	has, err := dao.HasUnseenSection(nil, runner.ID, "streaks")
	require.NoError(t, err)
	assert.False(t, has)

	require.NoError(t, db.Create(&dbs.GamificationUnseen{UserID: runner.ID, Section: "streaks"}).Error)

	has, err = dao.HasUnseenSection(nil, runner.ID, "streaks")
	require.NoError(t, err)
	assert.True(t, has)

	// Otra sección / otro usuario: false.
	has, err = dao.HasUnseenSection(nil, runner.ID, "achievements")
	require.NoError(t, err)
	assert.False(t, has)
	has, err = dao.HasUnseenSection(nil, runner.ID+9999, "streaks")
	require.NoError(t, err)
	assert.False(t, has)
}
