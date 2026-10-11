package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Fixture D10 batch: instancia de sesión con sus ejercicios/links propios
// (sin sharing entre instancias), creados directo por DAO sobre la DB de test.
type supersededFixture struct {
	instance  *dbs.SessionInstance
	exercises []dbs.ExerciseInstance
	links     []dbs.SessionExerciseInstance
}

func createSupersededInstance(t *testing.T, db *gorm.DB, name string, exerciseNames ...string) *supersededFixture {
	t.Helper()
	sessionDao := daos.NewSessionInstanceDao(db)
	exDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: name}
	require.NoError(t, sessionDao.Create(nil, inst))
	fix := &supersededFixture{instance: inst}
	for _, exName := range exerciseNames {
		ex := &dbs.ExerciseInstance{Name: exName, Kind: "running"}
		require.NoError(t, exDao.Create(nil, ex))
		link := &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}
		require.NoError(t, linkDao.Create(nil, link))
		fix.exercises = append(fix.exercises, *ex)
		fix.links = append(fix.links, *link)
	}
	return fix
}

func createSupersedeFeedback(t *testing.T, db *gorm.DB, sessionID, exerciseID int64, softDeleted bool) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/a.jpg"}))
	feedback := &dbs.WorkoutFeedback{
		AssignedSessionID:   sessionID,
		AssignedExerciseID:  exerciseID,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		SetNumber:           1,
		MediaURLs:           media,
	}
	require.NoError(t, db.Create(feedback).Error)
	if softDeleted {
		require.NoError(t, db.Model(&dbs.WorkoutFeedback{}).Where("id = ?", feedback.ID).Update("deleted_at", time.Now()).Error)
	}
}

// La instancia queda completa (sesión + links + ejercicios intactos) cuando
// sobrevive al borrado de superadas (semántica D10).
func assertInstanceIntact(t *testing.T, db *gorm.DB, fix *supersededFixture) {
	t.Helper()
	stmt := func(query string, args ...interface{}) int64 {
		var count int64
		require.NoError(t, db.Raw(query, args...).Scan(&count).Error)
		return count
	}
	assert.Equal(t, int64(1), stmt("SELECT COUNT(*) FROM session_instances WHERE id = ?", fix.instance.ID), "sesión instancia intacta")
	assert.Equal(t, int64(len(fix.links)), stmt("SELECT COUNT(*) FROM session_exercise_instances WHERE session_instance_id = ?", fix.instance.ID), "links intactos")
	exerciseIDs := make([]int64, 0, len(fix.exercises))
	for _, ex := range fix.exercises {
		exerciseIDs = append(exerciseIDs, ex.ID)
	}
	assert.Equal(t, int64(len(fix.exercises)), stmt("SELECT COUNT(*) FROM exercise_instances WHERE id IN ?", exerciseIDs), "ejercicios intactos")
}

func assertInstanceDeleted(t *testing.T, db *gorm.DB, fix *supersededFixture) {
	t.Helper()
	stmt := func(query string, args ...interface{}) int64 {
		var count int64
		require.NoError(t, db.Raw(query, args...).Scan(&count).Error)
		return count
	}
	assert.Equal(t, int64(0), stmt("SELECT COUNT(*) FROM session_instances WHERE id = ?", fix.instance.ID), "sesión instancia borrada")
	assert.Equal(t, int64(0), stmt("SELECT COUNT(*) FROM session_exercise_instances WHERE session_instance_id = ?", fix.instance.ID), "links borrados")
	exerciseIDs := make([]int64, 0, len(fix.exercises))
	for _, ex := range fix.exercises {
		exerciseIDs = append(exerciseIDs, ex.ID)
	}
	assert.Equal(t, int64(0), stmt("SELECT COUNT(*) FROM exercise_instances WHERE id IN ?", exerciseIDs), "ejercicios borrados")
}

// Mixto (D10/D6c): A protegida por feedback de sesión → queda COMPLETA; B sin
// feedback → borrada de abajo para arriba (links, ejercicios, sesión).
func TestCalendarService_deleteSupersededInstances_PreservaPorFeedbackDeSesion(t *testing.T) {
	db := testutils.SetupTestDB(t)
	protected := createSupersededInstance(t, db, "protegida sesión", "e1", "e2")
	deletable := createSupersededInstance(t, db, "borrable", "e3", "e4")
	createSupersedeFeedback(t, db, protected.instance.ID, 999999, false)
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	require.NoError(t, svc.deleteSupersededInstances(nil, db, []int64{protected.instance.ID, deletable.instance.ID}))

	assertInstanceIntact(t, db, protected)
	assertInstanceDeleted(t, db, deletable)
}

// Feedback de UN ejercicio protege la instancia entera (D10) y deja NO
// borrados los demás ejercicios de esa sesión.
func TestCalendarService_deleteSupersededInstances_PreservaPorFeedbackDeEjercicio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	protected := createSupersededInstance(t, db, "protegida ejercicio", "e1", "e2", "e3")
	createSupersedeFeedback(t, db, protected.instance.ID, protected.exercises[1].ID, false)
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	require.NoError(t, svc.deleteSupersededInstances(nil, db, []int64{protected.instance.ID}))

	assertInstanceIntact(t, db, protected)
}

// Mixto con protección via ejercicio: A protegida, B borrable → solo B se va.
func TestCalendarService_deleteSupersededInstances_MixtoProtegidaPorEjercicio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	protected := createSupersededInstance(t, db, "protegida por ejercicio", "e1")
	deletable := createSupersededInstance(t, db, "borrable", "e2")
	createSupersedeFeedback(t, db, protected.instance.ID, protected.exercises[0].ID, false)
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	require.NoError(t, svc.deleteSupersededInstances(nil, db, []int64{protected.instance.ID, deletable.instance.ID}))

	assertInstanceIntact(t, db, protected)
	assertInstanceDeleted(t, db, deletable)
}

// El feedback soft-deleteado NO protege (deleted_at ignorado): la instancia
// se borra igual que sin feedback.
func TestCalendarService_deleteSupersededInstances_FeedbackSoftDeleteNoProtege(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionInst := createSupersededInstance(t, db, "fb sesion soft", "e1")
	exerciseInst := createSupersededInstance(t, db, "fb ejercicio soft", "e2")
	createSupersedeFeedback(t, db, sessionInst.instance.ID, 999999, true)
	createSupersedeFeedback(t, db, exerciseInst.instance.ID, exerciseInst.exercises[0].ID, true)
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	require.NoError(t, svc.deleteSupersededInstances(nil, db, []int64{sessionInst.instance.ID, exerciseInst.instance.ID}))

	assertInstanceDeleted(t, db, sessionInst)
	assertInstanceDeleted(t, db, exerciseInst)
}

// Lote vacío (o con ids basura) es noop sin tocar la DB.
func TestCalendarService_deleteSupersededInstances_LoteVacioEsNoop(t *testing.T) {
	db := testutils.SetupTestDB(t)
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)
	require.NoError(t, svc.deleteSupersededInstances(nil, db, nil))
	require.NoError(t, svc.deleteSupersededInstances(nil, db, []int64{}))
	require.NoError(t, svc.deleteSupersededInstances(nil, db, []int64{0}))
}

// Fixture bulk-clear: días training existentes con sus instancias, creados
// directo por DAO (el write path batch real se cubre en la suite de stamp/bulk).
type bulkClearFixture struct {
	owner     dbs.User
	team      dbs.Team
	group     dbs.Group
	days      []dbs.GroupCalendarDay
	instances []*supersededFixture
}

func createBulkClearFixture(t *testing.T, db *gorm.DB, dateStrings []string) *bulkClearFixture {
	t.Helper()
	// DNI/email únicos por fixture: la suite corre varios fixtures sobre la
	// misma DB sin truncado entre tests (SetupTestDB).
	position := time.Now().UnixNano()
	owner := dbs.User{Name: "Bulkclear", Surname: "W", Email: fmt.Sprintf("bulk-clear-owner-%d@test.com", position), DNI: fmt.Sprintf("%d", 51009101+position%1000000), BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(&owner).Error)
	team := dbs.Team{Name: "Bulkclear team", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(&team).Error)
	group := dbs.Group{Name: "Bulkclear group", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(&group).Error)
	fix := &bulkClearFixture{owner: owner, team: team, group: group}
	for _, dateStr := range dateStrings {
		date, err := time.Parse("2006-01-02", dateStr)
		require.NoError(t, err)
		inst := createSupersededInstance(t, db, "instancia "+dateStr, "ex "+dateStr)
		day := dbs.GroupCalendarDay{
			GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &inst.instance.ID,
		}
		require.NoError(t, db.Create(&day).Error)
		fix.days = append(fix.days, day)
		fix.instances = append(fix.instances, inst)
	}
	return fix
}

func bulkClearService(db *gorm.DB) *calendarService {
	return NewCalendarService(
		daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), nil, nil, nil, nil, db,
	).(*calendarService)
}

// D7: el guard de cerrado dispara el 422 con las fechas cerradas EN EL
// ORDEN DEL INPUT (y el tx ni siquiera llega a borrar nada).
func TestCalendarService_BulkClear_ClosedErrorEnOrdenDelInput(t *testing.T) {
	db := testutils.SetupTestDB(t)
	closedPast := time.Now().AddDate(0, 0, -5).Format("2006-01-02")
	closedPast2 := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	openFuture := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	// El input intercala cerradas y abiertas — el 422 debe listar solo las
	// cerradas en el orden en que llegaron.
	reqDates := []string{openFuture, closedPast, closedPast2}
	fix := createBulkClearFixture(t, db, []string{closedPast, closedPast2})
	svc := bulkClearService(db)

	err := svc.BulkClear(nil, fix.group.ID, fix.owner.ID, calendar.BulkClearRequest{Dates: []string{reqDates[0], reqDates[1], reqDates[2]}})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	assert.Equal(t, newCalendarClosedDaysError([]string{reqDates[1], reqDates[2]}).Error(), err.Error(), "lista cerrada en orden del input, igual que el loop por fecha de antes")
	// El rollback de la tx dejó todo como estaba.
	fetched := make([]dbs.GroupCalendarDay, 0)
	require.NoError(t, db.Where("group_id = ?", fix.group.ID).Find(&fetched).Error)
	assert.Len(t, fetched, 2)
	assertInstanceIntact(t, db, fix.instances[0])
	assertInstanceIntact(t, db, fix.instances[1])
}

// D7: conteo de queries FIJO. Bulk-clear de 4 fechas training con instancia
// (1 ejercicio c/u), ninguna cerrada. Round trips: 2 del guard de owner
// (grupo + equipo) + 1 lectura IN de fechas (FindByGroupAndDates) +
// 1 DeleteByDates + 6 del batch de supersede (fb sessión + links + fb
// ejercicio + 3 deletes IN) + BEGIN/savepoint + commit.
func TestCalendarService_BulkClear_QueriesFijas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dates := []string{
		time.Now().AddDate(0, 0, 21).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 22).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 23).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 24).Format("2006-01-02"),
	}
	fix := createBulkClearFixture(t, db, dates)

	counter := 0
	counting := txCountingDB(t, db, &counter)
	svc := NewCalendarService(
		daos.NewGroupCalendarDayDao(counting), daos.NewGroupDao(counting), daos.NewTeamDao(counting),
		daos.NewGroupUserDao(counting), nil, nil, nil, nil, counting,
	).(*calendarService)

	err := svc.BulkClear(nil, fix.group.ID, fix.owner.ID, calendar.BulkClearRequest{Dates: dates})
	require.NoError(t, err)
	queries4 := counter
	t.Logf("bulk-clear 4 fechas: %d queries", queries4)

	// El conteo ES INDEPENDIENTE de la cantidad de fechas — segunda corrida
	// con 8 fechas (otro grupo) da el MISMO número: reads/deletes/supersede
	// van todos con IN, no por fecha.
	moreDates := []string{
		time.Now().AddDate(0, 0, 31).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 32).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 33).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 34).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 35).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 36).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 37).Format("2006-01-02"),
		time.Now().AddDate(0, 0, 38).Format("2006-01-02"),
	}
	fix2 := createBulkClearFixture(t, db, moreDates)
	counter = 0
	err = svc.BulkClear(nil, fix2.group.ID, fix2.owner.ID, calendar.BulkClearRequest{Dates: moreDates})
	require.NoError(t, err)
	t.Logf("bulk-clear 8 fechas: %d queries", counter)

	assert.Equal(t, queries4, counter, "el conteo no depende de la cantidad de fechas")
	assert.Equal(t, 11, queries4, "2 owner (grupo+equipo) + 1 FindByGroupAndDates + 1 DeleteByDates + 6 del batch de supersede (fb sesión + links + fb ejercicio + 3 deletes IN) + savepoint + commit")
}
