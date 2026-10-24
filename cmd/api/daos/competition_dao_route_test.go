package daos

import (
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

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

// max_attempts (Gap 36): el round-trip Postgres persiste/lee el valor (o el
// NULL = sin límite) íntegro; una competencia pre-change sin columna seteada
// se lee null.
func TestCompetitionDAO_MaxAttemptsRoundTrip(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "ma01")

	withMax := int64(3)
	limited := newRouteCompetition()
	limited.TeamID = fx.team.ID
	limited.CreatedByUserID = fx.owner.ID
	limited.CreatedByRole = competition.CreatorRoleTrainer
	limited.Status = competition.StatusActive
	limited.MaxAttempts = &withMax
	require.NoError(t, dao.CreateWithInstance(nil, limited, []int64{fx.runnerA.ID, fx.runnerB.ID}))

	loaded, err := dao.FindByID(nil, limited.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.MaxAttempts)
	assert.Equal(t, withMax, *loaded.MaxAttempts)

	unlimited := newRouteCompetition()
	unlimited.TeamID = fx.team.ID
	unlimited.CreatedByUserID = fx.runnerA.ID
	unlimited.CreatedByRole = competition.CreatorRoleRunner
	unlimited.Status = competition.StatusPendingApproval
	require.NoError(t, dao.CreateWithInstance(nil, unlimited, []int64{fx.runnerA.ID, fx.runnerB.ID}))

	loaded, err = dao.FindByID(nil, unlimited.ID)
	require.NoError(t, err)
	assert.Nil(t, loaded.MaxAttempts, "sin max_attempts = NULL")
}

// FindByRouteSessionInstance es la mitad inversa del guard de anclaje del
// intento (review extra I1): resuelve la dueña de la instancia sintética, y
// devuelve not-found para instancias que no anclan a ninguna route.
func TestCompetitionDAO_FindByRouteSessionInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt05")

	routeComp := newRouteCompetition()
	routeComp.TeamID = fx.team.ID
	routeComp.CreatedByUserID = fx.owner.ID
	routeComp.CreatedByRole = competition.CreatorRoleTrainer
	routeComp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, routeComp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	routeInst := *routeComp.RouteSessionInstanceID

	metricComp := newRouteCompetition()
	metricComp.Type = competition.TypeMetric
	metricComp.Metric = strPtr("distance")
	metricComp.TeamID = fx.team.ID
	metricComp.CreatedByUserID = fx.owner.ID
	metricComp.CreatedByRole = competition.CreatorRoleTrainer
	metricComp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, metricComp, []int64{fx.runnerA.ID, fx.runnerB.ID}))

	owner, err := dao.FindByRouteSessionInstance(nil, routeInst)
	require.NoError(t, err)
	assert.Equal(t, routeComp.ID, owner.ID)
	assert.Equal(t, competition.TypeRoute, owner.Type)

	// Ni una id inexistente ni la fk opaca vacía de las metric resuelven dueña.
	_, err = dao.FindByRouteSessionInstance(nil, 999999)
	require.ErrorIs(t, err, ErrCompetitionNotFound)
}

// ListAttemptsByCompetitionAndAthlete (Gap 36 D4): solo los intentos del
// atleta pedido en la competencia, orden id ASC (cronológico = intento 1
// primero) con cualquier status.
func TestCompetitionDAO_ListAttemptsByCompetitionAndAthlete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt06")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	sharedInst := *comp.RouteSessionInstanceID

	inst2 := &dbs.SessionInstance{Name: "Intento 2 de rt06"}
	require.NoError(t, db.Create(inst2).Error)
	end := time.Date(2026, 10, 3, 10, 30, 0, 0, time.UTC)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: inst2.ID, AthleteUserID: fx.runnerB.ID,
		Status: "finished", StartDate: time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC), EndDate: &end,
		CompetitionID: &comp.ID,
	}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: sharedInst, AthleteUserID: fx.runnerA.ID,
		Status: "wip", StartDate: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
		CompetitionID: &comp.ID,
	}).Error)

	rows, err := dao.ListAttemptsByCompetitionAndAthlete(nil, comp.ID, fx.runnerA.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, sharedInst, rows[0].SessionInstanceID)
	assert.Equal(t, "wip", rows[0].Status)

	finished, err := dao.ListAttemptsByCompetitionAndAthlete(nil, comp.ID, fx.runnerB.ID)
	require.NoError(t, err)
	require.Len(t, finished, 1)

	// Otro atleta con 2 intentos: el orden queda por id ASC.
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: sharedInst, AthleteUserID: fx.runnerB.ID,
		Status: "finished", StartDate: end, EndDate: func() *time.Time { e := end.Add(time.Hour); return &e }(),
		CompetitionID: &comp.ID,
	}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: inst2.ID, AthleteUserID: fx.runnerA.ID,
		Status: "wip", StartDate: end, CompetitionID: &comp.ID,
	}).Error)

	both, err := dao.ListAttemptsByCompetitionAndAthlete(nil, comp.ID, fx.runnerB.ID)
	require.NoError(t, err)
	require.Len(t, both, 2)
	assert.Less(t, both[0].ID, both[1].ID)
	assert.Equal(t, inst2.ID, both[0].SessionInstanceID)
	assert.Equal(t, sharedInst, both[1].SessionInstanceID)

	// Sin intentos en la competencia = lista vacía, sin error.
	empty, err := dao.ListAttemptsByCompetitionAndAthlete(nil, comp.ID, fx.outsider.ID)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// sumFeedbackRow crea un workout_feedback de sum distance (feedback "de route"
// con set/distance) para los tests de agregación; deleted simula soft-delete.
func sumFeedbackRow(t *testing.T, db *gorm.DB, athleteID, reporterID int64, teamID *int64, instanceID int64, setNumber int, distance float64, deleted bool) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	fb := &dbs.WorkoutFeedback{
		TeamID: teamID, AssignedSessionID: instanceID, AssignedExerciseID: 1,
		AthleteUserID: athleteID, FeedbackOwnerUserID: reporterID,
		ReportSource: "corredor", SessionDate: time.Now(), SetNumber: setNumber,
		DistanceMeters: &distance, MediaURLs: media,
	}
	if deleted {
		now := time.Now()
		fb.DeletedAt = &now
	}
	require.NoError(t, db.Create(fb).Error)
}

// SumFeedbackDistanceByInstances (Gap 36 D4): suma distance_meters del
// feedback activo del atleta por instancia; soft-deleted no suma; instancia
// sin feedback no aparece (el service deja el campo null).
func TestCompetitionDAO_SumFeedbackDistanceByInstances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "rt07")

	inst1 := &dbs.SessionInstance{Name: "Sum inst1"}
	inst2 := &dbs.SessionInstance{Name: "Sum inst2"}
	inst3 := &dbs.SessionInstance{Name: "Sum inst3"}
	require.NoError(t, db.Create(&[]*dbs.SessionInstance{inst1, inst2, inst3}).Error)

	teamID := fx.team.ID
	// inst1: dos series activas del atleta (100 + 50).
	sumFeedbackRow(t, db, fx.runnerA.ID, fx.runnerA.ID, &teamID, inst1.ID, 1, 100, false)
	sumFeedbackRow(t, db, fx.runnerA.ID, fx.runnerA.ID, &teamID, inst1.ID, 2, 50, false)
	// inst2: único feedback soft-deleted → no suma (no hay fila).
	sumFeedbackRow(t, db, fx.runnerA.ID, fx.runnerA.ID, &teamID, inst2.ID, 1, 999, true)
	// Otro atleta sobre inst1: no entra al atleta pedido.
	sumFeedbackRow(t, db, fx.runnerB.ID, fx.runnerB.ID, &teamID, inst1.ID, 1, 777, false)

	rows, err := dao.SumFeedbackDistanceByInstances(nil, []int64{inst1.ID, inst2.ID, inst3.ID}, fx.runnerA.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, inst1.ID, rows[0].SessionInstanceID)
	require.NotNil(t, rows[0].SumDistance)
	assert.InDelta(t, 150.0, *rows[0].SumDistance, 1e-9)

	// Sin instancias: no toca la DB.
	rows, err = dao.SumFeedbackDistanceByInstances(nil, nil, fx.runnerA.ID)
	require.NoError(t, err)
	assert.Nil(t, rows)
}
