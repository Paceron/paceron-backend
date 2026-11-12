package daos

// --- DAO de logros (gamificacion-core D5, task 3.2/3.6): Postgres real ---

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"

	"simple-arq-golang/cmd/api/domains/constants"
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

func TestAchievementDAO_InsertUnlockWithUnseen_Idempotente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "01")

	when := time.Now()

	inserted, err := dao.InsertUnlockWithUnseen(nil, runner.ID, "sessions_5", when)
	require.NoError(t, err)
	assert.True(t, inserted)

	// Re-evaluación idempotente: no segunda fila ni unlocked_at alterado.
	inserted, err = dao.InsertUnlockWithUnseen(nil, runner.ID, "sessions_5", when.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, inserted, "ON CONFLICT DO NOTHING: ya estaba desbloqueado")

	var row dbs.UserAchievementProgress
	require.NoError(t, db.Where("user_id = ? AND achievement_id = ?", runner.ID, "sessions_5").First(&row).Error)
	assert.WithinDuration(t, when, row.UnlockedAt, time.Second, "unlocked_at del primer desbloqueo intacto")

	var unseen int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?",
		runner.ID, constants.GamificationSectionAchievements).Count(&unseen).Error)
	assert.Equal(t, int64(1), unseen, "el unlock nuevo deja su unseen; la re-evaluación no lo duplica")

	// Otro usuario / otro logro sí insertan.
	inserted, err = dao.InsertUnlockWithUnseen(nil, runner.ID+1, "sessions_5", when)
	require.NoError(t, err)
	assert.True(t, inserted)
	inserted, err = dao.InsertUnlockWithUnseen(nil, runner.ID, "gps_first", when)
	require.NoError(t, err)
	assert.True(t, inserted)

	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?",
		runner.ID, constants.GamificationSectionAchievements).Count(&unseen).Error)
	assert.Equal(t, int64(2), unseen)
}

func TestAchievementDAO_InsertUnlocksWithUnseen_BatchIdempotenteYOrden(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "18")

	when := time.Now()
	inserted, err := dao.InsertUnlocksWithUnseen(nil, runner.ID, []string{"sessions_5", "early_5", "gps_first"}, when)
	require.NoError(t, err)
	assert.Equal(t, []string{"sessions_5", "early_5", "gps_first"}, inserted, "solo nuevos y en orden recibido")

	var first dbs.UserAchievementProgress
	require.NoError(t, db.Where("user_id = ? AND achievement_id = ?", runner.ID, "sessions_5").First(&first).Error)
	assert.WithinDuration(t, when, first.UnlockedAt, time.Second)
	assert.False(t, first.CreatedAt.IsZero(), "el insert batch completa created_at")

	// Re-evaluación: solo lo que faltaba, sin duplicar ni tocar unlocked_at.
	inserted, err = dao.InsertUnlocksWithUnseen(nil, runner.ID, []string{"sessions_5", "streak_7", "early_5"}, when.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, []string{"streak_7"}, inserted)

	var unchanged dbs.UserAchievementProgress
	require.NoError(t, db.Where("user_id = ? AND achievement_id = ?", runner.ID, "sessions_5").First(&unchanged).Error)
	assert.WithinDuration(t, when, unchanged.UnlockedAt, time.Second)

	var count int64
	require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", runner.ID).Count(&count).Error)
	assert.Equal(t, int64(4), count)

	var unseen int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?",
		runner.ID, constants.GamificationSectionAchievements).Count(&unseen).Error)
	assert.Equal(t, int64(4), unseen, "solo los 4 unlocks nuevos dejaron unseen")

	inserted, err = dao.InsertUnlocksWithUnseen(nil, runner.ID, nil, when)
	require.NoError(t, err)
	assert.Empty(t, inserted)

	// Otro usuario y otro logro del mismo usuario sí insertan.
	inserted, err = dao.InsertUnlocksWithUnseen(nil, runner.ID+1, []string{"sessions_5"}, when)
	require.NoError(t, err)
	assert.Equal(t, []string{"sessions_5"}, inserted)
	inserted, err = dao.InsertUnlocksWithUnseen(nil, runner.ID, []string{"distance_10k"}, when)
	require.NoError(t, err)
	assert.Equal(t, []string{"distance_10k"}, inserted)

	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?",
		runner.ID, constants.GamificationSectionAchievements).Count(&unseen).Error)
	assert.Equal(t, int64(5), unseen)
}

func TestAchievementDAO_InsertUnlocksWithUnseen_UnaSolaSentencia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	runner := setupAchievementsRunner(t, db, "19")

	rec := &sqlRecorderLogger{}
	dao := NewAchievementDao(db.Session(&gorm.Session{Logger: rec}))

	inserted, err := dao.InsertUnlocksWithUnseen(nil, runner.ID, []string{"sessions_5", "early_5"}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, []string{"sessions_5", "early_5"}, inserted)

	require.Len(t, rec.statements, 1, "unlock y unseen en una sola sentencia atómica")
	sql := strings.ToUpper(rec.statements[0])
	assert.Contains(t, sql, "WITH INSERTED AS")
	assert.Contains(t, sql, "INSERT INTO USER_ACHIEVEMENT_PROGRESS")
	assert.Contains(t, sql, "INSERT INTO GAMIFICATION_UNSEEN")
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

func TestAchievementDAO_CountEarlyFinishedSessions_Paridad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "10")

	n, err := dao.CountEarlyFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, n)

	// TZ Córdoba (UTC-3): 10:59 UTC = 07:59 → cuenta; 11:00 UTC = 08:00 → no.
	early := time.Date(2026, 10, 4, 10, 59, 0, 0, time.UTC)
	exact := time.Date(2026, 10, 4, 11, 0, 0, 0, time.UTC)
	late := time.Date(2026, 10, 4, 11, 1, 0, 0, time.UTC)
	midnight := time.Date(2026, 10, 4, 3, 30, 0, 0, time.UTC) // 00:30 Córdoba
	// Instante con otro offset: 18:30 en JST = 06:30 en Córdoba.
	tokyoEarly := time.Date(2026, 10, 5, 18, 30, 0, 0, time.FixedZone("JST", 9*3600))

	mkSession := func(instanceID int64, athleteID int64, status string, start time.Time) {
		t.Helper()
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: instanceID, AthleteUserID: athleteID, Status: status, StartDate: start,
		}).Error)
	}
	mkSession(600, runner.ID, "finished", early)
	mkSession(601, runner.ID, "finished", exact)
	mkSession(602, runner.ID, "finished", late)
	mkSession(603, runner.ID, "finished", midnight)
	mkSession(604, runner.ID, "finished", tokyoEarly)
	mkSession(605, runner.ID, "wip", early)
	mkSession(606, runner.ID, "interrupted", early)
	mkSession(607, runner.ID+1, "finished", early)

	n, err = dao.CountEarlyFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n, "07:59, 00:30 y el instante de otra TZ; 08:00 exacto no")
}

type sqlRecorderLogger struct {
	gl.Interface
	statements []string
}

func (r *sqlRecorderLogger) LogMode(gl.LogLevel) gl.Interface              { return r }
func (r *sqlRecorderLogger) Info(context.Context, string, ...interface{})  {}
func (r *sqlRecorderLogger) Warn(context.Context, string, ...interface{})  {}
func (r *sqlRecorderLogger) Error(context.Context, string, ...interface{}) {}
func (r *sqlRecorderLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.statements = append(r.statements, sql)
}

// cteErrorPool inyecta un error de transporte sobre la sentencia atómica de
// unlocks+unseen (empieza con WITH, así que FailingDB no la clasifica).
type cteErrorPool struct {
	underlying gorm.ConnPool
}

func (p *cteErrorPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func (p *cteErrorPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *cteErrorPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if strings.Contains(strings.ToUpper(query), "GAMIFICATION_UNSEEN") {
		return nil, errors.New("fallo inyectado en el batch")
	}
	return p.underlying.QueryContext(ctx, query, args...)
}

func (p *cteErrorPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.underlying.QueryRowContext(ctx, query, args...)
}

func TestAchievementDAO_CountEarlyFinishedSessions_UnaSolaQueryAgregada(t *testing.T) {
	db := testutils.SetupTestDB(t)
	runner := setupAchievementsRunner(t, db, "11")

	for i := int64(0); i < 20; i++ {
		start := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
		if i%2 == 0 {
			start = time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
		}
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: 700 + i, AthleteUserID: runner.ID, Status: "finished", StartDate: start,
		}).Error)
	}

	rec := &sqlRecorderLogger{}
	dao := NewAchievementDao(db.Session(&gorm.Session{Logger: rec}))

	n, err := dao.CountEarlyFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(10), n)
	require.Len(t, rec.statements, 1, "agrega en una sola query, sin traer el histórico")
	sql := strings.ToUpper(rec.statements[0])
	assert.Contains(t, sql, "COUNT(*)")
	assert.Contains(t, sql, "AT TIME ZONE")
	assert.NotContains(t, sql, "SELECT START_DATE")
}

func TestAchievementDAO_CountFinishedSessionStats_Paridad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "14")

	stats, err := dao.CountFinishedSessionStats(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, stats.Finished)
	assert.Zero(t, stats.Early)

	early := time.Date(2026, 10, 4, 10, 59, 0, 0, time.UTC)   // 07:59 Córdoba
	exact := time.Date(2026, 10, 4, 11, 0, 0, 0, time.UTC)    // 08:00 Córdoba
	late := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)     // 11:00 Córdoba
	midnight := time.Date(2026, 10, 4, 3, 30, 0, 0, time.UTC) // 00:30 Córdoba
	tokyoEarly := time.Date(2026, 10, 5, 18, 30, 0, 0, time.FixedZone("JST", 9*3600))

	mkSession := func(instanceID int64, athleteID int64, status string, start time.Time) {
		t.Helper()
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: instanceID, AthleteUserID: athleteID, Status: status, StartDate: start,
		}).Error)
	}
	mkSession(800, runner.ID, "finished", early)
	mkSession(801, runner.ID, "finished", exact)
	mkSession(802, runner.ID, "finished", late)
	mkSession(803, runner.ID, "finished", midnight)
	mkSession(804, runner.ID, "finished", tokyoEarly)
	mkSession(805, runner.ID, "wip", early)
	mkSession(806, runner.ID, "interrupted", early)
	mkSession(807, runner.ID+1, "finished", early)

	stats, err = dao.CountFinishedSessionStats(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5), stats.Finished, "solo las 5 finished del runner")
	assert.Equal(t, int64(3), stats.Early, "07:59, 00:30 y el instante de otra TZ; 08:00 exacto no")

	// Paridad con los agregados separados que ya consumía el evaluador.
	finished, err := dao.CountFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	earlyCount, err := dao.CountEarlyFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, finished, stats.Finished)
	assert.Equal(t, earlyCount, stats.Early)
}

func TestAchievementDAO_CountFinishedSessionStats_UnaSolaQuery(t *testing.T) {
	db := testutils.SetupTestDB(t)
	runner := setupAchievementsRunner(t, db, "15")

	for i := int64(0); i < 20; i++ {
		start := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
		if i%2 == 0 {
			start = time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
		}
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: 900 + i, AthleteUserID: runner.ID, Status: "finished", StartDate: start,
		}).Error)
	}

	rec := &sqlRecorderLogger{}
	dao := NewAchievementDao(db.Session(&gorm.Session{Logger: rec}))

	stats, err := dao.CountFinishedSessionStats(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(20), stats.Finished)
	assert.Equal(t, int64(10), stats.Early)
	require.Len(t, rec.statements, 1, "finished y early salen de una sola lectura")
	sql := strings.ToUpper(rec.statements[0])
	assert.Contains(t, sql, "COUNT(*) FILTER")
	assert.Contains(t, sql, "AT TIME ZONE")
	assert.NotContains(t, sql, "SELECT START_DATE")
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

func TestAchievementDAO_SumAndMaxDistance_Paridad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "16")

	stats, err := dao.SumAndMaxDistance(nil, runner.ID)
	require.NoError(t, err)
	assert.Zero(t, stats.Total)
	assert.Zero(t, stats.MaxSingle)

	d35 := 35000.0
	d15 := 15000.0
	d20 := 20000.0
	seedFeedbackDistance(t, db, runner.ID, 21, 111, 1, &d35, nil)
	seedFeedbackDistance(t, db, runner.ID, 21, 111, 2, nil, nil)
	seedFeedbackDistance(t, db, runner.ID, 22, 112, 1, &d15, nil)
	seedFeedbackDistance(t, db, runner.ID, 22, 112, 2, &d20, nil) // misma sesión: 35k
	deleted := time.Now()
	seedFeedbackDistance(t, db, runner.ID, 23, 113, 1, &d35, &deleted)
	seedFeedbackDistance(t, db, runner.ID+1, 21, 111, 1, &d35, nil)

	stats, err = dao.SumAndMaxDistance(nil, runner.ID)
	require.NoError(t, err)
	assert.InEpsilon(t, 70000.0, stats.Total, 0.0001)
	assert.InEpsilon(t, 35000.0, stats.MaxSingle, 0.0001)

	total, err := dao.SumDistanceMeters(nil, runner.ID)
	require.NoError(t, err)
	maxSingle, err := dao.MaxDistanceSingleSession(nil, runner.ID)
	require.NoError(t, err)
	assert.InEpsilon(t, total, stats.Total, 0.0001)
	assert.InEpsilon(t, maxSingle, stats.MaxSingle, 0.0001)
}

func TestAchievementDAO_SumAndMaxDistance_UnaSolaQuery(t *testing.T) {
	db := testutils.SetupTestDB(t)
	runner := setupAchievementsRunner(t, db, "17")

	d := 12000.0
	for i := int64(0); i < 10; i++ {
		seedFeedbackDistance(t, db, runner.ID, 30+i, 1, 1, &d, nil)
	}

	rec := &sqlRecorderLogger{}
	dao := NewAchievementDao(db.Session(&gorm.Session{Logger: rec}))

	stats, err := dao.SumAndMaxDistance(nil, runner.ID)
	require.NoError(t, err)
	assert.InEpsilon(t, 120000.0, stats.Total, 0.0001)
	assert.InEpsilon(t, 12000.0, stats.MaxSingle, 0.0001)
	require.Len(t, rec.statements, 1, "total y máximo por sesión salen de una sola lectura")
	sql := strings.ToUpper(rec.statements[0])
	assert.Contains(t, sql, "GROUP BY ASSIGNED_SESSION_ID")
	assert.Contains(t, sql, "MAX(")
}

func seedGPSFeedback(t *testing.T, db *gorm.DB, athleteID, key int64, deletedAt *time.Time) int64 {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/a.jpg"}))
	f := &dbs.WorkoutFeedback{
		AssignedSessionID: key, AssignedExerciseID: 1,
		AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID,
		ReportSource: "corredor",
		SessionDate:  time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
		SetNumber:    1, DeletedAt: deletedAt, MediaURLs: media,
	}
	require.NoError(t, db.Create(f).Error)
	return f.ID
}

func seedGPSPoint(t *testing.T, db *gorm.DB, feedbackID, instanceID int64, order int, at time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&dbs.WorkoutFeedbackPoint{
		FeedbackID: feedbackID, SessionInstanceID: instanceID, ExerciseInstanceID: 1,
		Order: order, Latitude: -31.4, Longitude: -64.2, RecordedAt: at,
	}).Error)
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

	seedGPSPoint(t, db, seedGPSFeedback(t, db, runner.ID, 300, nil), 300, 0, start) // GPS en la instancia 300
	seedGPSPoint(t, db, seedGPSFeedback(t, db, runner.ID, 301, nil), 301, 0, start)
	seedGPSPoint(t, db, seedGPSFeedback(t, db, runner.ID, 302, nil), 302, 0, start) // wip: NO cuenta
	seedGPSPoint(t, db, seedGPSFeedback(t, db, runner.ID, 399, nil), 399, 0, start) // sin sesión del runner: NO cuenta

	n, err = dao.CountGPSFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

func TestAchievementDAO_CountGPSFinishedSessions_AtribuyePuntosAlAtletaDelFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "12")
	other := setupAchievementsRunner(t, db, "13")

	start := time.Date(2026, 10, 6, 7, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	mkFinished := func(instanceID, athleteID int64) {
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: instanceID, AthleteUserID: athleteID,
			Status: "finished", StartDate: start, EndDate: &end,
		}).Error)
	}

	// 500: instancia compartida con puntos de ambos; los del runner cuentan una vez.
	mkFinished(500, runner.ID)
	mkFinished(500, other.ID)
	own500 := seedGPSFeedback(t, db, runner.ID, 500, nil)
	seedGPSPoint(t, db, own500, 500, 0, start)
	seedGPSPoint(t, db, own500, 500, 1, start.Add(time.Minute))
	seedGPSPoint(t, db, seedGPSFeedback(t, db, other.ID, 500, nil), 500, 0, start)

	// 501: solo el otro atleta tiene puntos; no cuenta para el runner.
	mkFinished(501, runner.ID)
	mkFinished(501, other.ID)
	seedGPSPoint(t, db, seedGPSFeedback(t, db, other.ID, 501, nil), 501, 0, start)

	// 502: feedback soft-deleted del propio atleta; la política vigente lo mantiene.
	mkFinished(502, runner.ID)
	deleted := time.Now()
	seedGPSPoint(t, db, seedGPSFeedback(t, db, runner.ID, 502, &deleted), 502, 0, start)

	n, err := dao.CountGPSFinishedSessions(nil, runner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "500 y 502; 501 solo tiene puntos del otro atleta")

	// Re-evaluación idempotente: mismo agregado.
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

func TestAchievementDAO_CountAttendanceDaysByUsers(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runnerA := setupAchievementsRunner(t, db, "08a")
	runnerB := setupAchievementsRunner(t, db, "08b")
	runnerC := setupAchievementsRunner(t, db, "08c")

	empty, err := dao.CountAttendanceDaysByUsers(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	d1 := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 6, 15, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: 1, Date: d1, Kind: "training", SessionInstanceID: achievementsInstanceID(510)}).Error)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: 1, Date: d2, Kind: "training", SessionInstanceID: achievementsInstanceID(511)}).Error)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{GroupID: 2, Date: d2, Kind: "training", SessionInstanceID: achievementsInstanceID(512)}).Error)

	// A: dos días distintos, incluso desde equipos distintos. B y C: un día,
	// compartido entre dos grupos para probar el COUNT(DISTINCT date).
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 510, UserID: runnerA.ID, Source: "qr"}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 2, TrainingSessionID: 511, UserID: runnerA.ID, Source: "manual"}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 2, TrainingSessionID: 512, UserID: runnerA.ID, Source: "qr"}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 511, UserID: runnerB.ID, Source: "qr"}).Error)
	require.NoError(t, db.Create(&dbs.Attendance{TeamID: 1, TrainingSessionID: 512, UserID: runnerC.ID, Source: "qr"}).Error)

	counts, err := dao.CountAttendanceDaysByUsers(nil, []int64{runnerA.ID, runnerB.ID, runnerC.ID, runnerA.ID + 999})
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{
		runnerA.ID: 2,
		runnerB.ID: 1,
		runnerC.ID: 1,
	}, counts, "solo los usuarios con filas aparecen; los días se agrupan por usuario")
}

func TestAchievementDAO_CountAttendanceDaysByUsers_QueryCountConstante(t *testing.T) {
	db := testutils.SetupTestDB(t)
	counting, counter := countingDB(db)
	dao := NewAchievementDao(counting)

	_, err := dao.CountAttendanceDaysByUsers(nil, []int64{1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), *counter, "la agregación es una sola lectura")

	*counter = 0
	users := make([]int64, 50)
	for i := range users {
		users[i] = int64(i + 1)
	}
	_, err = dao.CountAttendanceDaysByUsers(nil, users)
	require.NoError(t, err)
	assert.Equal(t, int64(1), *counter, "el roster grande no multiplica queries")
}

func TestAchievementDAO_CountAttendanceDaysByUsers_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewAchievementDao(failing)

	counts, err := dao.CountAttendanceDaysByUsers(nil, []int64{1, 2})

	require.Error(t, err)
	assert.Nil(t, counts)
	assert.Contains(t, err.Error(), "error counting attendance days by users")
}

func TestAchievementDAO_InsertUnlockWithUnseen_ConsistenciaConGetByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewAchievementDao(db)
	runner := setupAchievementsRunner(t, db, "09")

	when := time.Now()
	require.NoError(t, db.Create(&dbs.UserAchievementProgress{UserID: runner.ID, AchievementID: "team_first", UnlockedAt: when}).Error)

	inserted, err := dao.InsertUnlockWithUnseen(nil, runner.ID, "team_first", when)
	require.NoError(t, err)
	assert.False(t, inserted)

	var count int64
	require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", runner.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "la PK compuesta impide la segunda fila")

	var unseen int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ?", runner.ID).Count(&unseen).Error)
	assert.Zero(t, unseen, "un unlock viejo no genera unseen nuevo")
}

// El conteo agregado de early_N propaga el error de DB sin tragarlo (el caller
// de gamificación decide si re-evalúa o no).
func TestAchievementDAO_CountEarlyFinishedSessions_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewAchievementDao(failing)

	n, err := dao.CountEarlyFinishedSessions(nil, 1)

	require.Error(t, err)
	assert.Zero(t, n)
	assert.Contains(t, err.Error(), "error counting early finished runner sessions")
}

func TestAchievementDAO_CountFinishedSessionStats_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewAchievementDao(failing)

	stats, err := dao.CountFinishedSessionStats(nil, 1)

	require.Error(t, err)
	assert.Zero(t, stats)
	assert.Contains(t, err.Error(), "error counting finished session stats")
}

func TestAchievementDAO_SumAndMaxDistance_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewAchievementDao(failing)

	stats, err := dao.SumAndMaxDistance(nil, 1)

	require.Error(t, err)
	assert.Zero(t, stats)
	assert.Contains(t, err.Error(), "error computing distance stats")
}

func TestAchievementDAO_InsertUnlocksWithUnseen_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	failing.Statement.ConnPool = &cteErrorPool{underlying: db.Statement.ConnPool}
	dao := NewAchievementDao(failing)

	inserted, err := dao.InsertUnlocksWithUnseen(nil, 1, []string{"sessions_5"}, time.Now())

	require.Error(t, err)
	assert.Empty(t, inserted)
	assert.Contains(t, err.Error(), "error inserting achievement unlocks")
}
