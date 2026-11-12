package services

// Cierre de competencias (Task 4, D5): con DAO real y pool que cuenta lecturas,
// winners y respuesta comparten la MISMA lectura del request — metric agrega
// una vez y no lista intentos; route lista sesiones/puntos una vez. Se fija la
// paridad de winners/progress/stamps/unseen además del conteo de queries.

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// countingNestedPool cuenta lecturas (Query/QueryRow) y expone Commit/Rollback
// para que gorm trate el Close como transacción anidada (SAVEPOINT) sobre el tx
// de test. Las escrituras van por Exec y no afectan el conteo de lecturas.
type countingNestedPool struct {
	underlying gorm.ConnPool
	queries    *int
}

func (p *countingNestedPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.underlying.PrepareContext(ctx, query)
}

func (p *countingNestedPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.underlying.ExecContext(ctx, query, args...)
}

func (p *countingNestedPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	p.count(query)
	return p.underlying.QueryContext(ctx, query, args...)
}

func (p *countingNestedPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	p.count(query)
	return p.underlying.QueryRowContext(ctx, query, args...)
}

// count suma solo SELECT: los INSERT ... RETURNING de gorm también van por
// Query y no son lecturas del cierre.
func (p *countingNestedPool) count(query string) {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
		*p.queries++
	}
}

func (p *countingNestedPool) Commit() error {
	if c, ok := p.underlying.(interface{ Commit() error }); ok {
		return c.Commit()
	}
	return nil
}

func (p *countingNestedPool) Rollback() error {
	if r, ok := p.underlying.(interface{ Rollback() error }); ok {
		return r.Rollback()
	}
	return nil
}

func countingNestedDB(t *testing.T, db *gorm.DB, counter *int) *gorm.DB {
	t.Helper()
	sess := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true})
	sess.Statement.ConnPool = &countingNestedPool{underlying: db.Statement.ConnPool, queries: counter}
	return sess
}

func seedMetricFeedback(t *testing.T, db *gorm.DB, teamID, athleteID int64, distance float64, sessionDate time.Time) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID: &teamID, AssignedSessionID: 1000, AssignedExerciseID: 1,
		AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID, ReportSource: "corredor",
		SessionDate: sessionDate, SetNumber: 1, DistanceMeters: &distance, MediaURLs: media,
	}).Error)
}

func TestCompetitionService_CloseMetric_QueryCountYParidad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fx := attemptsFlowSetup(t, db, "cqm4")
	compSvc := NewCompetitionService(daos.NewCompetitionDao(db), nil, nil)

	end := gametime.DateOnly(time.Now()).Add(-time.Hour)
	start := end.AddDate(0, 0, -1)
	metric := "distance"
	created, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
		Type: competition.TypeMetric, Metric: &metric, StartAt: start, EndAt: &end,
		ParticipantIDs: []int64{fx.runnerA.ID, fx.runnerB.ID},
	})
	require.NoError(t, err)

	// Empate exacto: mismos metros → ambos ganan, stamps para ambos.
	seedMetricFeedback(t, db, fx.team.ID, fx.runnerA.ID, 5000, end)
	seedMetricFeedback(t, db, fx.team.ID, fx.runnerB.ID, 5000, end)

	queries := 0
	countingSvc := NewCompetitionService(daos.NewCompetitionDao(countingNestedDB(t, db, &queries)), nil, nil)
	resp, instances, err := countingSvc.Close(nil, fx.owner.ID, created.ID)
	require.NoError(t, err)

	// bundle (FindByID+participants+winners) + owner + UNA agregación metric.
	const expected = 5
	assert.Equal(t, expected, queries, "metric: una sola agregación y sin listar intentos")

	assert.Equal(t, competition.StatusFinished, resp.Status)
	assert.Equal(t, []int64{fx.runnerA.ID, fx.runnerB.ID}, resp.WinnerUserIDs)
	require.Len(t, resp.Progress, 2)
	for i, uid := range []int64{fx.runnerA.ID, fx.runnerB.ID} {
		item, ok := resp.Progress[i].(competition.CompetitionMetricProgressItem)
		require.True(t, ok)
		assert.Equal(t, uid, item.UserID)
		require.NotNil(t, item.Value)
		assert.Equal(t, 5000.0, *item.Value)
	}
	assert.Empty(t, instances, "metric no registra intentos ni canales WS")

	var stamps, unseen int64
	require.NoError(t, db.Model(&dbs.GamificationStamp{}).Where("source_id = ?", created.ID).Count(&stamps).Error)
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("team_id = ? AND section = ?",
		fx.team.ID, constants.GamificationSectionCompetitions).Count(&unseen).Error)
	assert.Equal(t, int64(2), stamps)
	assert.Equal(t, int64(2), unseen)
}

func TestCompetitionService_CloseRoute_QueryCountYParidad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fx := attemptsFlowSetup(t, db, "cqr4")
	compDao := daos.NewCompetitionDao(db)
	compSvc := NewCompetitionService(compDao, nil, nil)
	runnerDao := daos.NewRunnerSessionDao(db)

	created, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
		Type: competition.TypeRoute, RouteGeoJSON: strPtr(routeFlowGeoJSON()),
		AcceptanceRadiusMeters: fptr(50), StartAt: time.Now().AddDate(0, 0, -2),
		ParticipantIDs: []int64{fx.runnerA.ID, fx.runnerB.ID},
	})
	require.NoError(t, err)
	comp, err := compDao.FindByID(nil, created.ID)
	require.NoError(t, err)

	// Ambos intentos finished sobre la instancia compartida, con match pleno y
	// la misma duración: empate exacto → dos winners.
	base := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	for _, athlete := range []*dbs.User{fx.runnerA, fx.runnerB} {
		rs, err := compDao.CreateAttemptSession(nil, comp, athlete.ID)
		require.NoError(t, err)
		require.NoError(t, runnerDao.UpdateStatus(nil, rs.ID, runnersession.RunnerSessionStatusFinished,
			[]string{runnersession.RunnerSessionStatusWip, runnersession.RunnerSessionStatusInterrupted}, time.Now()))
		pinAttemptSpan(t, db, rs.ID, base, base.Add(5*time.Minute))
		attachAttemptEvidence(t, db, athlete.ID, rs.SessionInstanceID, &fx.team.ID, 400, 11)
	}

	queries := 0
	countingSvc := NewCompetitionService(daos.NewCompetitionDao(countingNestedDB(t, db, &queries)), nil, nil)
	resp, instances, err := countingSvc.Close(nil, fx.owner.ID, created.ID)
	require.NoError(t, err)

	// bundle + owner + sesiones + puntos (una vez) + counts del caller.
	const expected = 7
	assert.Equal(t, expected, queries, "route: intentos leídos una sola vez para winners y progress")

	assert.Equal(t, []int64{fx.runnerA.ID, fx.runnerB.ID}, resp.WinnerUserIDs)
	require.Len(t, resp.Progress, 2)
	for i, uid := range []int64{fx.runnerA.ID, fx.runnerB.ID} {
		item, ok := resp.Progress[i].(competition.CompetitionRouteProgressItem)
		require.True(t, ok)
		assert.Equal(t, uid, item.UserID)
		require.NotNil(t, item.ComplianceRatio)
		assert.Equal(t, 100.0, *item.ComplianceRatio)
		require.NotNil(t, item.DurationMs)
		assert.Equal(t, int64(5*time.Minute/time.Millisecond), *item.DurationMs)
		require.NotNil(t, item.SessionInstanceID)
		assert.Equal(t, comp.RouteSessionInstanceID, item.SessionInstanceID)
	}
	require.Len(t, instances, 1, "ambos intentos comparten la instancia sintética")

	var stamps, unseen int64
	require.NoError(t, db.Model(&dbs.GamificationStamp{}).Where("source_id = ?", created.ID).Count(&stamps).Error)
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("team_id = ? AND section = ?",
		fx.team.ID, constants.GamificationSectionCompetitions).Count(&unseen).Error)
	assert.Equal(t, int64(2), stamps)
	assert.Equal(t, int64(2), unseen)
}

func TestCompetitionService_CloseRoute_SinIntentos_QueryCountYParidad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fx := attemptsFlowSetup(t, db, "cqrn4")
	compSvc := NewCompetitionService(daos.NewCompetitionDao(db), nil, nil)

	created, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
		Type: competition.TypeRoute, RouteGeoJSON: strPtr(routeFlowGeoJSON()),
		AcceptanceRadiusMeters: fptr(50), StartAt: time.Now().AddDate(0, 0, -2),
		ParticipantIDs: []int64{fx.runnerA.ID, fx.runnerB.ID},
	})
	require.NoError(t, err)

	queries := 0
	countingSvc := NewCompetitionService(daos.NewCompetitionDao(countingNestedDB(t, db, &queries)), nil, nil)
	resp, instances, err := countingSvc.Close(nil, fx.owner.ID, created.ID)
	require.NoError(t, err)

	// bundle + owner + listado de intentos vacío (sin puntos/samples) + counts.
	const expected = 6
	assert.Equal(t, expected, queries, "sin intentos no se leen puntos ni muestras")

	assert.Empty(t, resp.WinnerUserIDs)
	assert.Empty(t, instances)
	require.Len(t, resp.Progress, 2)
	for i, uid := range []int64{fx.runnerA.ID, fx.runnerB.ID} {
		item, ok := resp.Progress[i].(competition.CompetitionRouteProgressItem)
		require.True(t, ok)
		assert.Equal(t, uid, item.UserID)
		assert.Nil(t, item.ComplianceRatio)
		assert.Nil(t, item.DurationMs)
		assert.Nil(t, item.SessionInstanceID)
	}

	var stamps, unseen int64
	require.NoError(t, db.Model(&dbs.GamificationStamp{}).Where("source_id = ?", created.ID).Count(&stamps).Error)
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("team_id = ? AND section = ?",
		fx.team.ID, constants.GamificationSectionCompetitions).Count(&unseen).Error)
	assert.Zero(t, stamps)
	assert.Equal(t, int64(2), unseen)
}
