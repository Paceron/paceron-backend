package daos

// --- DAO de logros (gamificacion-core D5, task 3.2/3.6): Postgres real ---

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func setupAchievementsRunner(t *testing.T, db *gorm.DB, suffix string) *dbs.User {
	t.Helper()
	return persistUser(db, "ach-runner-"+suffix+"@test.com", "92000"+suffix)
}

func achievementsInstanceID(id int64) *int64 { return &id }

func TestDaos_AchievementDAO_MigracionAccesible(t *testing.T) {
	db := testutils.SetupTestDB(t)
	var n int64
	require.NoError(t, db.Table("user_achievement_progress").Count(&n).Error)
	require.NoError(t, db.Table("gamification_stamp").Count(&n).Error)
}

func TestAchievementDAO_InsertUnlock_Idempotente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "01")

	when := time.Now()

	inserted, err := dao.InsertUnlock(nil, runner.ID, "sessions_5", when)
	require.NoError(t, err)
	assert.True(t, inserted)

	// Re-evaluación idempotente: no segunda fila ni unlocked_at alterado.
	inserted, err = dao.InsertUnlock(nil, runner.ID, "sessions_5", when.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, inserted, "ON CONFLICT DO NOTHING: ya estaba desbloqueado")

	var row dbs.UserAchievementProgress
	require.NoError(t, db.Where("user_id = ? AND achievement_id = ?", runner.ID, "sessions_5").First(&row).Error)
	assert.WithinDuration(t, when, row.UnlockedAt, time.Second, "unlocked_at del primer desbloqueo intacto")

	// Otro usuario / otro logro sí insertan.
	inserted, err = dao.InsertUnlock(nil, runner.ID+1, "sessions_5", when)
	require.NoError(t, err)
	assert.True(t, inserted)
	inserted, err = dao.InsertUnlock(nil, runner.ID, "gps_first", when)
	require.NoError(t, err)
	assert.True(t, inserted)
}

func TestAchievementDAO_ListProgress(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "02")

	rows, err := dao.ListProgress(nil, runner.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)

	now := time.Now()
	require.NoError(t, db.Create(&dbs.UserAchievementProgress{UserID: runner.ID, AchievementID: "streak_7", UnlockedAt: now}).Error)
	require.NoError(t, db.Create(&dbs.UserAchievementProgress{UserID: runner.ID, AchievementID: "gps_first", UnlockedAt: now}).Error)

	rows, err = dao.ListProgress(nil, runner.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "streak_7", rows[0].AchievementID)
	assert.Equal(t, "gps_first", rows[1].AchievementID)

	// De otro usuario: aislado.
	rows, err = dao.ListProgress(nil, runner.ID+777)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestAchievementDAO_ListStamps(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "03")

	stamps, err := dao.ListStamps(nil, runner.ID)
	require.NoError(t, err)
	assert.Empty(t, stamps, "la vitrina arranca vacía hasta E4")

	granted := time.Now()
	require.NoError(t, db.Create(&dbs.GamificationStamp{
		UserID: runner.ID, Type: "competition", SourceID: 9,
		SourceName: "Competencia 9", GrantedAt: granted, Count: 1,
	}).Error)

	stamps, err = dao.ListStamps(nil, runner.ID)
	require.NoError(t, err)
	require.Len(t, stamps, 1)
	assert.Equal(t, "competition", stamps[0].Type)
	assert.Equal(t, int64(9), stamps[0].SourceID)
	assert.WithinDuration(t, granted, stamps[0].GrantedAt, time.Second)
}

func TestAchievementDAO_CountFinishedSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "04")

	n, err := dao.CountFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, n)

	start := time.Date(2026, 10, 1, 7, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 100 + int64(i), AthleteUserID: runner.ID, Status: "finished", StartDate: start, EndDate: &end}).Error)
	}
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 200, AthleteUserID: runner.ID, Status: "wip", StartDate: start}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 201, AthleteUserID: runner.ID, Status: "interrupted", StartDate: start, EndDate: &end}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 100, AthleteUserID: runner.ID + 1, Status: "finished", StartDate: start}).Error)

	n, err = dao.CountFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func seedFeedbackDistance(t *testing.T, db *gorm.DB, runnerID int64, sessionID, exerciseID int64, setNumber int, meters *float64, deletedAt *time.Time) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/a.jpg"}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID: nil, AssignedSessionID: sessionID, AssignedExerciseID: exerciseID,
		AthleteUserID: runnerID, FeedbackOwnerUserID: runnerID, ReportSource: "corredor",
		SessionDate: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), SetNumber: setNumber,
		DistanceMeters: meters, DeletedAt: deletedAt, MediaURLs: media,
	}).Error)
}

func TestAchievementDAO_SumAndMaxDistance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "05")

	total, err := dao.SumDistanceMeters(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, total)
	maxSingle, err := dao.MaxDistanceSingleSession(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, maxSingle)

	d35 := 35000.0
	d15 := 15000.0
	seedFeedbackDistance(t, db, runner.ID, 11, 111, 1, &d35, nil)
	seedFeedbackDistance(t, db, runner.ID, 11, 111, 2, nil, nil)  // set sin distancia: no suma
	seedFeedbackDistance(t, db, runner.ID, 12, 112, 1, &d15, nil) // otra sesión: 15k
	deleted := time.Now()
	seedFeedbackDistance(t, db, runner.ID, 13, 113, 1, &d35, &deleted) // bajado: excluido

	total, err = dao.SumDistanceMeters(nil, runner.ID)
	require.NoError(t, err)
	assert.InEpsilon(t, 50000.0, total, 0.0001)

	maxSingle, err = dao.MaxDistanceSingleSession(nil, runner.ID)
	require.NoError(t, err)
	assert.InEpsilon(t, 35000.0, maxSingle, 0.0001)

	// De otro usuario: aislado.
	seedFeedbackDistance(t, db, runner.ID+1, 11, 111, 1, &d35, nil)
	total, err = dao.SumDistanceMeters(nil, runner.ID)
	require.NoError(t, err)
	assert.InEpsilon(t, 50000.0, total, 0.0001)
}

func TestAchievementDAO_CountGPSFinishedSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "06")

	n, err := dao.CountGPSFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, n)

	start := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 300, AthleteUserID: runner.ID, Status: "finished", StartDate: start, EndDate: &end}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 301, AthleteUserID: runner.ID, Status: "finished", StartDate: start, EndDate: &end}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 302, AthleteUserID: runner.ID, Status: "wip", StartDate: start}).Error)

	point := func(instance int64) *dbs.WorkoutFeedbackPoint {
		return &dbs.WorkoutFeedbackPoint{FeedbackID: instance, SessionInstanceID: instance, ExerciseInstanceID: 1, Order: 0, Latitude: -31.4, Longitude: -64.2, RecordedAt: start}
	}
	require.NoError(t, db.Create(point(300)).Error) // GPS en la instancia 300
	require.NoError(t, db.Create(point(301)).Error)
	require.NoError(t, db.Create(point(302)).Error) // wip: NO cuenta
	require.NoError(t, db.Create(point(399)).Error) // sin sesión del runner: NO cuenta

	n, err = dao.CountGPSFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

func TestAchievementDAO_ListFinishedSessionStarts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "07")

	starts, err := dao.ListFinishedSessionStarts(nil, runner.ID)
	require.NoError(t, err)
	assert.Empty(t, starts)

	early := time.Date(2026, 10, 4, 10, 30, 0, 0, time.UTC) // 07:30 en Cordoba
	late := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)   // 11:00 en Cordoba
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 400, AthleteUserID: runner.ID, Status: "finished", StartDate: early}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 401, AthleteUserID: runner.ID, Status: "finished", StartDate: late}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: 402, AthleteUserID: runner.ID, Status: "interrupted", StartDate: early}).Error)

	starts, err = dao.ListFinishedSessionStarts(nil, runner.ID)
	require.NoError(t, err)
	require.Len(t, starts, 2)
}

func TestAchievementDAO_CountAttendanceDays(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "08")

	n, err := dao.CountAttendanceDays(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, n)

	// Dos instancias en días distintos + una tercera el MISMO día (otro grupo,
	// porque el calendario es único por (grupo, fecha)).
	d1 := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 6, 15, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: 1, Date: d1, Kind: "training", SessionInstanceID: achievementsInstanceID(500)}).Error)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: 1, Date: d2, Kind: "training", SessionInstanceID: achievementsInstanceID(501)}).Error)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: 2, Date: d2, Kind: "training", SessionInstanceID: achievementsInstanceID(502)}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 500, UserID: runner.ID, Source: "qr"}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 501, UserID: runner.ID, Source: "manual"}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 502, UserID: runner.ID, Source: "qr"}).Error) // mismo día
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 501, UserID: runner.ID + 1, Source: "qr"}).Error)

	// 3 asistencias pero solo 2 días distintos.
	n, err = dao.CountAttendanceDays(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

func TestAchievementDAO_InsertUnlock_ConsistenciaConGetByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "09")

	when := time.Now()
	require.NoError(t, db.Create(&dbs.UserAchievementProgress{UserID: runner.ID, AchievementID: "team_first", UnlockedAt: when}).Error)

	inserted, err := dao.InsertUnlock(nil, runner.ID, "team_first", when)
	require.NoError(t, err)
	assert.False(t, inserted)

	var count int64
	require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", runner.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "la PK compuesta impide la segunda fila")
}
