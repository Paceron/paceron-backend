package services

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// countingConnPool cuenta las lecturas que pasan por s.db (los DAOs de
// auth/calendario del service van por mock, así que todo lo que cuenta acá
// son los selects del detalle de instancia).
type countingConnPool struct {
	underlying gorm.ConnPool
	queries    *int
}

func (p *countingConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func (p *countingConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *countingConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	*p.queries++
	return p.underlying.QueryContext(ctx, query, args...)
}

func (p *countingConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	*p.queries++
	return p.underlying.QueryRowContext(ctx, query, args...)
}

func (p *countingConnPool) BeginTx(ctx context.Context, opt *sql.TxOptions) (*sql.Tx, error) {
	return nil, gorm.ErrInvalidTransaction
}

func countingDB(t *testing.T, db *gorm.DB, counter *int) *gorm.DB {
	t.Helper()
	sess := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	sess.Statement.ConnPool = &countingConnPool{underlying: db.Statement.ConnPool, queries: counter}
	return sess
}

// prefs: 2.4 — GetRange de N días con instancia debe tirar exactamente 3
// queries de detalle (instancias IN + links IN + ejercicios IN), ninguna
// FindByID por fila; los días sin instancia no agregan queries.
func TestCalendarService_GetRange_BatchDetalleQueries(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)

	instances := make([]dbs.SessionInstance, 3)
	days := make([]dbs.GroupCalendarDay, 0, 4)
	base := time.Now().AddDate(0, 0, 30)
	for i := 0; i < 3; i++ {
		inst := &dbs.SessionInstance{Name: fmt.Sprintf("S batch %d", i)}
		require.NoError(t, sessionDao.Create(nil, inst))
		instances[i] = *inst
		ex := &dbs.ExerciseInstance{Name: fmt.Sprintf("E batch %d", i), Kind: "running"}
		require.NoError(t, exerciseDao.Create(nil, ex))
		require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{
			SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main",
			RepeatCount: 2, RestMinutes: 60,
		}))
		id := inst.ID
		days = append(days, dbs.GroupCalendarDay{
			GroupID: 7, Date: base.AddDate(0, 0, i), Kind: "training",
			SessionInstanceID: &id, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}
	days = append(days, dbs.GroupCalendarDay{
		GroupID: 7, Date: base.AddDate(0, 0, 3), Kind: "rest",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return days, nil
	}}
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: 7, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 1, OwnerID: 9}, nil
	}}

	counter := 0
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, countingDB(t, db, &counter))

	responses, err := svc.GetRange(nil, 7, 9, base, base.AddDate(0, 0, 3))

	require.NoError(t, err)
	require.Len(t, responses, 4)
	for i := 0; i < 3; i++ {
		resp := responses[i]
		assert.Equal(t, base.AddDate(0, 0, i).Format("2006-01-02"), resp.Date)
		require.NotNil(t, resp.SessionInstance)
		assert.Equal(t, fmt.Sprintf("S batch %d", i), resp.SessionInstance.Name)
		assert.Equal(t, instances[i].ID, resp.SessionInstance.ID)
		require.Len(t, resp.SessionInstance.Exercises, 1)
		assert.Equal(t, fmt.Sprintf("E batch %d", i), resp.SessionInstance.Exercises[0].Name)
		assert.Equal(t, "main", resp.SessionInstance.Exercises[0].Role)
	}
	assert.Nil(t, responses[3].SessionInstance)
	assert.Equal(t, 3, counter, "3 queries batch para 4 días, sin FindByID por fila")
}
