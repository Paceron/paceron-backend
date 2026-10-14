package main

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// cleanupFixture arma el escenario del §8.4: un scope sembrado por el seed y, al
// lado, filas que NO son del seed (otro grupo, otra fecha, otro entrenador) que el
// cleanup tiene que dejar intactas.
type cleanupFixture struct {
	db             *gorm.DB
	scope          cleanupScope
	seedCatalog    int64
	seedSession    int64
	seedPlan       int64
	seedDay        int64
	seedInstance   int64
	foreignDay     int64
	foreignCatalog int64
}

func newCleanupFixture(t *testing.T, tag string) *cleanupFixture {
	t.Helper()
	db := testutils.SetupTestDB(t)

	owner := cleanupTestUser(t, db, "cleanup-owner-"+tag+"@test.com", "7"+tag)
	athlete := cleanupTestUser(t, db, "cleanup-athlete-"+tag+"@test.com", "8"+tag)

	seedTeam := cleanupTestTeam(t, db, "equipo_cleanup_"+tag, owner.ID)
	seedGroup := cleanupTestGroup(t, db, "grupo_cleanup_"+tag, seedTeam.ID)
	otherTeam := cleanupTestTeam(t, db, "equipo_otro_"+tag, owner.ID)
	otherGroup := cleanupTestGroup(t, db, "grupo_otro_"+tag, otherTeam.ID)

	window := seedWindow{Start: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)}
	inWindow := window.weekStart(0).AddDate(0, 0, planDayTuesday-1)
	outsideWindow := window.to().AddDate(0, 0, 7)

	seedPlan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Semilla dashboard · Semana 1",
		Description: seedDescription("plan")}
	require.NoError(t, db.Create(seedPlan).Error)
	seedSession := &dbs.Session{OwnerID: owner.ID, Name: "Sesión de volumen", Description: seedDescription("sesión")}
	require.NoError(t, db.Create(seedSession).Error)
	seedCatalog := &dbs.Exercise{OwnerID: owner.ID, Name: "Distancia 4 km",
		Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtr(30), DistanceM: intPtr(4000),
		Description: seedDescription("ejercicio")}
	require.NoError(t, db.Create(seedCatalog).Error)

	seedInstance := cleanupTestInstance(t, db, "Sesión de volumen", seedCatalog.ID)
	seedDay := &dbs.GroupCalendarDay{
		GroupID: seedGroup.ID, Date: inWindow, Kind: string(constants.GroupCalendarDayKindTraining),
		SessionInstanceID: &seedInstance.ID, SourcePlanID: &seedPlan.ID, IsPresencial: true,
	}
	require.NoError(t, db.Create(seedDay).Error)

	// Fuera del scope: otro grupo del mismo dueño, con su propia sesión instancia.
	foreignInstance := cleanupTestInstance(t, db, "Sesión ajena", seedCatalog.ID)
	foreignDay := &dbs.GroupCalendarDay{
		GroupID: otherGroup.ID, Date: inWindow, Kind: string(constants.GroupCalendarDayKindTraining),
		SessionInstanceID: &foreignInstance.ID,
	}
	require.NoError(t, db.Create(foreignDay).Error)

	// Fuera de la ventana: mismo grupo, un día después.
	outsideInstance := cleanupTestInstance(t, db, "Sesión de la otra semana", seedCatalog.ID)
	outsideDay := &dbs.GroupCalendarDay{
		GroupID: seedGroup.ID, Date: outsideWindow, Kind: string(constants.GroupCalendarDayKindTraining),
		SessionInstanceID: &outsideInstance.ID, SourcePlanID: &seedPlan.ID,
	}
	require.NoError(t, db.Create(outsideDay).Error)

	// Feedback y estado de sesión de los tres días.
	cleanupTestSeries(t, db, seedDay.Date, seedInstance.ID, owner.ID, athlete.ID, seedTeam.ID, runnersessionFinished)
	cleanupTestSeries(t, db, foreignDay.Date, foreignInstance.ID, owner.ID, athlete.ID, otherTeam.ID, runnersessionFinished)
	cleanupTestSeries(t, db, outsideDay.Date, outsideInstance.ID, owner.ID, athlete.ID, seedTeam.ID, runnersessionFinished)

	// Un ejercicio del catálogo ajeno, que el seed no puede borrar.
	foreignCatalog := &dbs.Exercise{OwnerID: owner.ID, Name: "Ejercicio propio", Kind: string(constants.ExerciseKindLiviano)}
	require.NoError(t, db.Create(foreignCatalog).Error)

	return &cleanupFixture{
		db: db,
		scope: cleanupScope{
			TeamID: seedTeam.ID, GroupID: seedGroup.ID, OwnerID: owner.ID,
			From: window.from(), To: window.to(),
		},
		seedCatalog: seedCatalog.ID, seedSession: seedSession.ID, seedPlan: seedPlan.ID,
		seedDay: seedDay.ID, seedInstance: seedInstance.ID,
		foreignDay: foreignDay.ID, foreignCatalog: foreignCatalog.ID,
	}
}

const runnersessionFinished = "finished"

// 8.4: la limpieza borra lo del seed y no toca nada fuera del scope.
func TestCleanup_NoDejaResidiosNiTocaFueraDelScope(t *testing.T) {
	fixture := newCleanupFixture(t, "residuos")

	steps, err := buildCleanupSteps(fixture.db, fixture.scope)
	require.NoError(t, err)
	require.NotEmpty(t, steps)

	counts, err := countCleanupSteps(fixture.db, steps)
	require.NoError(t, err)
	assert.Positive(t, counts["session_instances"])
	assert.Positive(t, counts["exercise_instances"])
	assert.Positive(t, counts["workout_feedback"])
	assert.Positive(t, counts["runner_session"])
	assert.Positive(t, counts["group_calendar_days"])
	assert.Positive(t, counts["exercises"])

	affected, err := executeCleanup(fixture.db, steps)
	require.NoError(t, err)
	assert.Positive(t, affected["session_instances"])

	assert.Zero(t, countRows(t, fixture.db, "session_instances", "id = ?", fixture.seedInstance))
	assert.Zero(t, countRows(t, fixture.db, "exercise_instances", "id IN (SELECT exercise_instance_id FROM session_exercise_instances WHERE session_instance_id = ?)", fixture.seedInstance))
	assert.Zero(t, countRows(t, fixture.db, "workout_feedback", "assigned_session_id = ?", fixture.seedInstance))
	assert.Zero(t, countRows(t, fixture.db, "runner_session", "session_instance_id = ?", fixture.seedInstance))
	assert.Zero(t, countRows(t, fixture.db, "group_calendar_days", "id = ?", fixture.seedDay))
	assert.Zero(t, countRows(t, fixture.db, "training_plans", "id = ?", fixture.seedPlan))
	assert.Zero(t, countRows(t, fixture.db, "exercises", "id = ?", fixture.seedCatalog))

	// Lo ajeno sigue ahí: otro grupo del mismo dueño, la otra semana, el
	// ejercicio de catálogo propio y la fila del plan que no se toca.
	assert.Equal(t, int64(1), countRows(t, fixture.db, "group_calendar_days", "id = ?", fixture.foreignDay))
	assert.Equal(t, int64(1), countRows(t, fixture.db, "exercises", "id = ?", fixture.foreignCatalog))
	assert.Equal(t, int64(1), countRows(t, fixture.db, "group_calendar_days",
		"group_id = ? AND date > ?", fixture.scope.GroupID, fixture.scope.To))
	assert.Positive(t, countRows(t, fixture.db, "workout_feedback", "team_id <> ?", fixture.scope.TeamID))
}

// El día de la otra semana es del MISMO plan: si el cleanup no lo acotara por
// ventana, se llevaría la sesión instancia y su feedback también.
func TestCleanup_NoSaleDeLaVentanaTemporal(t *testing.T) {
	fixture := newCleanupFixture(t, "ventana")

	steps, err := buildCleanupSteps(fixture.db, fixture.scope)
	require.NoError(t, err)
	_, err = executeCleanup(fixture.db, steps)
	require.NoError(t, err)

	var remaining int64
	require.NoError(t, fixture.db.Table("group_calendar_days").
		Where("group_id = ? AND date > ?", fixture.scope.GroupID, fixture.scope.To).
		Count(&remaining).Error)
	assert.Positive(t, remaining, "el día fuera de la ventana no podía borrarse")
}

// El runner_session va primero en el orden de borrado: si fuera último, el
// cleanup terminaría con filas apuntando a session_instances inexistentes.
func TestCleanupOrder_EmpiezaPorRunnerSession(t *testing.T) {
	require.NotEmpty(t, cleanupOrder)
	assert.Equal(t, "runner_session", cleanupOrder[0])
	assert.Equal(t, "workout_feedback_points", cleanupOrder[1])
	assert.Equal(t, "workout_feedback", cleanupOrder[2])
	assert.Equal(t, "exercises", cleanupOrder[len(cleanupOrder)-1])
	assert.NotContains(t, cleanupOrder, "group_users",
		"group_users no se borra: el seed solo mueve date_start (tarea 8.1)")
}

// Idempotente: correrlo dos veces no falla y la segunda no hace nada.
func TestCleanup_Idempotente(t *testing.T) {
	fixture := newCleanupFixture(t, "idempotente")

	steps, err := buildCleanupSteps(fixture.db, fixture.scope)
	require.NoError(t, err)
	_, err = executeCleanup(fixture.db, steps)
	require.NoError(t, err)

	stepsAgain, err := buildCleanupSteps(fixture.db, fixture.scope)
	require.NoError(t, err)
	counts, err := countCleanupSteps(fixture.db, stepsAgain)
	require.NoError(t, err)
	affected, err := executeCleanup(fixture.db, stepsAgain)
	require.NoError(t, err)

	for _, table := range cleanupOrder {
		assert.Zero(t, affected[table], "la segunda pasada borró filas de %s", table)
		assert.Equal(t, counts[table], affected[table], "%s: el conteo previo y el borrado tienen que coincidir", table)
	}
}

// Sin nada del seed, el cleanup no tiene que inventar pasos.
func TestCleanup_SinSeedNoHaceNada(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "cleanup-vacio-owner@test.com", "90001")
	team := cleanupTestTeam(t, db, "equipo_cleanup_vacio", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_cleanup_vacio", team.ID)

	steps, err := buildCleanupSteps(db, cleanupScope{
		TeamID: team.ID, GroupID: group.ID, OwnerID: owner.ID,
		From: time.Now().AddDate(0, 0, -20), To: time.Now(),
	})
	require.NoError(t, err)
	assert.Empty(t, steps)
}

// emptyMediaURLs inicializa el TextArray que la migración deja NOT NULL: sin esto
// el insert falla con un error de encoding que no dice nada de media.
func emptyMediaURLs(t *testing.T) pgtype.TextArray {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{}))
	return media
}

func countRows(t *testing.T, db *gorm.DB, table, where string, args ...interface{}) int64 {
	t.Helper()
	var total int64
	require.NoError(t, db.Table(table).Where(where, args...).Count(&total).Error)
	return total
}

func cleanupTestUser(t *testing.T, db *gorm.DB, email, dni string) *dbs.User {
	t.Helper()
	user := &dbs.User{
		Name: "Seed", Surname: "Cleanup", Email: email, DNI: dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "x",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func cleanupTestTeam(t *testing.T, db *gorm.DB, name string, ownerID int64) *dbs.Team {
	t.Helper()
	team := &dbs.Team{Name: name, OwnerID: ownerID, MaxMembers: 10}
	require.NoError(t, db.Create(team).Error)
	return team
}

func cleanupTestGroup(t *testing.T, db *gorm.DB, name string, teamID int64) *dbs.Group {
	t.Helper()
	group := &dbs.Group{Name: name, TeamID: teamID}
	require.NoError(t, db.Create(group).Error)
	return group
}

// cleanupTestInstance arma una SessionInstance con un ExerciseInstance y su link.
func cleanupTestInstance(t *testing.T, db *gorm.DB, name string, catalogID int64) *dbs.SessionInstance {
	t.Helper()
	instance := &dbs.SessionInstance{Name: name}
	require.NoError(t, db.Create(instance).Error)
	exerciseInstance := &dbs.ExerciseInstance{
		Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo),
		Minutes: intPtr(30), DistanceM: intPtr(4000), SourceExerciseID: &catalogID,
	}
	require.NoError(t, db.Create(exerciseInstance).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: instance.ID, ExerciseInstanceID: exerciseInstance.ID,
		Role: string(constants.SessionExerciseRoleMain),
	}).Error)
	return instance
}

func cleanupTestSeries(t *testing.T, db *gorm.DB, date time.Time, instanceID, ownerID, athleteID, teamID int64, status string) {
	t.Helper()
	exerciseInstanceID := int64(0)
	require.NoError(t, db.Table("session_exercise_instances").
		Where("session_instance_id = ?", instanceID).Pluck("exercise_instance_id", &exerciseInstanceID).Error)
	timeMs := int64(1_500_000)
	distance := 4000.0
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID:              &teamID,
		AssignedSessionID:   instanceID,
		AssignedExerciseID:  exerciseInstanceID,
		AthleteUserID:       athleteID,
		FeedbackOwnerUserID: ownerID,
		ReportSource:        "corredor",
		SessionDate:         date,
		SetNumber:           1,
		DurationMs:          &timeMs,
		ActiveDurationMs:    &timeMs,
		DistanceMeters:      &distance,
		MediaURLs:           emptyMediaURLs(t),
	}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: instanceID, AthleteUserID: athleteID,
		Status: status, StartDate: date,
	}).Error)
}
