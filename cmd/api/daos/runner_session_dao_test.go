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

func TestNewRunnerSessionDao(t *testing.T) {
	dao := NewRunnerSessionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestRunnerSessionDao_ImplementsInterface(t *testing.T) {
	dao := NewRunnerSessionDao(&gorm.DB{})
	var iface RunnerSessionDAOInterface = dao
	_ = iface
}

func testRunnerSessionSeed(t *testing.T, db *gorm.DB, sessionInstanceID, athleteUserID int64, start time.Time, status string, end *time.Time) *dbs.RunnerSession {
	t.Helper()
	if status == "" {
		status = "wip"
	}
	rs := &dbs.RunnerSession{
		SessionInstanceID: sessionInstanceID,
		AthleteUserID:     athleteUserID,
		Status:            status,
		StartDate:         start,
		EndDate:           end,
	}
	require.NoError(t, db.Create(rs).Error)
	return rs
}

func TestRunnerSessionDao_Create_Inserts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)

	rs := &dbs.RunnerSession{
		SessionInstanceID: 1,
		AthleteUserID:     7,
		StartDate:         time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC),
	}

	created, err := dao.Create(nil, rs)

	require.NoError(t, err)
	assert.True(t, created)
	assert.NotZero(t, rs.ID)
	assert.Equal(t, "wip", rs.Status)
	assert.Nil(t, rs.EndDate)
}

func TestRunnerSessionDao_Create_Existing_NoDuplicate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)

	first := &dbs.RunnerSession{
		SessionInstanceID: 1,
		AthleteUserID:     7,
		StartDate:         time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC),
	}
	_, createErr := dao.Create(nil, first)
	require.NoError(t, createErr)

	// Idempotencia: reintento del MISMO (session, athlete) → no inserta ni pisa.
	retry := *first
	retry.ID = 0
	created, err := dao.Create(nil, &retry)

	require.NoError(t, err)
	assert.False(t, created)

	var count int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestRunnerSessionDao_Create_AnotherAthleteSameSession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)

	a := &dbs.RunnerSession{
		SessionInstanceID: 1,
		AthleteUserID:     7,
		StartDate:         time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC),
	}
	_, createErr := dao.Create(nil, a)
	require.NoError(t, createErr)

	// Dos atletas comparten la sesión (día de grupo): la UNIQUE es por par
	// (session, athlete), no por sesión sola.
	b := &dbs.RunnerSession{
		SessionInstanceID: 1,
		AthleteUserID:     8,
		StartDate:         time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC),
	}

	created, err := dao.Create(nil, b)

	require.NoError(t, err)
	assert.True(t, created)
	assert.NotZero(t, b.ID)
}

func TestRunnerSessionDao_GetBySessionAndAthlete_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)
	rs := testRunnerSessionSeed(t, db, 1, 7, time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC), "", nil)

	got, err := dao.GetBySessionAndAthlete(nil, 1, 7)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, rs.ID, got.ID)
	assert.Equal(t, "wip", got.Status)
}

func TestRunnerSessionDao_GetBySessionAndAthlete_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)

	_, err := dao.GetBySessionAndAthlete(nil, 1, 7)

	require.ErrorIs(t, err, ErrRunnerSessionNotFound)
}

func TestRunnerSessionDao_GetBySessionAndAthlete_OtherAthleteIsolated(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)
	testRunnerSessionSeed(t, db, 1, 7, time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC), "", nil)

	_, err := dao.GetBySessionAndAthlete(nil, 1, 8)

	require.ErrorIs(t, err, ErrRunnerSessionNotFound)
}

func TestRunnerSessionDao_Finish_MarksFinished(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)
	rs := testRunnerSessionSeed(t, db, 1, 7, time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC), "", nil)
	end := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	rs.EndDate = &end

	err := dao.Finish(nil, rs)

	require.NoError(t, err)

	got, err := dao.GetBySessionAndAthlete(nil, 1, 7)
	require.NoError(t, err)
	assert.Equal(t, "finished", got.Status)
	require.NotNil(t, got.EndDate)
	assert.Equal(t, end, *got.EndDate)
}

func TestRunnerSessionDao_Finish_OnFinished_DoesNotRewrite(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)
	// Se arranca ya finalizado con un end_date conocido.
	originalEnd := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	rs := testRunnerSessionSeed(t, db, 1, 7, time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC), "finished", &originalEnd)

	// Segundo intento de finish con un end_date distinto: el WHERE status='wip'
	// no matchea → no reescribe nada.
	newEnd := originalEnd.Add(time.Hour)
	rs.EndDate = &newEnd

	err := dao.Finish(nil, rs)

	require.NoError(t, err)

	got, err := dao.GetBySessionAndAthlete(nil, 1, 7)
	require.NoError(t, err)
	assert.Equal(t, "finished", got.Status)
	require.NotNil(t, got.EndDate)
	assert.Equal(t, originalEnd, *got.EndDate)
}

func TestRunnerSessionDao_SessionInstanceExists(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)

	exists, err := dao.SessionInstanceExists(nil, 999999)
	require.NoError(t, err)
	assert.False(t, exists)

	inst := &dbs.SessionInstance{Name: "Fartlek 5K"}
	require.NoError(t, db.Create(inst).Error)

	exists, err = dao.SessionInstanceExists(nil, inst.ID)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRunnerSessionDao_MembershipDelegation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewRunnerSessionDao(db)

	owner := persistUser(db, "rs-owner@test.com", "32000001")
	athlete := persistUser(db, "rs-athlete@test.com", "32000002")
	team := testTeam(db, "rs_delegation", owner.ID)

	inTeam, err := dao.ExistsUserInTeamOwnedBy(nil, athlete.ID, owner.ID)
	require.NoError(t, err)
	// El atleta todavía no fue agregado al equipo.
	assert.False(t, inTeam)

	exists, err := dao.TeamExists(nil, team.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, db.Create(&dbs.TeamUser{
		TeamID:         team.ID,
		UserID:         athlete.ID,
		RoleInTeam:     "corredor",
		AssignmentDate: time.Now(),
	}).Error)

	inTeam, err = dao.ExistsUserInTeamOwnedBy(nil, athlete.ID, owner.ID)
	require.NoError(t, err)
	assert.True(t, inTeam)
}