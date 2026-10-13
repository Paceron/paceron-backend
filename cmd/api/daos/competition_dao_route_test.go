package daos

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Escenario DAO de la instancia sintética de competencias route y del intake
// de intentos (competencias-ruta-intake D1/D2) contra Postgres real.

func newRouteCompetition() *dbs.Competition {
	route := `{"type":"LineString","coordinates":[[-64.18,-31.42],[-64.19,-31.43]]}`
	radius := 50.0
	return &dbs.Competition{
		Type:                   competition.TypeRoute,
		RouteGeoJSON:           &route,
		AcceptanceRadiusMeters: &radius,
		StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestCompetitionDAO_CreateWithInstanceRoute(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt01")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	require.NotZero(t, comp.ID)
	require.NotNil(t, comp.RouteSessionInstanceID)
	instanceID := *comp.RouteSessionInstanceID

	// Roundtrip: la instancia + ejercicio + link quedaron en sus tablas.
	var sessionInstance dbs.SessionInstance
	require.NoError(t, db.Where("id = ?", instanceID).First(&sessionInstance).Error)
	assert.Equal(t, fmt.Sprintf("Competencia ruta %d", comp.ID), sessionInstance.Name)
	require.NotNil(t, sessionInstance.Description)
	assert.Equal(t, "Instancia sintética de la competencia de ruta", *sessionInstance.Description)
	assert.Nil(t, sessionInstance.SourceSessionID)

	var links []dbs.SessionExerciseInstance
	require.NoError(t, db.Where("session_instance_id = ?", instanceID).Find(&links).Error)
	require.Len(t, links, 1)
	assert.Equal(t, string(constants.SessionExerciseRoleMain), links[0].Role)
	assert.Equal(t, 1, links[0].RepeatCount)
	assert.Equal(t, 0, links[0].RestMinutes)

	var exerciseInstance dbs.ExerciseInstance
	require.NoError(t, db.Where("id = ?", links[0].ExerciseInstanceID).First(&exerciseInstance).Error)
	assert.Equal(t, "Corrida", exerciseInstance.Name)
	assert.Equal(t, string(constants.ExerciseKindVelocidadDistancia), exerciseInstance.Kind)
	assert.Nil(t, exerciseInstance.SourceExerciseID)

	// La competencia persistió la FK opaca; sin día de calendario referenciándola.
	var dayCount int64
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("session_instance_id = ?", instanceID).Count(&dayCount).Error)
	assert.Zero(t, dayCount)

	parts, err := dao.ListParticipantsByCompetitionIDs(nil, []int64{comp.ID})
	require.NoError(t, err)
	require.Len(t, parts, 2)
}

func TestCompetitionDAO_CreateWithInstanceMetric(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt02")

	comp := newRouteCompetition()
	comp.Type = competition.TypeMetric
	comp.Metric = strPtr("distance")
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.runnerA.ID
	comp.CreatedByRole = competition.CreatorRoleRunner
	comp.Status = competition.StatusPendingApproval
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	assert.Nil(t, comp.RouteSessionInstanceID)

	byName := fmt.Sprintf("Competencia ruta %d", comp.ID)
	var count int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Where("name = ?", byName).Count(&count).Error)
	assert.Zero(t, count, "metric no crea instancia sintética")

	parts, err := dao.ListParticipantsByCompetitionIDs(nil, []int64{comp.ID})
	require.NoError(t, err)
	require.Len(t, parts, 2)
}

// Rollback del tx (D1): si falla la instancia (ej. participantes viola el PK
// compuesto), no queda ni la competencia ni las piezas de instancia.
func TestCompetitionDAO_CreateWithInstanceRollback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt03")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive

	err := dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerA.ID})
	require.Error(t, err, "participante duplicado viola el PK compuesto")

	_, err = dao.FindByID(nil, comp.ID)
	require.ErrorIs(t, err, ErrCompetitionNotFound, "la competencia no queda huérfana tras el rollback")

	// Nada con el nombre de esa competencia (la instancia también se revirtió).
	var nameCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).
		Where("name LIKE ?", "Competencia ruta %").Count(&nameCount).Error)
	assert.Zero(t, nameCount)
}

// Intake: ExistsAttempt consulta la marca competition_id de runner_session.
func TestCompetitionDAO_ExistsAttemptByCompetitionAndAthlete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt04")
	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	instanceID := *comp.RouteSessionInstanceID

	exists, err := dao.ExistsAttemptByCompetitionAndAthlete(nil, comp.ID, fx.runnerA.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	// Doble intento e2e: el segundo insert viola el unique
	// uq_runner_session_session_athlete y el ON CONFLICT del DAO devuelve
	// created=false (respaldo del unique, D2).
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: instanceID, AthleteUserID: fx.runnerA.ID,
		StartDate: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC), CompetitionID: &comp.ID,
	}).Error)
	exists, err = dao.ExistsAttemptByCompetitionAndAthlete(nil, comp.ID, fx.runnerA.ID)
	require.NoError(t, err)
	assert.True(t, exists)

	runnerDao := NewRunnerSessionDao(db)
	second := &dbs.RunnerSession{
		SessionInstanceID: instanceID, AthleteUserID: fx.runnerA.ID,
		StartDate: time.Date(2026, 10, 2, 9, 5, 0, 0, time.UTC), CompetitionID: &comp.ID,
	}
	created, err := runnerDao.Create(nil, second)
	require.NoError(t, err)
	assert.False(t, created, "el unique arbitra el doble intento: no inserta ni falla")

	// Otra atleta sigue sin intento.
	exists, err = dao.ExistsAttemptByCompetitionAndAthlete(nil, comp.ID, fx.runnerB.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	// Un insert plano concurrente viola el unique: así es el respaldo de la
	// constraint contra la carrera entre el pre-check y el write (D2).
	var raceErr error
	clean := *second
	clean.ID = 0
	if e := db.Create(&clean).Error; e != nil {
		raceErr = e
	}
	require.Error(t, raceErr)
}
