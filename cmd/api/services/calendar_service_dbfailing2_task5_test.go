package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/testutils"
)

// Ramas restantes de error de DB: llamadas directas a helpers internos con
// FailingDB por contador de operación, y flujos de servicio con falla inyectada
// en la posición exacta del encadenamiento transaccional.

// --- sessionInstanceResponse directamente ----------------------------------

func TestDBError_sessionInstanceResponse_SelectsFallidas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	sessionDao := daos.NewSessionInstanceDao(db)
	sess := &dbs.SessionInstance{Name: "S fallas"}
	require.NoError(t, sessionDao.Create(nil, sess))
	require.NoError(t, daos.NewSessionExerciseInstanceDao(db).Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: 1, Role: "main"}))
	id := sess.ID
	for nth, expected := range map[int]string{2: "error listing session exercise instances", 3: "error finding exercise instances by ids"} {
		failing := testutils.FailingDB(t, db, nthFail("select", nth))
		svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, failing).(*calendarService)

		_, err := svc.sessionInstanceResponse(nil, failing, &id)

		require.Error(t, err, "select #%d", nth)
		assert.Contains(t, err.Error(), expected)
	}

	// Select #1 (instancia inexistente) devuelve el caso nil-instance sin envejar.
	notFailing := testutils.FailingDB(t, db, nthFail("select", 99))
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, notFailing).(*calendarService)
	orphan := int64(987654321)
	_, err := svc.sessionInstanceResponse(nil, notFailing, &orphan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
	_ = id
}

// IDs sin instancia: el detalle completo sale del detalle D9.
func TestDBError_sessionInstanceResponse_Actual(t *testing.T) {
	db := testutils.SetupTestDB(t)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	sess := &dbs.SessionInstance{Name: "S detalle"}
	require.NoError(t, sessionDao.Create(nil, sess))
	ex := &dbs.ExerciseInstance{Name: "E detalle", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: sess.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	resp, err := svc.sessionInstanceResponse(nil, db, &sess.ID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "S detalle", resp.Name)
}

// --- deleteSupersededInstance directa --------------------------------------

func TestDBError_deleteSupersededInstance_SelectsFallidas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	instanceDao := daos.NewSessionInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	exDao := daos.NewExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S superseded"}
	require.NoError(t, instanceDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E superseded", Kind: "running"}
	require.NoError(t, exDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	// Selects: #1 HasFeedback sesión, #2 FindBySessionInstance links,
	// #3 HasFeedback ejercicio (por cada link).
	for _, nth := range []int{1, 2, 3} {
		failing := testutils.FailingDB(t, db, nthFail("select", nth))
		err := svc.deleteSupersededInstance(nil, failing, inst.ID)
		require.Error(t, err, "select #%d", nth)
	}

	// Deletes: #1 DeleteBySessionInstance (links), #2 ejercicio físico,
	// #3 instancia (al fallar los primeros, las siguientes pueden no correr).
	for _, nth := range []int{1, 3} {
		failing := testutils.FailingDB(t, db, nthFail("delete", nth))
		err := svc.deleteSupersededInstance(nil, failing, inst.ID)
		require.Error(t, err, "delete #%d", nth)
	}

	// El handle real sigue intacto y la instancia no llegó a borrarse.
	found, ferr := instanceDao.FindByID(nil, inst.ID)
	require.NoError(t, ferr)
	require.NotNil(t, found)
}

// Sin links (instancia sin ejercicios): el loop de HasFeedback de ejercicios no
// corre y el DELETE de instancia falla solo si se inyecta el delete #1 directo
// (sin el paso de links).
func TestDBError_deleteSupersededInstance_SinLinksFallaAlBorrarInstancia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	instanceDao := daos.NewSessionInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S sin links"}
	require.NoError(t, instanceDao.Create(nil, inst))
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, db).(*calendarService)

	failing := testutils.FailingDB(t, db, nthFail("delete", 1))
	err := svc.deleteSupersededInstance(nil, failing, inst.ID)

	require.Error(t, err)
}

// --- instantiateSession con fallas puntuales ----------------------------

func TestDBError_instantiateSession_FallasPuntuales(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr20")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr20")
	dao := daos.NewGroupCalendarDayDao(db)
	date := time.Now().AddDate(0, 0, 8)
	req := calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID}

	// Select #4 (FindByGroupAndDate tx = #1, session = #2, links = #3,
	// ejercicio = #4) → error al buscar ejercicio para instanciar.
	failing := testutils.FailingDB(t, db, nthFail("select", 4))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, req)
	require.EqualError(t, err, "error al guardar día de calendario")

	// Insert #2 (ejercicio instancia; #1 es la sesión instancia).
	failing = testutils.FailingDB(t, db, nthFail("insert", 2))
	svc = NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err = svc.UpsertDay(nil, group.ID, owner.ID, date, req)
	require.EqualError(t, err, "error al guardar día de calendario")

	// Insert #3 (vínculo).
	failing = testutils.FailingDB(t, db, nthFail("insert", 3))
	svc = NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err = svc.UpsertDay(nil, group.ID, owner.ID, date, req)
	require.EqualError(t, err, "error al guardar día de calendario")
}

// --- UpsertDay con día existente → excludeIDs en detección ------------------

func TestDBError_UpsertDay_ExistingPresencialExcludeSelf(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr21")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr21")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 9)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	sessionID := session.ID
	from := "10:00"
	to := "11:00"

	resp, err := svc.UpsertDay(nil, group.ID, owner.ID, d1, calendar.CalendarDayRequest{
		Kind: "training", SessionID: &sessionID, IsPresencial: boolPtr(true),
		PresencialTimeFrom: &from, PresencialTimeTo: &to,
		PresencialLocation: &trainingplan.Location{Lat: -31.4, Lng: -64.2, Label: calStrPtr("pista")},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.IsPresencial)
}

func boolPtr(b bool) *bool { return &b }

// derr inyectada a mitad de tx (2º select de la tx = query de presencial).
func TestDBError_UpsertDay_DetectaColisionesFailingSelect(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr22")
	d1 := time.Now().AddDate(0, 0, 9)
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	from := "10:00"
	to := "11:00"

	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr22")
	sessionID := session.ID
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, d1, calendar.CalendarDayRequest{
		Kind: "training", SessionID: &sessionID, IsPresencial: boolPtr(true),
		PresencialTimeFrom: &from, PresencialTimeTo: &to,
		PresencialLocation: &trainingplan.Location{Lat: -31.4, Lng: -64.2, Label: calStrPtr("pista")},
	})

	require.EqualError(t, err, "error al guardar día de calendario")
	_ = owner
}

// --- DeleteDay -------------------------------------------------------------

func TestDBError_DeleteDay_TXSelectFailIdentical(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr24a")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 4)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	err := svc.DeleteDay(nil, group.ID, owner.ID, d1)

	require.EqualError(t, err, "error al borrar día de calendario")
	kept, ferr := dao.FindByGroupAndDate(nil, group.ID, d1)
	require.NoError(t, ferr)
	assert.NotNil(t, kept, "el rollback no borra nada")
}

// Entrada mockeada y tx con fila desaparecida: txExisting == nil → no-op.
func TestDBError_DeleteDay_TxRowDesaparecida(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "rest"}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, db)

	err := svc.DeleteDay(nil, 1, 7, time.Now().AddDate(0, 0, 3))

	require.NoError(t, err)
}

func TestDBError_DeleteDay_TxDiaSinInstancia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr23")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 4)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	err := svc.DeleteDay(nil, group.ID, owner.ID, d1)

	require.NoError(t, err)
	orphanLeft, ferr := dao.FindByGroupAndDate(nil, group.ID, d1)
	require.NoError(t, ferr)
	assert.Nil(t, orphanLeft)
}

func TestDBError_DeleteDay_TXDeleteFail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr24")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 4)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	failing := testutils.FailingDB(t, db, nthFail("select", 99)) // selects OK; delete falla
	_ = failing
	deleteFailing := testutils.FailingDB(t, db, nthFail("delete", 1))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, deleteFailing)

	err := svc.DeleteDay(nil, group.ID, owner.ID, d1)

	require.EqualError(t, err, "error al borrar día de calendario")
}

// --- Stamp ----------------------------------------------------------------

func TestDBError_Stamp_OwnerError(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return nil, errors.New("boom owner")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar grupo")
}

func TestDBError_Stamp_PlanDdaoError(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return nil, errors.New("boom días del plan")
	}}
	svc := NewCalendarService(nil, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01"})

	require.EqualError(t, err, "error al buscar días del plan")
}

// Stamp tx: candidates presencial + falla del select de colisiones (#2).
func TestDBError_Stamp_DetectFailingSelect(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr25")
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan presencial dberr25"}
	require.NoError(t, db.Create(plan).Error)
	from := "10:00"
	to := "11:00"
	catalogSession, _ := referenciaCatalogSession(t, db, owner.ID, "dberr25")
	sessionID := catalogSession.ID
	location := calStrPtr(`{"lat":-31.4,"lng":-64.2}`)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &sessionID, DefaultPresencial: true, DefaultTimeFrom: utcTimeHHMM(from), DefaultTimeTo: utcTimeHHMM(to), DefaultLocation: location}
	require.NoError(t, db.Create(&pd).Error)
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, failing)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: "2999-10-01"})

	require.EqualError(t, err, "error al estampar plan")
}

// Stamp insert falla al escribir el día del plan (#1).
func TestDBError_Stamp_UpsertInsertFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr26")
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan insert fail dberr25"}
	require.NoError(t, db.Create(plan).Error)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "rest"}
	require.NoError(t, db.Create(&pd).Error)
	failing := testutils.FailingDB(t, db, nthFail("insert", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, failing)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: "2999-10-01"})

	require.EqualError(t, err, "error al estampar plan")
}
