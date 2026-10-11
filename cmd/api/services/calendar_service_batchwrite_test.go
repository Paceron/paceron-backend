package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// txCountingPool cuenta TODAS las queries del service (dentro y fuera de la
// tx): cubre el gorm.ConnPool + Commit/Rollback de TxCommitter. El BeginTx
// inválido replica el comportamiento de countingConnPool: sobre handle de
// transacción (SetupTestDB), la Transaction del service usa SavePoint.
type txCountingPool struct {
	underlying gorm.ConnPool
	committer  gorm.TxCommitter
	queries    *int
}

func (p *txCountingPool) wrap(op string) { *p.queries++ }

func (p *txCountingPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func (p *txCountingPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	p.wrap("exec")
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *txCountingPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	p.wrap("query")
	return p.underlying.QueryContext(ctx, query, args...)
}

func (p *txCountingPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	p.wrap("row")
	return p.underlying.QueryRowContext(ctx, query, args...)
}

func (p *txCountingPool) Commit() error {
	*p.queries++
	return p.committer.Commit()
}

func (p *txCountingPool) Rollback() error {
	*p.queries++
	return p.committer.Rollback()
}

func txCountingDB(t *testing.T, db *gorm.DB, counter *int) *gorm.DB {
	t.Helper()
	pool := &txCountingPool{
		underlying: db.Statement.ConnPool,
		queries:    counter,
	}
	if committer, ok := db.Statement.ConnPool.(gorm.TxCommitter); ok {
		pool.committer = committer
	}
	require.NotNil(t, pool.committer, "el handle de SetupTestDB debe ser TxCommitter")
	sess := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	sess.Statement.ConnPool = pool
	return sess
}

// D6 (task 3) + D7 (task 8): stamp de plan de 4 días training en calendario
// "fresco" con round trips constantes en la tx del write path. Hoy queda
// fijo: 2 selects fuera de la tx (owner/plan etc) + 1 planDays + 1 lectura
// batch de guard (FindByGroupAndDates, task 8) + 3 selects del catálogo +
// 3 inserts de instancias + 1 insert UpsertMany + 1 INSERT savepoint +
// commit + 3 selects post-commit del detalle batch.
// El conteo total ES INDEPENDIENTE del tamaño del plan (ver segunda mitad).
func TestCalendarService_Stamp_BatchWriteQueriesConstantes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Batch", Surname: "W", Email: "batch-write-owner@test.com", DNI: "51009001", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Batch team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Batch group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "Batch exercise", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	sessionA := &dbs.Session{OwnerID: owner.ID, Name: "Batch session A"}
	require.NoError(t, db.Create(sessionA).Error)
	sessionB := &dbs.Session{OwnerID: owner.ID, Name: "Batch session B"}
	require.NoError(t, db.Create(sessionB).Error)
	for _, session := range []*dbs.Session{sessionA, sessionB} {
		require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main", RepeatCount: 2, RestMinutes: 3}).Error)
	}
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Batch plan"}
	require.NoError(t, db.Create(plan).Error)
	aID, bID := sessionA.ID, sessionB.ID
	planDays := []dbs.PlanDay{
		{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &aID},
		{PlanID: plan.ID, SequenceNo: 2, Kind: "training", SessionID: &bID},
		{PlanID: plan.ID, SequenceNo: 3, Kind: "training", SessionID: &aID},
		{PlanID: plan.ID, SequenceNo: 4, Kind: "training", SessionID: &bID},
	}
	require.NoError(t, db.Create(&planDays).Error)

	counter := 0
	counting := txCountingDB(t, db, &counter)
	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(counting), daos.NewGroupDao(counting), daos.NewTeamDao(counting),
		daos.NewGroupUserDao(counting), nil, daos.NewTrainingPlanDao(counting), daos.NewPlanDayDao(counting), nil, counting,
	)

	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: time.Now().AddDate(0, 0, 60).Format("2006-01-02")})

	require.NoError(t, err)
	require.Len(t, resp.Days, 4)
	dayQueries := counter
	t.Logf("stamp 4 días: %d queries", dayQueries)
	// El conteo no crece con el tamaño del plan: 30 días training con el
	// mismo catálogo dan exactamente el mismo número de round trips.
	counter = 0
	bigPlan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Batch plan 30"}
	require.NoError(t, db.Create(bigPlan).Error)
	bigDays := make([]dbs.PlanDay, 30)
	for i := 0; i < 30; i++ {
		bigDays[i] = dbs.PlanDay{PlanID: bigPlan.ID, SequenceNo: i + 1, Kind: "training", SessionID: &aID}
	}
	require.NoError(t, db.Create(&bigDays).Error)

	resp, err = svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: bigPlan.ID, StartDate: time.Now().AddDate(0, 0, 120).Format("2006-01-02")})
	require.NoError(t, err)
	require.Len(t, resp.Days, 30)
	t.Logf("stamp 30 días: %d queries", counter)

	// El WRITE path es constante y ya sin lecturas per-day (D7, task 8): el
	// guard de todas las fechas es una única SELECT IN dentro de la tx,
	// evaluada en memoria con el mismo orden/mensajes que el loop por fecha.
	//
	// 16 fijo: 2 (owner/plan) + 1 planDays + 1 savepoint + 1 lecturas de
	// guard batch + 3 catálogo + 3 inserts instancias + 1 UpsertMany +
	// 3 detalle post-commit + 1 commit.
	assert.Equal(t, 16, dayQueries, "stamp 4 días: sin lectura guard +1/día")
	assert.Equal(t, dayQueries, counter, "stamp de 30 días = exactamente el mismo write path (sin término lineal)")

	// Contenido batch correcto: cada día tiene su propia instancia con links.
	days := make([]dbs.GroupCalendarDay, 0)
	start := time.Now().AddDate(0, 0, 120).UTC()
	require.NoError(t, db.Where("group_id = ? AND date >= ? AND date <= ?", group.ID, start, start.AddDate(0, 0, 29)).Order("date").Find(&days).Error)
	require.Len(t, days, 30)
	ids := make(map[int64]bool, 30)
	for _, day := range days {
		require.NotNil(t, day.SessionInstanceID)
		assert.False(t, ids[*day.SessionInstanceID], "una instancia por día, sin dedup")
		ids[*day.SessionInstanceID] = true
		var instance dbs.SessionInstance
		require.NoError(t, db.First(&instance, *day.SessionInstanceID).Error)
		assert.Equal(t, sessionA.Name, instance.Name)
		var links []dbs.SessionExerciseInstance
		require.NoError(t, db.Where("session_instance_id = ?", instance.ID).Find(&links).Error)
		require.Len(t, links, 1)
	}
}

// D7 (task 8): bulk de N días también queda constante — la lectura de guard
// de todas las fechas es una única SELECT IN dentro de la tx, y los guards
// (missing training sin instancia, cerrado, transición cancel) se evalúan en
// memoria en el mismo orden que la lectura por fecha.
func TestCalendarService_Bulk_BatchWriteQueriesConstantes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbulktime")
	dao := daos.NewGroupCalendarDayDao(db)

	counter := 0
	counting := txCountingDB(t, db, &counter)
	svc := NewCalendarService(dao, daos.NewGroupDao(counting), daos.NewTeamDao(counting),
		daos.NewGroupUserDao(counting), nil, nil, nil, nil, counting)

	base := time.Now().AddDate(0, 0, 210)
	fechas := func(cantidad int) []string {
		out := make([]string, 0, cantidad)
		for i := 0; i < cantidad; i++ {
			out = append(out, base.AddDate(0, 0, i).Format("2006-01-02"))
		}
		return out
	}

	resp, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: fechas(3), Kind: "rest"})
	require.NoError(t, err)
	require.Len(t, resp.Days, 3)
	dayQueries := counter
	t.Logf("bulk 3 días: %d queries", dayQueries)

	counter = 0
	resp, err = svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: fechas(10), Kind: "rest"})
	require.NoError(t, err)
	require.Len(t, resp.Days, 10)
	t.Logf("bulk 10 días: %d queries", counter)
	assert.Equal(t, dayQueries, counter, "bulk de 10 días = mismo write path constante (sin término lineal por fecha)")
	// 5 fijo (el commit del pool de conteo se incrementa aparte): 2 selects
	// del owner (grupo+equipo) + 1 savepoint + 1 lectura de guard batch +
	// 1 UpsertMany; días rest sin instancia → el detalle post-commit no agrega.
	assert.Equal(t, 5, dayQueries, "bulk 3 días: 2 owner + savepoint + guard batch + UpsertMany")
}
