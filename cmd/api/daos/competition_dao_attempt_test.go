package daos

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/testutils"
)

// CreateAttemptSession (Gap 36, intentos múltiples) contra Postgres real:
// intento 1 reusa la instancia compartida, intentos 2+ crean instancia
// propia "(intento M)" y el límite lo decide el tx con lock FOR UPDATE.

func TestCompetitionDAO_CreateAttemptSessionFirstReusesSharedInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "att01")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	shared := *comp.RouteSessionInstanceID

	rs, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.NoError(t, err)
	require.NotNil(t, rs)
	assert.NotZero(t, rs.ID)
	assert.Equal(t, shared, rs.SessionInstanceID, "intento 1 reusa la instancia compartida")
	assert.Equal(t, fx.runnerA.ID, rs.AthleteUserID)
	assert.Equal(t, runnersession.RunnerSessionStatusWip, rs.Status)
	require.NotNil(t, rs.CompetitionID)
	assert.Equal(t, comp.ID, *rs.CompetitionID)

	var attemptInstances int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).
		Where("name LIKE ?", "Competencia ruta % (intento%").Count(&attemptInstances).Error)
	assert.Zero(t, attemptInstances, "el intento 1 no crea instancia nueva")
}

func TestCompetitionDAO_CreateAttemptSessionSecondThirdCreateInstances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "att02")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.MaxAttempts = nil
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	shared := *comp.RouteSessionInstanceID

	// max_attempts NULL = intents ilimitados: 3 intentos sin rechazo.
	for i := 1; i <= 3; i++ {
		_, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
		require.NoError(t, err, "con max_attempts NULL no hay rechazo, intento %d", i)
	}

	expected := map[string]int64{
		fmt.Sprintf("Competencia ruta %d (intento 2)", comp.ID): 1,
		fmt.Sprintf("Competencia ruta %d (intento 3)", comp.ID): 1,
	}
	for name, n := range expected {
		var count int64
		require.NoError(t, db.Model(&dbs.SessionInstance{}).Where("name = ?", name).Count(&count).Error)
		assert.Equal(t, n, count, "instancia del intento %s", name)
	}

	// Cada instancia nueva es una réplica del sintético: link main/repeat 1 y
	// exercise "Corrida" kind velocidad_distancia.
	for name := range expected {
		var inst dbs.SessionInstance
		require.NoError(t, db.Where("name = ?", name).First(&inst).Error)
		var links []dbs.SessionExerciseInstance
		require.NoError(t, db.Where("session_instance_id = ?", inst.ID).Find(&links).Error)
		require.Len(t, links, 1)
		assert.Equal(t, string(constants.SessionExerciseRoleMain), links[0].Role)
		assert.Equal(t, 1, links[0].RepeatCount)
		assert.Equal(t, 0, links[0].RestMinutes)
		var ex dbs.ExerciseInstance
		require.NoError(t, db.Where("id = ?", links[0].ExerciseInstanceID).First(&ex).Error)
		assert.Equal(t, "Corrida", ex.Name)
		assert.Nil(t, ex.SourceExerciseID)
	}

	var attemptCount int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).
		Where("competition_id = ? AND athlete_user_id = ?", comp.ID, fx.runnerA.ID).
		Count(&attemptCount).Error)
	assert.Equal(t, int64(3), attemptCount)

	var sharedCount int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).
		Where("session_instance_id = ? AND athlete_user_id = ?", shared, fx.runnerA.ID).
		Count(&sharedCount).Error)
	assert.Equal(t, int64(1), sharedCount, "solo el intento 1 corrió sobre la compartida")
}

func TestCompetitionDAO_CreateAttemptSessionLimitReached(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "att03")

	maxTwo := int64(2)
	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.MaxAttempts = &maxTwo
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))

	// Dos intentos del mismo atleta caben (aunque el atleta B no compite acá).
	_, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.NoError(t, err)
	_, err = dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.NoError(t, err)

	// El límite es por atleta: el otro participante sigue teniendo su intento 1.
	_, err = dao.CreateAttemptSession(nil, comp, fx.runnerB.ID)
	require.NoError(t, err)

	// Tercero del atleta A: límite alcanzado y cero filas nuevas.
	_, err = dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.ErrorIs(t, err, ErrCompetitionAttemptLimit)

	var rsCount int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).
		Where("competition_id = ?", comp.ID).Count(&rsCount).Error)
	assert.Equal(t, int64(3), rsCount, "A=2 + B=1, el rechazado no insertó")

	var attemptInstances int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).
		Where("name LIKE ?", fmt.Sprintf("Competencia ruta %%%d (intento%%", comp.ID)).
		Count(&attemptInstances).Error)
	assert.Equal(t, int64(1), attemptInstances, "solo el intento 2 creó instancia; el rechazado no la creó")
}

// El COUNT cuenta cualquier status: un intento finished/interrupted bloquea
// igual que un wip (el límite es de intentos, no de sesiones en curso).
func TestCompetitionDAO_CreateAttemptSessionLimitCountsAnyStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "att04")

	maxOne := int64(1)
	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.MaxAttempts = &maxOne
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	shared := *comp.RouteSessionInstanceID

	// Intento previo finished tagueado a mano (no via CreateAttemptSession):
	// igualmente consume el límite.
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: shared, AthleteUserID: fx.runnerA.ID,
		Status:        runnersession.RunnerSessionStatusFinished,
		StartDate:     time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
		CompetitionID: &comp.ID,
	}).Error)

	_, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.ErrorIs(t, err, ErrCompetitionAttemptLimit)

	// Un atleta sin intentos previos pasa por encima del mismo límite.
	_, err = dao.CreateAttemptSession(nil, comp, fx.runnerB.ID)
	require.NoError(t, err)
}

// Conflicto del intento 1 (ON CONFLICT DO NOTHING): si el atleta ya tiene
// runner_session sobre la instancia compartida SIN tag de competencia, el
// insert no inserta — error de doble intento sin modificar nada.
func TestCompetitionDAO_CreateAttemptSessionConflictOnSharedInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "att05")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	shared := *comp.RouteSessionInstanceID

	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: shared, AthleteUserID: fx.runnerA.ID,
		StartDate: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
	}).Error)

	_, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.ErrorIs(t, err, ErrCompetitionAttemptConflict)

	var rsCount int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).
		Where("session_instance_id = ? AND athlete_user_id = ?", shared, fx.runnerA.ID).
		Count(&rsCount).Error)
	assert.Equal(t, int64(1), rsCount)
}

// Rollback total (fallo forzado post-instancia): trigger que bloquea el
// INSERT sobre runner_session. La instancia nueva debe revertirse y no debe
// quedar sesión huérfana — y el trigger vuela con el rollback del test tx.
func TestCompetitionDAO_CreateAttemptSessionRollbackPostInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "att06")

	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.MaxAttempts = nil
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))

	// Intento 1 real para que el siguiente entre por el branch de instancia nueva.
	_, err := dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.NoError(t, err)

	require.NoError(t, db.Exec(`CREATE FUNCTION daos_attempt_block_insert_test() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'bloqueo de test para rollback';
		END;
		$$ LANGUAGE plpgsql;`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER daos_attempt_block_test
		BEFORE INSERT ON runner_session FOR EACH ROW
		EXECUTE FUNCTION daos_attempt_block_insert_test();`).Error)

	_, err = dao.CreateAttemptSession(nil, comp, fx.runnerA.ID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCompetitionAttemptLimit)
	assert.NotErrorIs(t, err, ErrCompetitionAttemptConflict)

	var rsCount int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).
		Where("competition_id = ? AND athlete_user_id = ?", comp.ID, fx.runnerA.ID).
		Count(&rsCount).Error)
	assert.Equal(t, int64(1), rsCount, "solo quedó el intento 1")

	var attemptInstances int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).
		Where("name LIKE ?", "Competencia ruta % (intento%").Count(&attemptInstances).Error)
	assert.Zero(t, attemptInstances, "la instancia del intento fallido se revirtió: sin huérfanos")
}

// Carrera concurrente real (dos conexiones, límite para 1 intento más): una
// crea, la otra recibe el error de límite. El lock FOR UPDATE serializa; sin
// él, ambos COUNT=0 permitirían insertar encima del límite.
func TestCompetitionDAO_CreateAttemptSessionRace(t *testing.T) {
	db := testutils.SetupTestDB(t) // solo salta el test sin TEST_DB_HOST seteada
	_ = db

	raceFixtures := newRaceAttemptFixture(t)

	type outcome struct {
		rs  *dbs.RunnerSession
		err error
	}
	start := make(chan struct{})
	out := make(chan outcome, 2)
	var ready sync.WaitGroup
	ready.Add(2)

	for _, racerDB := range raceFixtures.racerDBs {
		racer := NewCompetitionDao(racerDB)
		go func() {
			ready.Done()
			<-start
			rs, err := racer.CreateAttemptSession(nil, raceFixtures.comp, raceFixtures.athleteID)
			out <- outcome{rs: rs, err: err}
		}()
	}
	ready.Wait()
	close(start)

	okCount, limitCount := 0, 0
	for i := 0; i < 2; i++ {
		o := <-out
		switch {
		case o.err == nil:
			okCount++
		case errors.Is(o.err, ErrCompetitionAttemptLimit):
			limitCount++
		default:
			require.NoError(t, o.err, "error inesperado en la carrera")
		}
	}
	assert.Equal(t, 1, okCount, "solo uno de los concurrentes abre intento")
	assert.Equal(t, 1, limitCount, "el otro recibe el límite")

	var rsCount int64
	require.NoError(t, raceFixtures.cleanerDB.Model(&dbs.RunnerSession{}).
		Where("competition_id = ? AND athlete_user_id = ?", raceFixtures.comp.ID, raceFixtures.athleteID).
		Count(&rsCount).Error)
	assert.Equal(t, int64(1), rsCount, "lock FOR UPDATE: el límite se respeta en competencia")
}

// raceAttemptFixtureData agrupa el estado commiteado que necesita el test de
// carrera (Todo va commiteado: el tx-rollback de SetupTestDB no protege datos
// que otras conexiones deben ver).
type raceAttemptFixtureData struct {
	athleteID int64
	comp      *dbs.Competition
	racerDBs  []*gorm.DB
	cleanerDB *gorm.DB
}

func newRaceAttemptFixture(t *testing.T) raceAttemptFixtureData {
	t.Helper()
	raw := openRawTestDB(t)

	fx := newCompetitionFixture(t, raw, "race36")

	one := int64(1)
	comp := newRouteCompetition()
	comp.TeamID = fx.team.ID
	comp.CreatedByUserID = fx.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.MaxAttempts = &one
	dao := NewCompetitionDao(raw)
	require.NoError(t, dao.CreateWithInstance(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID}))
	require.NotZero(t, comp.ID)

	race := raceAttemptFixtureData{
		athleteID: fx.runnerA.ID,
		comp:      comp,
		racerDBs:  []*gorm.DB{openRawTestDB(t), openRawTestDB(t)},
		cleanerDB: raw,
	}

	// Los datos viven commiteados en la DB de test persistente: cleanup
	// explícito (self-contained, en orden inverso de creación y con los
	// subqueries ANTES de borrar las tablas de las que dependen).
	t.Cleanup(func() {
		if comp.ID != 0 {
			cleanupRaceExec(t, race.cleanerDB, `DELETE FROM runner_session WHERE competition_id = ?`, comp.ID)
			cleanupRaceExec(t, race.cleanerDB, `DELETE FROM exercise_instances WHERE id IN (
				SELECT exercise_instance_id FROM session_exercise_instances
				WHERE session_instance_id IN (SELECT id FROM session_instances WHERE name LIKE ?))`,
				raceInstancePattern(comp.ID))
			cleanupRaceExec(t, race.cleanerDB, `DELETE FROM session_exercise_instances WHERE session_instance_id IN (
				SELECT id FROM session_instances WHERE name LIKE ?)`, raceInstancePattern(comp.ID))
			cleanupRaceExec(t, race.cleanerDB, `DELETE FROM session_instances WHERE name LIKE ?`, raceInstancePattern(comp.ID))
			cleanupRaceExec(t, race.cleanerDB, `DELETE FROM competition_participant WHERE competition_id = ?`, comp.ID)
			cleanupRaceExec(t, race.cleanerDB, `DELETE FROM competition WHERE id = ?`, comp.ID)
		}
		userIDs := []int64{fx.owner.ID, fx.runnerA.ID, fx.runnerB.ID, fx.outsider.ID}
		teamIDs := []int64{fx.team.ID, fx.otherTeam.ID}
		cleanupRaceExec(t, race.cleanerDB, `DELETE FROM group_users WHERE user_id IN ?`, userIDs)
		cleanupRaceExec(t, race.cleanerDB, `DELETE FROM groups WHERE team_id IN ?`, teamIDs)
		cleanupRaceExec(t, race.cleanerDB, `DELETE FROM teams WHERE id IN ?`, teamIDs)
		cleanupRaceExec(t, race.cleanerDB, `DELETE FROM users WHERE id IN ?`, userIDs)
	})
	return race
}

func raceInstancePattern(competitionID int64) string {
	return fmt.Sprintf("Competencia ruta %d%%", competitionID)
}

// cleanupRaceExec corre un DELETE de limpieza y FALLA el test (en el cleanup)
// si la query no anduvo: no pueden quedar residuales silenciosos commiteados
// en la DB de test compartida.
func cleanupRaceExec(t *testing.T, db *gorm.DB, query string, args ...interface{}) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Errorf("limpieza del test de carrera falló (%s): %v", query, err)
	}
}

// openRawTestDB abre una conexión gorm propia (no el tx compartido del
// SetupTestDB): el test de carrera necesita transacciones paralelas sobre
// conexiones distintas.
func openRawTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		t.Skip("TEST_DB_HOST no seteada, saltando test de integración con Postgres (ver docs/TESTING.md)")
	}
	port := os.Getenv("TEST_DB_PORT")
	if port == "" {
		port = "5432"
	}
	cfg := config.DB{
		Host:               host,
		Port:               port,
		Username:           envDefaultOr("TEST_DB_USER", "postgres"),
		Password:           envDefaultOr("TEST_DB_PASSWORD", "postgres"),
		Name:               envDefaultOr("TEST_DB_NAME", "paceron_test"),
		MaxIdleConnections: 5,
		MaxOpenConnections: 15,
		ConnMaxLifetime:    time.Hour,
	}
	db, err := gorm.Open(postgres.Open(postgresdb.ConnString(cfg)), &gorm.Config{
		Logger: gl.Default.LogMode(gl.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func envDefaultOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
