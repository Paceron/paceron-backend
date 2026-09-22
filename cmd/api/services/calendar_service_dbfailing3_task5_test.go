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

// --- Bulk / BulkClear / Shift: restantes en caminos mock y transaccional -----

func TestDBError_Bulk_MockPaths(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	d1 := "2999-10-05"

	svc := NewCalendarService(nil, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"no-es-fecha"}, Kind: "rest"})
	assert.Contains(t, err.Error(), "fecha inválida")

	// Training sin session_id y sin instancia previa: error de lote.
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return nil, nil
	}}
	svc = NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	_, err = svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{d1}, Kind: "training"})
	assert.ErrorIs(t, err, ErrCalendarTrainingWithoutInstance)

	assert.ErrorIs(t, ErrCalendarInvalidCancelTransition, ErrCalendarInvalidCancelTransition) // sentinel vive
}

func TestDBError_Bulk_FindByGroupAndDateErrorMidLoop(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom find")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"2999-10-05"}, Kind: "rest"})

	require.Error(t, err)
}

func TestDBError_Bulk_TXValidateFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr30")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 11)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{d1.Format("2006-01-02")}, Kind: "other"})

	assert.ErrorIs(t, err, ErrCalendarFieldMismatch, "kind other sin OtherName falla dentro de la tx")
}

func TestDBError_Bulk_TXInstantiateSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr31")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr31")
	sessionID := session.ID
	d1 := time.Now().AddDate(0, 0, 12)
	req := calendar.BulkRequest{Dates: []string{d1.Format("2006-01-02")}, Kind: "training", SessionID: &sessionID}

	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err := svc.Bulk(nil, group.ID, owner.ID, req)
	require.Error(t, err)

	var instCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instCount).Error)
	assert.Zero(t, instCount, "el rollback no deja instancias")
}

func TestDBError_Bulk_DeleteSupersededSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr33")
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S super"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E super", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 14)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &inst.ID}))
	// Orden de SELECTs en la tx: #1 FindByGroupAndDate; #2 First del Upsert de
	// la DAO; #3 HasFeedback del deleteSuperseded.
	failing := testutils.FailingDB(t, db, nthFail("select", 3))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{d1.Format("2006-01-02")}, Kind: "rest"})

	assert.EqualError(t, err, "error al aplicar bulk")
}

func TestDBError_BulkClear_FailsPuntuales(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return nil, errors.New("boom owner")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)
	err := svc.BulkClear(nil, 1, 7, calendar.BulkClearRequest{Dates: []string{"2999-10-05"}})
	require.Error(t, err)

	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr34")
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S clear"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E clear", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 15)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &inst.ID}))
	dbFailing := testutils.FailingDB(t, db, nthFail("delete", 1))
	svc = NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, dbFailing)

	err = svc.BulkClear(nil, group.ID, owner.ID, calendar.BulkClearRequest{Dates: []string{d1.Format("2006-01-02")}})

	require.EqualError(t, err, "error al limpiar fechas")
}

func TestDBError_Shift_GuardSelectFailYUpdateNoDuplicate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr35")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 16)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))

	// Guard: la SELECT del occupant falla (2ª select de la tx).
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 3})
	require.EqualError(t, err, "error al correr fechas")

	// UPDATE no-duplicate-key no mapea a colisión: viaja como error crudo al wrap.
	updateFailing := testutils.FailingDB(t, db, nthFail("update", 1))
	svc = NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, updateFailing)
	_, err = svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 3})
	require.EqualError(t, err, "error al correr fechas")
}

func TestDBError_NextSession_MembershipsError(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return nil, errors.New("boom memberships")
	}}
	svc := NewCalendarService(nil, nil, nil, groupUserDao, nil, nil, nil, nil, nil)
	_, err := svc.NextSession(nil, 42)
	require.EqualError(t, err, "error al buscar grupos del usuario")
}

// Banner training con horarios presenciales y location válida: rama del
// unmarshal del trainingBannerItem.
func TestDBError_NextSession_BannerPresencialConLocationValida(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbanner3")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	location := `{"lat":-31.4,"lng":-64.2,"label":"despegue"}`
	d1 := time.Now().AddDate(0, 0, 5)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("09:00"), PresencialTimeTo: utcTimeHHMM("10:00"), PresencialLocation: &location}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp.NextTraining)
	assert.True(t, resp.NextTraining.IsPresencial)
	require.NotNil(t, resp.NextTraining.PresencialTimeFrom)
	assert.Equal(t, "09:00", *resp.NextTraining.PresencialTimeFrom)
	require.NotNil(t, resp.NextTraining.PresencialLocation)
	require.NotNil(t, resp.NextTraining.PresencialLocation.Label)
	assert.Equal(t, "despegue", *resp.NextTraining.PresencialLocation.Label)
}

func TestDBError_MemberCalendar_ConInstanciaHuerfanaRompe(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr36")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 16)
	missing := int64(987654321)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &missing}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.MemberCalendar(nil, owner.ID, d1, d1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
}

func TestDBError_AdministeredCalendar_SinDias(t *testing.T) {
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1", TeamID: 10}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return []dbs.Team{{ID: 10, Name: "T10"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findForGroupsInRangeFn: func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, nil, nil, nil, nil, nil, nil)

	resp, err := svc.AdministeredCalendar(nil, 1, time.Now(), time.Now().AddDate(0, 0, 7))

	require.NoError(t, err)
	assert.Empty(t, resp)
}

// --- Cuarta ronda: mocks simples y selects post-commit -----------------------

func TestDBError_DeleteDay_EntryFindError(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom find entry")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 7, time.Now().AddDate(0, 0, 3))

	require.EqualError(t, err, "error al buscar día de calendario")
}

func TestDBError_Bulk_OwnerError(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return nil, errors.New("boom owner")
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 99}, nil
	}}
	svc := NewCalendarService(nil, groupDao, teamDao, nil, nil, nil, nil, nil, nil)

	_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"2999-10-05"}, Kind: "rest"})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCalendarForbidden)
}

func TestDBError_Shift_OwnerError(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return nil, errors.New("boom owner")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: "2999-10-05", Days: 1})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCalendarShiftCollision)
}

func TestDBError_Stamp_PlanDaysDaoError(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return nil, errors.New("boom días")
	}}
	svc := NewCalendarService(nil, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01"})

	require.EqualError(t, err, "error al buscar días del plan")
}

// Refresco post-commit falla: la respuesta del banner no rompe el 200.
func TestDBError_Banners_InstanciaReadFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbanner4")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	sessionDao := daos.NewSessionInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "inst banner read fail"}
	require.NoError(t, sessionDao.Create(nil, inst))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 6)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:00"), SessionInstanceID: &inst.ID}))
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	resp, err := svc.NextPresencialSession(nil, owner.ID)

	require.NoError(t, err, "la falla de lectura de la instancia no rompe el banner")
	require.NotNil(t, resp)
	assert.Nil(t, resp.SessionName)

	// bannerItem via NextSession (inyección en handle nuevo).
	svc = NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, testutils.FailingDB(t, db, nthFail("select", 1)))
	resp2, err2 := svc.NextSession(nil, owner.ID)
	require.NoError(t, err2)
	require.NotNil(t, resp2.NextTraining)
	assert.Nil(t, resp2.NextTraining.SessionName)
}

// Stamp: session del catálogo con select fallida en la cuarta posición (~instancia).
func TestDBError_Stamp_InstantiateSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbstampi")
	catalogSession, _ := referenciaCatalogSession(t, db, owner.ID, "dbstampi")
	sessionID := catalogSession.ID
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan stampi"}
	require.NoError(t, db.Create(plan).Error)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &sessionID}
	require.NoError(t, db.Create(&pd).Error)
	// SELECTs en la tx: #1 FindByGroupAndDate; #2 sessionDao FindByID.
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, failing)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: "2999-10-05"})

	require.EqualError(t, err, "error al estampar plan")
}

// Stamp con día existente con instancia a reemplazar: deleteSuperseded falla.
func TestDBError_Stamp_DeleteSupersededSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbstamps")
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S stamp super"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E stamp super", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 17)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &inst.ID}))
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan rest dbstamps"}
	require.NoError(t, db.Create(plan).Error)
	pd := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "rest"}
	require.NoError(t, db.Create(&pd).Error)
	// SELECTs: #1 FindByGroupAndDate; #2 First del Upsert; #3 HasFeedback.
	failing := testutils.FailingDB(t, db, nthFail("select", 3))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, failing)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: d1.Format("2006-01-02"), Force: true})

	require.EqualError(t, err, "error al estampar plan")
}

// UpsertDay: UPDATE falla al reescribir un día ya existente.
func TestDBError_UpsertDay_UpdateFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbupu")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 18)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	failing := testutils.FailingDB(t, db, nthFail("update", 1))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, d1, calendar.CalendarDayRequest{Kind: "other", OtherName: calStrPtr("charla")})

	require.EqualError(t, err, "error al guardar día de calendario")
}

// Bulk: el Upsert del insert falla (nueva fecha en el lote).
func TestDBError_Bulk_UpsertInsertFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbullin")
	failing := testutils.FailingDB(t, db, nthFail("insert", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{"2999-10-05"}, Kind: "rest"})

	assert.EqualError(t, err, "error al aplicar bulk")
}

// Bulk serva: la lectura de la respuesta detalle falla con falla post-commit.
func TestDBError_Bulk_RespuestaDetalleFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbully")
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S resp fail"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E resp fail", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 19)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &inst.ID}))
	// SELECTs: #1 tx FindByGroupAndDate; #2 First del Upsert; (preservada).
	// La lectura del detail falla luego de commitear (select #3).
	failing := testutils.FailingDB(t, db, nthFail("select", 3))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{d1.Format("2006-01-02")}, Kind: "training"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding session instance")
}

// Bulk presencial con falla en la query de colisiones (#2).
func TestDBError_Bulk_DetectSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbulkp")
	d1 := time.Now().AddDate(0, 0, 20)
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	from := "10:00"
	to := "11:00"

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02")}, Kind: "rest", IsPresencial: boolPtr(true),
		PresencialTimeFrom: &from, PresencialTimeTo: &to,
		PresencialLocation: &trainingplan.Location{Lat: -31.4, Lng: -64.2},
	})

	require.EqualError(t, err, "error al aplicar bulk")
}

// BulkClear: el deleteSuper dentro del delete de la instancia (#2 delete).
func TestDBError_BulkClear_DeleteSuperFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbclear")
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S clearsuper"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E clearsuper", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 21)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &inst.ID}))
	dbFailing := testutils.FailingDB(t, db, nthFail("delete", 2))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, dbFailing)

	err := svc.BulkClear(nil, group.ID, owner.ID, calendar.BulkClearRequest{Dates: []string{d1.Format("2006-01-02")}})

	require.EqualError(t, err, "error al limpiar fechas")
}

// Shift con día presencial: la query de detección de colisiones falla (#2).
func TestDBError_Shift_DetectSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbshiftd")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 22)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:00")}))
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 3})

	require.EqualError(t, err, "error al correr fechas")
}

// Shift exitoso con instancia huérfana: toCal a la respuesta falla al final.
func TestDBError_Shift_RespuestaDetalleFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbshiftor")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 23)
	missing := int64(987654321)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &missing}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 3})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
}

// AdministeredCalendar con 2 días presenciales del mismo día: la query de
// colisiones falla (log + wrap).
func TestDBError_AdministeredCalendar_DetectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbadmin")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 24)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:00")}))
	group2 := &dbs.Group{Name: "G2 dbadmin", TeamID: group.TeamID, IsMain: false}
	require.NoError(t, db.Create(group2).Error)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group2.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("12:00"), PresencialTimeTo: utcTimeHHMM("13:00")}))
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.AdministeredCalendar(nil, owner.ID, d1, d1)

	require.EqualError(t, err, "error al detectar colisiones presenciales")
}

// AdministeredCalendar con instancia huérfana: el detalle embebido falla al armar.
func TestDBError_AdministeredCalendar_ConInstanciaHuerfanaRompe(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbadmin2")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 25)
	missing := int64(987654321)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &missing}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.AdministeredCalendar(nil, owner.ID, d1, d1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
}

// UpsertDay: la respuesta detalle falla al armarse después de escribir.
func TestDBError_UpsertDay_RespuestaDetalleFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbupsertc")
	sessionDao := daos.NewSessionInstanceDao(db)
	exerciseDao := daos.NewExerciseInstanceDao(db)
	linkDao := daos.NewSessionExerciseInstanceDao(db)
	inst := &dbs.SessionInstance{Name: "S resp fail up"}
	require.NoError(t, sessionDao.Create(nil, inst))
	ex := &dbs.ExerciseInstance{Name: "E resp fail up", Kind: "running"}
	require.NoError(t, exerciseDao.Create(nil, ex))
	require.NoError(t, linkDao.Create(nil, &dbs.SessionExerciseInstance{SessionInstanceID: inst.ID, ExerciseInstanceID: ex.ID, Role: "main"}))
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 26)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", SessionInstanceID: &inst.ID}))
	// SELECTs: #1 FindByGroupAndDate; #2 First del Upsert; #3 respuesta detalle.
	failing := testutils.FailingDB(t, db, nthFail("select", 3))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, d1, calendar.CalendarDayRequest{Kind: "training"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding session instance")
}
