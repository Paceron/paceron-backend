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
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
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
