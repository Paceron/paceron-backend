package daos

// CountAttemptsByCompetitionAndAthlete (Gap 36 D7) contra Postgres real: una
// sola query batch devuelve competencia→cantidad de intentos (cualquier
// status) con fila para todo participante (count 0 sin intentos) y ausencia
// de fila para el no-participante.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestCompetitionDAO_CountAttemptsByCompetitionAndAthlete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "cnt01")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))

	// A: 2 intentos (uno wip, uno terminado — ambos cuentan). B: ninguno.
	rs1, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.NoError(t, err)
	rs1.Status = competition.StatusFinished
	require.NoError(t, db.Save(&rs1).Error)
	_, err = dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.NoError(t, err)

	// Batch de una sola query para A: participante con intentos = 2.
	counts, err := dao.CountAttemptsByCompetitionAndAthlete(nil, []int64{comp.ID}, fx.runnerA.ID)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{comp.ID: 2}, counts)

	// B: participante sin intentos = 0 (fila del LEFT JOIN), no ausencia.
	counts, err = dao.CountAttemptsByCompetitionAndAthlete(nil, []int64{comp.ID}, fx.runnerB.ID)
	require.NoError(t, err)
	require.Contains(t, counts, comp.ID)
	assert.Equal(t, int64(0), counts[comp.ID])

	// A + una métrica del mismo team: batch coherente en una sola query.
	metric := newMetricCompetition(t, db, fx, competition.StatusActive, "cnt01", nil)
	rsMetric := dbs.RunnerSession{
		SessionInstanceID: 79001, AthleteUserID: fx.runnerA.ID,
		StartDate: metric.StartAt, CompetitionID: &metric.ID,
		Status: competition.StatusActive,
	}
	require.NoError(t, db.Create(&rsMetric).Error)

	counts, err = dao.CountAttemptsByCompetitionAndAthlete(nil, []int64{comp.ID, metric.ID}, fx.runnerA.ID)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{comp.ID: 2, metric.ID: 1}, counts)

	// Outsider: no participante de ninguna → map vacío.
	counts, err = dao.CountAttemptsByCompetitionAndAthlete(nil, []int64{comp.ID, metric.ID}, fx.outsider.ID)
	require.NoError(t, err)
	assert.Empty(t, counts)

	// Lista vacía: sin query, map vacío.
	counts, err = dao.CountAttemptsByCompetitionAndAthlete(nil, nil, fx.runnerA.ID)
	require.NoError(t, err)
	assert.Empty(t, counts)
}
