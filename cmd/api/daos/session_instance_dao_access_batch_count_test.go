package daos

import (
	"database/sql"
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/testutils"
)

type countingConnPool struct {
	underlying gorm.ConnPool
	queries    int64
}

func (p *countingConnPool) count() { atomic.AddInt64(&p.queries, 1) }

func (p *countingConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	p.count()
	return p.underlying.QueryContext(ctx, query, args...)
}

func (p *countingConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	p.count()
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *countingConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	p.count()
	return p.underlying.QueryRowContext(ctx, query, args...)
}

func (p *countingConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func countingDB(db *gorm.DB) (*gorm.DB, *int64) {
	pool := &countingConnPool{underlying: db.Statement.ConnPool}
	sess := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	sess.Statement.ConnPool = pool
	return sess, &pool.queries
}

func TestSessionInstanceDao_HasInstanceAccessBatch_ConstantQueries(t *testing.T) {
	db := testutils.SetupTestDB(t)
	f := accessFixtureSetup(t, db)
	counting, counter := countingDB(db)
	dao := NewSessionInstanceDao(counting)

	// Todos resueltos por la rama de día: 1 sola query, sin escalado.
	access, err := dao.HasInstanceAccessBatch(nil, f.instDay.ID, []int64{f.athlete.ID, f.owner.ID})
	require.NoError(t, err)
	require.Len(t, access, 2)
	assert.True(t, access[f.athlete.ID])
	assert.True(t, access[f.owner.ID])
	assert.Equal(t, int64(1), *counter, "día resuelve todo en 1 query")

	// Mixto día/feedback/competencia: 3 queries como mínimo posible.
	*counter = 0
	access, err = dao.HasInstanceAccessBatch(nil, 999999, []int64{f.athlete.ID, f.stranger.ID})
	require.NoError(t, err)
	assert.Len(t, access, 2)
	assert.Equal(t, int64(3), *counter, "sin acceso: día + feedback + competencia")
}
