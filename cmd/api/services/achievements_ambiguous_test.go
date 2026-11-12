package services

// Task 5 fix round: el batch de unlocks y sus unseen viajan en una sola
// sentencia. Si la DB la ejecuta pero la respuesta se pierde, el unseen ya
// quedó durable; el fallback no lo duplica ni inventa avisos para unlocks
// viejos.

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// lostResponsePool ejecuta de verdad la sentencia atómica de unlocks+unseen y
// recién después descarta su respuesta una sola vez, simulando la caída de la
// respuesta con la DB ya confirmada. No falsea el commit: la sentencia corre
// completa contra Postgres.
type lostResponsePool struct {
	underlying gorm.ConnPool
	committer  gorm.TxCommitter
	injected   bool
}

func (p *lostResponsePool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func (p *lostResponsePool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *lostResponsePool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := p.underlying.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	upper := strings.ToUpper(query)
	if !p.injected && strings.Contains(upper, "INSERT INTO USER_ACHIEVEMENT_PROGRESS") &&
		strings.Contains(upper, "INSERT INTO GAMIFICATION_UNSEEN") {
		p.injected = true
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, errors.New("respuesta perdida después de ejecutar el batch")
	}
	return rows, nil
}

func (p *lostResponsePool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.underlying.QueryRowContext(ctx, query, args...)
}

func (p *lostResponsePool) Commit() error {
	if p.committer == nil {
		return errors.New("lostResponsePool sin committer")
	}
	return p.committer.Commit()
}

func (p *lostResponsePool) Rollback() error {
	if p.committer == nil {
		return errors.New("lostResponsePool sin committer")
	}
	return p.committer.Rollback()
}

func lostResponseDB(t *testing.T, db *gorm.DB, pool *lostResponsePool) *gorm.DB {
	t.Helper()
	pool.underlying = db.Statement.ConnPool
	if committer, ok := db.Statement.ConnPool.(gorm.TxCommitter); ok {
		pool.committer = committer
	}
	sess := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	sess.Statement.ConnPool = pool
	return sess
}

func TestAchievementsService_AmbiguousBatch_UnseenDurable(t *testing.T) {
	db := testutils.SetupTestDB(t)
	const athleteID int64 = 99101

	start := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
	for i := int64(0); i < 5; i++ {
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: 3000 + i, AthleteUserID: athleteID, Status: "finished", StartDate: start,
		}).Error)
	}

	pool := &lostResponsePool{}
	cdb := lostResponseDB(t, db, pool)
	client := &stubPushClient{}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	svc := NewAchievementsService(
		daos.NewAchievementDao(cdb), daos.NewGamificationDao(cdb),
		&fakeStreakView{}, tokens, client,
	)

	// El batch se ejecuta y su respuesta se pierde; el fallback por logro no
	// encuentra filas nuevas y no duplica nada.
	require.NoError(t, svc.EvaluateEventWithSnapshot(nil, athleteID, AchievementTriggerRunnerSession, &StreakSnapshot{}))
	require.True(t, pool.injected, "el test debe haber simulado la respuesta perdida")

	countUnlocks := func() int64 {
		t.Helper()
		var n int64
		require.NoError(t, db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ?", athleteID).Count(&n).Error)
		return n
	}
	countUnseen := func() int64 {
		t.Helper()
		var n int64
		require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section = ?",
			athleteID, constants.GamificationSectionAchievements).Count(&n).Error)
		return n
	}

	assert.Equal(t, int64(2), countUnlocks(), "first_session y sessions_5")
	assert.Equal(t, int64(2), countUnseen(), "el unseen quedó durable pese al error de respuesta")

	// Re-evaluación: idempotente, sin avisos ni filas nuevas.
	require.NoError(t, svc.EvaluateEventWithSnapshot(nil, athleteID, AchievementTriggerRunnerSession, &StreakSnapshot{}))
	assert.Equal(t, int64(2), countUnlocks())
	assert.Equal(t, int64(2), countUnseen())

	// Limitación residual: el push no es durable y el service no pudo saber
	// qué IDs eran nuevos tras el error; no se inventan pushes para unlocks
	// viejos. El punto rojo (unseen) sí está garantizado.
	assert.Empty(t, client.calls)
}
