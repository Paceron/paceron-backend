package daos

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/testutils"
)

type runnerKpisFixture struct {
	db             *gorm.DB
	dao            DashboardDaoInterface
	athleteID      int64
	otherAthleteID int64
	ritmoEx        int64
	velTiempoEx    int64
	velDistEx      int64
	from, to       time.Time
}

func newRunnerKpisFixture(t *testing.T, db *gorm.DB, tag string) *runnerKpisFixture {
	t.Helper()
	athlete := persistUser(db, "runner-kpis-"+tag+"@test.com", "8"+tag)
	other := persistUser(db, "runner-kpis-other-"+tag+"@test.com", "9"+tag)

	ritmo := &dbs.ExerciseInstance{Name: "Ritmo " + tag, Kind: string(constants.ExerciseKindRitmoContinuo)}
	velTiempo := &dbs.ExerciseInstance{Name: "Velocidad tiempo " + tag, Kind: string(constants.ExerciseKindVelocidadTiempo)}
	velDist := &dbs.ExerciseInstance{Name: "Velocidad distancia " + tag, Kind: string(constants.ExerciseKindVelocidadDistancia)}
	require.NoError(t, db.Create(ritmo).Error)
	require.NoError(t, db.Create(velTiempo).Error)
	require.NoError(t, db.Create(velDist).Error)

	return &runnerKpisFixture{
		db:             db,
		dao:            NewDashboardDao(db),
		athleteID:      athlete.ID,
		otherAthleteID: other.ID,
		ritmoEx:        ritmo.ID,
		velTiempoEx:    velTiempo.ID,
		velDistEx:      velDist.ID,
		from:           time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		to:             time.Date(2026, time.October, 31, 0, 0, 0, 0, time.UTC),
	}
}

func (f *runnerKpisFixture) persistFeedback(
	t *testing.T,
	athleteID, exerciseInstanceID, sessionInstanceID int64,
	date time.Time,
	distanceM float64,
	teamID *int64,
	deleted bool,
) int64 {
	t.Helper()
	timeMs := int64(600000)
	feedback := &dbs.WorkoutFeedback{
		TeamID:              teamID,
		AssignedSessionID:   sessionInstanceID,
		AssignedExerciseID:  exerciseInstanceID,
		AthleteUserID:       athleteID,
		FeedbackOwnerUserID: athleteID,
		ReportSource:        "atleta",
		SessionDate:         date,
		SetNumber:           1,
		DurationMs:          &timeMs,
		ActiveDurationMs:    &timeMs,
		DistanceMeters:      &distanceM,
		MediaURLs:           emptyMediaURLs(t),
	}
	if deleted {
		deletedAt := date.Add(time.Hour)
		feedback.DeletedAt = &deletedAt
	}
	require.NoError(t, f.db.Create(feedback).Error)
	return feedback.ID
}

func (f *runnerKpisFixture) persistRunnerSession(t *testing.T, athleteID, sessionInstanceID int64, status string) {
	t.Helper()
	require.NoError(t, f.db.Create(&dbs.RunnerSession{
		SessionInstanceID: sessionInstanceID,
		AthleteUserID:     athleteID,
		Status:            status,
		StartDate:         f.from,
	}).Error)
}

// La query del Home parte de workout_feedback y no exige grupo ni equipo:
// incluye dos equipos distintos, un grupo del que el corredor ya no es miembro,
// finished/interrupted y feedback sin runner_session, y deja afuera wip,
// soft-delete, otro corredor y fechas fuera del rango.
func TestDashboardDao_FindRunnerSeries(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newRunnerKpisFixture(t, db, "scope")

	teamOne := testTeam(db, "runner-kpis-team-one", fixture.athleteID)
	teamTwo := testTeam(db, "runner-kpis-team-two", fixture.athleteID)
	// Grupo sin membresía: el dato es del corredor y se incluye igual.
	testGroup(db, "runner-kpis-group-left", teamOne.ID)

	included := []int64{
		fixture.persistFeedback(t, fixture.athleteID, fixture.ritmoEx, 100, fixture.from, 5000, &teamOne.ID, false),
		fixture.persistFeedback(t, fixture.athleteID, fixture.velTiempoEx, 101,
			time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC), 3000, &teamTwo.ID, false),
		fixture.persistFeedback(t, fixture.athleteID, fixture.velDistEx, 102, fixture.to, 2000, &teamOne.ID, false),
	}
	fixture.persistRunnerSession(t, fixture.athleteID, 101, runnersession.RunnerSessionStatusFinished)
	fixture.persistRunnerSession(t, fixture.athleteID, 102, runnersession.RunnerSessionStatusInterrupted)

	fixture.persistFeedback(t, fixture.athleteID, fixture.ritmoEx, 103,
		time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC), 4000, &teamOne.ID, false)
	fixture.persistRunnerSession(t, fixture.athleteID, 103, runnersession.RunnerSessionStatusWip)

	fixture.persistFeedback(t, fixture.athleteID, fixture.ritmoEx, 104,
		time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC), 4000, &teamOne.ID, true)
	fixture.persistFeedback(t, fixture.otherAthleteID, fixture.ritmoEx, 105,
		time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC), 4000, &teamOne.ID, false)
	fixture.persistFeedback(t, fixture.athleteID, fixture.ritmoEx, 106,
		fixture.from.AddDate(0, 0, -1), 4000, &teamOne.ID, false)
	fixture.persistFeedback(t, fixture.athleteID, fixture.ritmoEx, 107,
		fixture.to.AddDate(0, 0, 1), 4000, &teamOne.ID, false)

	rows, err := fixture.dao.FindRunnerSeries(nil, fixture.athleteID, fixture.from, fixture.to)

	require.NoError(t, err)
	require.Len(t, rows, len(included))
	for i, row := range rows {
		assert.Equal(t, included[i], row.FeedbackID, "ordenado por session_date, id")
	}
	assert.Equal(t, fixture.ritmoEx, rows[0].ExerciseInstanceID)
	assert.Equal(t, string(constants.ExerciseKindRitmoContinuo), rows[0].Kind)
	require.NotNil(t, rows[0].DistanceMeters)
	assert.Equal(t, 5000.0, *rows[0].DistanceMeters)
	assert.False(t, rows[0].HasRunnerSession)
	assert.True(t, rows[1].HasRunnerSession)
	assert.Equal(t, string(constants.ExerciseKindVelocidadTiempo), rows[1].Kind)
	assert.Equal(t, "2026-10-01", rows[0].SessionDate.Format("2006-01-02"))
	assert.Equal(t, "2026-10-31", rows[2].SessionDate.Format("2006-01-02"))
}

func TestDashboardDao_FindRunnerSeries_SinDatos(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newRunnerKpisFixture(t, db, "empty")

	rows, err := fixture.dao.FindRunnerSeries(nil, fixture.athleteID, fixture.from, fixture.to)

	require.NoError(t, err)
	assert.Empty(t, rows)
}

// La consulta no filtra por rol ni depende del calendario/grupo: si alguna de
// esas condiciones vuelve, el Home dejaría de sumar warmup/cooldown o el
// histórico de grupos dejados.
func TestDashboardDao_RunnerSeriesQueryNoUsaGruposNiRoles(t *testing.T) {
	assert.NotContains(t, runnerSeriesFromClause, "group_calendar_days")
	assert.NotContains(t, runnerSeriesFromClause, "session_exercise_instances")
	assert.NotContains(t, runnerSeriesFromClause, "role")
	assert.NotContains(t, runnerSeriesFromClause, "SELECT *")
	assert.Contains(t, runnerSeriesFromClause, "wf.deleted_at IS NULL")
	assert.Contains(t, runnerSeriesFromClause, "rs.status IS NULL",
		"el LEFT JOIN tiene que admitir feedback sin runner_session")
}

// La query usa el índice existente (athlete_user_id, session_date); no se agrega
// ninguno nuevo. Con seqscan deshabilitado el plan tiene que nombrarlo.
func TestDashboardDao_RunnerSeriesUsaIndiceAthleteDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newRunnerKpisFixture(t, db, "explain")
	fixture.persistFeedback(t, fixture.athleteID, fixture.ritmoEx, 200, fixture.from, 5000, nil, false)

	require.NoError(t, db.Exec("SET LOCAL enable_seqscan = off").Error)
	query := "EXPLAIN SELECT wf.id" + runnerSeriesFromClause + " ORDER BY wf.session_date, wf.id"
	sqlRows, err := db.Raw(query, fixture.athleteID, fixture.from, fixture.to,
		runnersession.RunnerSessionStatusFinished,
		runnersession.RunnerSessionStatusInterrupted).Rows()
	require.NoError(t, err)
	defer sqlRows.Close()

	var lines []string
	for sqlRows.Next() {
		var line string
		require.NoError(t, sqlRows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, sqlRows.Err())
	assert.Contains(t, strings.Join(lines, "\n"), "idx_feedback_athlete_date")
}
