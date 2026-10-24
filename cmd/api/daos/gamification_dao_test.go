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

func TestGamificationDAO_DeleteUnseen(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, _ := setupGamificationGroupWithRunner(t, db, "06")

	teamA, teamB := int64(11), int64(22)
	require.NoError(t, db.Create(&dbs.GamificationUnseen{UserID: runner.ID, Section: "streaks"}).Error)
	require.NoError(t, db.Create(&dbs.GamificationUnseen{UserID: runner.ID, Section: "competitions", TeamID: &teamA}).Error)
	require.NoError(t, db.Create(&dbs.GamificationUnseen{UserID: runner.ID, Section: "competitions", TeamID: &teamB}).Error)
	require.NoError(t, db.Create(&dbs.GamificationUnseen{UserID: runner.ID, Section: "achievements"}).Error)
	require.NoError(t, db.Create(&dbs.GamificationUnseen{UserID: runner.ID + 1, Section: "streaks"}).Error)

	// Con team_id: solo filas de competitions de ese team.
	deleted, err := dao.DeleteUnseen(nil, runner.ID, "competitions", &teamA)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)

	var rest int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?", runner.ID, "competitions").Count(&rest).Error)
	assert.EqualValues(t, 1, rest, "queda la fila del team B")

	// Sin team_id en competitions: limpia TODAS las de la sección del
	// usuario, incluidas las por-equipo.
	deleted, err = dao.DeleteUnseen(nil, runner.ID, "competitions", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)

	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?", runner.ID, "competitions").Count(&rest).Error)
	assert.Zero(t, rest)

	// Otras secciones / otros usuarios intactas + seen repetido deja 0.
	deleted, err = dao.DeleteUnseen(nil, runner.ID, "streaks", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	deleted, err = dao.DeleteUnseen(nil, runner.ID, "streaks", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 0, deleted, "no-op exitoso")
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?", runner.ID+1, "streaks").Count(&rest).Error)
	assert.EqualValues(t, 1, rest)
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?", runner.ID, "achievements").Count(&rest).Error)
	assert.EqualValues(t, 1, rest)
}

func TestGamificationDAO_InsertUnseen(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, _ := setupGamificationGroupWithRunner(t, db, "07")

	err := dao.InsertUnseen(nil, nil)
	require.NoError(t, err, "nil vacío = no-op")

	rows := []dbs.GamificationUnseen{
		{UserID: runner.ID, Section: "streaks"},
		{UserID: runner.ID, Section: "streaks"},
	}
	err = dao.InsertUnseen(nil, rows)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?", runner.ID, "streaks").Count(&count).Error)
	assert.EqualValues(t, 2, count)
}

func TestGamificationDAO_UpsertStreakState(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, _ := setupGamificationGroupWithRunner(t, db, "08")

	// Alta cuando no hay fila.
	err := dao.UpsertStreakState(nil, runner.ID, 3, 7)
	require.NoError(t, err)
	state, err := dao.GetStreakState(nil, runner.ID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 3, state.StrictCurrent)
	assert.Equal(t, 7, state.FlexibleCurrent)

	// Update del mismo user_id (conflicto ya va por esa PK).
	err = dao.UpsertStreakState(nil, runner.ID, 10, 20)
	require.NoError(t, err)
	state, err = dao.GetStreakState(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, state.StrictCurrent)
	assert.Equal(t, 20, state.FlexibleCurrent)
	assert.False(t, state.UpdatedAt.IsZero())
}

func TestGamificationDAO_GetStreakState_SinFila(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGamificationDao(db)
	runner, _ := setupGamificationGroupWithRunner(t, db, "09")

	state, err := dao.GetStreakState(nil, runner.ID)
	require.NoError(t, err)
	assert.Nil(t, state, "sin fila aún: nil, el trigger trata el prev como 0/0")
}
