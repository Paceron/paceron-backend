package services

import (
	"errors"
	"math"
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

// ---------------------------------------------------------------------------
// Ramas de error de DB de calendar_service: inyección vía testutils.FailingDB
// (condición por TIPO de operación) y mocks de DAO para las envolturas que no
// tocan s.db. Los fmt.Errorf de calendar_service usan %w (ErrorIs con sentinel)
// y los errores de nivel-top son mensajes contractuales (EqualError).
// ---------------------------------------------------------------------------

// nthFailing arma una condición que falla la llamada N-ésima de una operación
// dada (útil para que fallen selecciones SQL puntuales del encadenamiento dentro
// de una transacción sin fallar las anteriores).
func nthFail(op string, n int) func(string) bool {
	calls := map[string]int{}
	return func(o string) bool {
		calls[o]++
		return o == op && calls[o] == n
	}
}

// --- GetRange ---------------------------------------------------------------

func TestDBError_GetRange_ListCalendarDaoError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr01")
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(failing), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.GetRange(nil, group.ID, owner.ID, time.Now(), time.Now())

	require.EqualError(t, err, "error al listar calendario")
}

// --- isGroupOwnerOrMember / findPresencialCollisions ------------------------

func TestDBError_IsGroupOwnerOrMember_RelayaErrorNoForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return nil, errors.New("db exploto")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)

	err := svc.(*calendarService).isGroupOwnerOrMember(nil, 1, 7)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar grupo")
}

func TestDBError_IsGroupOwnerOrMember_MembershipQueryError(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 99}, nil
	}}
	groupUserDao := &mockGroupUserDao{findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
		return nil, errors.New("boom membresía")
	}}
	svc := NewCalendarService(nil, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)

	err := svc.(*calendarService).isGroupOwnerOrMember(nil, 1, 7)

	require.EqualError(t, err, "error al validar membresía")
}

func TestDBError_findPresencialCollisions_DaoOwnerError(t *testing.T) {
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return nil, errors.New("boom owner")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)

	_, _, err := svc.(*calendarService).findPresencialCollisions(nil, nil, 1, nil, nil, []time.Time{time.Now()}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar grupos del owner")
}

func TestDBError_findPresencialCollisions_DaoTeamError(t *testing.T) {
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1}}, nil
	}}
	teamDao := &mockTeamDao{getAllByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Team, error) {
		return nil, errors.New("boom teams")
	}}
	svc := NewCalendarService(nil, groupDao, teamDao, nil, nil, nil, nil, nil, nil)

	_, _, err := svc.(*calendarService).findPresencialCollisions(nil, nil, 1, nil, nil, []time.Time{time.Now()}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar equipos del owner")
}

// Días existentes excluidos por ID y por grupo: se saltan del cruce y el mismo
// grupo nunca choca consigo mismo. Corre contra Postgres real porque
// findPresencialCollisions arma su propia DAO sobre s.db.
func TestDBError_findPresencialCollisions_SkipExcludedDayAndGrupo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr10")
	group2 := &dbs.Group{Name: "G2 dberr10", TeamID: group.TeamID, IsMain: false}
	require.NoError(t, db.Create(group2).Error)
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 10)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:00")}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	candidates := []dbs.GroupCalendarDay{{
		GroupID: group2.ID, Date: d1, Kind: "training", IsPresencial: true,
		PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:30"),
	}}
	existing, err := dao.FindPresencialForGroupsInRange(nil, []int64{group.ID}, []time.Time{d1})
	require.NoError(t, err)
	require.Len(t, existing, 1)
	existingDayID := existing[0].ID

	// El día excluido por ID se saltea del cruce (branch de excludeDayIDs).
	cross, same, err := svc.(*calendarService).findPresencialCollisions(nil, db, owner.ID, nil, []int64{existingDayID}, []time.Time{d1}, candidates)
	require.NoError(t, err)
	assert.Empty(t, cross)
	assert.Empty(t, same, "el único colisionante está excluido por excludeDayIDs")

	// Con excludeGroupID apuntando al grupo del día existente también se salta.
	cross, same, err = svc.(*calendarService).findPresencialCollisions(nil, db, owner.ID, &group.ID, nil, []time.Time{d1}, candidates)
	require.NoError(t, err)
	assert.Empty(t, cross)
	assert.Empty(t, same)

	// Sin exclusiones el colisionante es del MISMO equipo → same-team warning.
	cross, same, err = svc.(*calendarService).findPresencialCollisions(nil, db, owner.ID, nil, nil, []time.Time{d1}, candidates)
	require.NoError(t, err)
	assert.Empty(t, cross)
	require.Len(t, same, 1)
	assert.Equal(t, "10:00-11:00", same[0].PresencialTimeFrom+"-"+same[0].PresencialTimeTo)
}

func TestDBError_findPresencialCollisions_DayGroupDesconocidoSaltea(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr11")
	dao := daos.NewGroupCalendarDayDao(db)
	d1 := time.Now().AddDate(0, 0, 10)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:00")}))
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	// Sin groupByID no hay grupo para el día existente fantasma.
	cross, same, err := svc.(*calendarService).findPresencialCollisions(nil, db, owner.ID, nil, nil, []time.Time{d1}, []dbs.GroupCalendarDay{{
		GroupID: 42, Date: d1, Kind: "training", IsPresencial: true,
		PresencialTimeFrom: utcTimeHHMM("10:00"), PresencialTimeTo: utcTimeHHMM("11:00"),
	}})

	require.NoError(t, err)
	assert.Empty(t, cross)
	assert.Empty(t, same, "día de un grupo que el owner ya no administra no choca")
}

// s.db falsificado con select-fail: la query de días presenciales falla.
func TestDBError_findPresencialCollisions_QueryPresencialError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1", TeamID: 10}}, nil
	}}
	teamDao := &mockTeamDao{getAllByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Team, error) {
		return []dbs.Team{{ID: 10, Name: "T10"}}, nil
	}}
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(nil, groupDao, teamDao, nil, nil, nil, nil, nil, failing)

	_, _, err := svc.(*calendarService).findPresencialCollisions(nil, failing, 1, nil, nil, []time.Time{time.Now()}, []dbs.GroupCalendarDay{{GroupID: 1, Kind: "training", IsPresencial: true}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar días presenciales")
}

// --- UpsertDay camino mock --------------------------------------------------

func TestDBError_UpsertDay_Mock_UpsertDaoError(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{upsertFn: func(ctx *gin.Context, day *dbs.GroupCalendarDay) error {
		return errors.New("boom upsert")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, time.Now().AddDate(0, 0, 2), calendar.CalendarDayRequest{Kind: "rest"})

	require.EqualError(t, err, "error al guardar día de calendario")
}

func TestDBError_UpsertDay_Mock_OtroNombreExitoso(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.UpsertDay(nil, 1, 7, time.Now().AddDate(0, 0, 5), calendar.CalendarDayRequest{Kind: "other", OtherName: calStrPtr("charla")})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "charla", *resp.OtherName)
}

// Location con float NaN: json.Marshal falla de forma determinista y cubre la
// rama de error de buildRow y de jsonMarshalLocation (dato inválido real,
// no un hack).
func TestDBError_UpsertDay_Mock_LocationNoSerializable(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	isPresencial := true
	from := "09:00"
	to := "10:00"

	_, err := svc.UpsertDay(nil, 1, 7, time.Now().AddDate(0, 0, 5), calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial, PresencialTimeFrom: &from, PresencialTimeTo: &to,
		PresencialLocation: &trainingplan.Location{Lat: math.NaN(), Lng: -34.6},
	})

	require.Error(t, err)
	assert.EqualError(t, err, "error al serializar ubicación")
}

// --- UpsertDay camino transaccional con selects fallidas puntuales ----------

func TestDBError_UpsertDay_TXSelectDaoError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr02")
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, time.Now().AddDate(0, 0, 3), calendar.CalendarDayRequest{Kind: "rest"})

	assert.EqualError(t, err, "error al guardar día de calendario")
	assert.NotErrorIs(t, err, ErrCalendarDayClosed, "la falla de DB no debe mapear al sentinel de día cerrado")
}

// instantiateSession: la 2ª SELECT de la tx es sessionDao.FindByID, la 3ª es
// FindBySession (ejercicios). Cada falla rompe y hace rollback.
func TestDBError_InstantiateSession_SelectFailsEnTx(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr03")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr03")
	date := time.Now().AddDate(0, 0, 5)

	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
	require.EqualError(t, err, "error al guardar día de calendario")

	failing = testutils.FailingDB(t, db, nthFail("select", 3))
	svc = NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)
	_, err = svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
	require.EqualError(t, err, "error al guardar día de calendario")

	var instCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instCount).Error)
	assert.Zero(t, instCount, "el rollback no deja instancias de las operaciones fallidas")
	_ = owner
}

// instantiateSession: primer INSERT (sesión instancia) falla.
func TestDBError_InstantiateSession_CreateFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr04")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr04")
	failing := testutils.FailingDB(t, db, nthFail("insert", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, time.Now().AddDate(0, 0, 4), calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})

	require.EqualError(t, err, "error al guardar día de calendario")
}

// --- deleteSupersededInstance dentro de la tx con selects/delete fallidas ---

func TestDBError_UpsertDay_DeleteSupersededSelectFails(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr05")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "dberr05")
	dao := daos.NewGroupCalendarDayDao(db)
	date := time.Now().AddDate(0, 0, 5)
	svc := NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	// Día existente con instancia propia; reasignar a REST borra la superada.
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &session.ID}))
	// Orden de SELECTs de la 2ª llamada: FindByGroupAndDate (1),
	// sessionDao.FindByID (2), FindBySession links (3), HasFeedback (4).
	failing := testutils.FailingDB(t, db, nthFail("select", 4))
	svc = NewCalendarService(dao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "rest"})

	require.Error(t, err)
	assert.EqualError(t, err, "error al guardar día de calendario")
}

// Bulk/Stamp/BulkClear/Shift: selects puntuales fallando dentro de la tx.
func TestDBError_Stamp_TXSelectDaoError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr06")
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan dberr06"}
	require.NoError(t, db.Create(plan).Error)
	sessionDay := dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "rest"}
	require.NoError(t, db.Create(&sessionDay).Error)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, failing)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: "2999-10-01"})

	require.Error(t, err)
	assert.EqualError(t, err, "error al estampar plan")
}

func TestDBError_Bulk_TXSelectDaoError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr07")
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{"2999-10-01", "2999-10-02"}, Kind: "rest"})

	assert.EqualError(t, err, "error al aplicar bulk")
	assert.NotErrorIs(t, err, ErrCalendarDayClosed)
}

func TestDBError_BulkClear_TXSelectDaoError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr07")
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	err := svc.BulkClear(nil, group.ID, owner.ID, calendar.BulkClearRequest{Dates: []string{"2999-10-01"}})

	assert.EqualError(t, err, "error al limpiar fechas")
	assert.NotErrorIs(t, err, ErrCalendarDayClosed)
}

func TestDBError_Shift_TXSelectDaoError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dberr08")
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, failing)

	_, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: "2999-10-01", Days: 2})

	assert.EqualError(t, err, "error al correr fechas")
	assert.NotErrorIs(t, err, ErrCalendarShiftCollision)
}

// --- NextSession / banners / calendarios agregados con mocks -----------------

func TestDBError_NextSession_CancelledFindError(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextForGroupsByKindFn: func(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom next")
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	_, err := svc.NextSession(nil, 42)

	require.EqualError(t, err, "error al buscar próxima sesión")
}

func TestDBError_NextSession_TrainingFindError(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextForGroupsByKindFn: func(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		if kind == "cancelled" {
			return nil, nil
		}
		return nil, errors.New("boom training")
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	_, err := svc.NextSession(nil, 42)

	require.EqualError(t, err, "error al buscar próxima sesión")
}

func TestDBError_NextPresencialSession_DaoErrors(t *testing.T) {
	// groupDao.
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return nil, errors.New("boom admin groups")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.NextPresencialSession(nil, 1)
	require.EqualError(t, err, "error al buscar grupos administrados")

	// findNextPresencial.
	groupDao = &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextPresencialFn: func(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom next presencial")
	}}
	svc = NewCalendarService(calDao, groupDao, &mockTeamDao{}, nil, nil, nil, nil, nil, nil)
	_, err = svc.NextPresencialSession(nil, 1)
	require.EqualError(t, err, "error al buscar próxima sesión presencial")

	// teams para el banner.
	groupDao = &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1", TeamID: 10}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return nil, errors.New("boom teams banner")
	}}
	calDao = &mockGroupCalendarDao{findNextPresencialFn: func(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: 1, Date: today.AddDate(0, 0, 1), Kind: "training", IsPresencial: true}, nil
	}}
	svc = NewCalendarService(calDao, groupDao, teamDao, nil, nil, nil, nil, nil, nil)
	_, err = svc.NextPresencialSession(nil, 1)
	require.EqualError(t, err, "error al buscar equipos del entrenador")
}

// Banner del entrenador con instancia real contra Postgres real: session_name
// sale de la lectura de la instancia embebida.
func TestDBError_NextPresencialSession_BannerConInstanciaReal(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "dbbanner")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	inst := &dbs.SessionInstance{Name: "instancia banner"}
	require.NoError(t, db.Create(inst).Error)
	d1 := time.Now().AddDate(0, 0, 3)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("09:00"), PresencialTimeTo: utcTimeHHMM("10:00"), SessionInstanceID: &inst.ID}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.NextPresencialSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.SessionName)
	assert.Equal(t, inst.Name, *resp.SessionName)
	require.NotNil(t, resp.PresencialTimeFrom)
	require.NotNil(t, resp.PresencialTimeTo)
}

// groupNamesByIDs: errores de grupo y de team envuelven igual; y la rama del
// fallback por team_name para grupos sin nombre sale bien.
func TestDBError_groupNamesByIDs_DaoErrors(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return nil, errors.New("boom grupos")
	}}
	svc := NewCalendarService(nil, groupDao, nil, groupUserDao, nil, nil, nil, nil, nil)
	_, err := svc.NextSession(nil, 42)
	require.EqualError(t, err, "error al buscar grupos del usuario")

	// Grupo sin nombre y team lookup err → mismo envoltorio.
	groupDao = &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "", TeamID: 10}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return nil, errors.New("boom team find")
	}}
	svc = NewCalendarService(nil, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)
	_, err = svc.NextSession(nil, 42)
	require.EqualError(t, err, "error al buscar grupos del usuario")

	// Grupo sin nombre, team existe → el nombre sale del team.
	teamDao = &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return []dbs.Team{{ID: 10, Name: "T10"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextForGroupsByKindFn: func(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		if kind == "cancelled" {
			return nil, nil
		}
		return &dbs.GroupCalendarDay{GroupID: 1, Date: today.AddDate(0, 0, 1), Kind: "training"}, nil
	}}
	svc = NewCalendarService(calDao, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)
	resp, err := svc.NextSession(nil, 42)
	require.NoError(t, err)
	require.NotNil(t, resp.NextTraining)
	assert.Equal(t, "T10", resp.NextTraining.GroupName)
}

func TestDBError_MemberCalendar_DaoErrors(t *testing.T) {
	from, to := time.Now(), time.Now().AddDate(0, 0, 7)
	d1, _ := time.Parse("2006-01-02", "2999-10-05")

	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return nil, errors.New("boom memberships")
	}}
	svc := NewCalendarService(nil, nil, nil, groupUserDao, nil, nil, nil, nil, nil)
	_, err := svc.MemberCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al buscar grupos del usuario")

	groupUserDao = &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	calDao := &mockGroupCalendarDao{findForGroupsInRangeFn: func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom days")
	}}
	svc = NewCalendarService(calDao, nil, nil, groupUserDao, nil, nil, nil, nil, nil)
	_, err = svc.MemberCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al listar calendario")

	groupUserDao = &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	calDao = &mockGroupCalendarDao{findForGroupsInRangeFn: func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{{GroupID: 1, Date: d1, Kind: "rest"}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return nil, errors.New("boom groups")
	}}
	svc = NewCalendarService(calDao, groupDao, nil, groupUserDao, nil, nil, nil, nil, nil)
	_, err = svc.MemberCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al buscar grupos del usuario")

	groupDao = &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1", TeamID: 10}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return nil, errors.New("boom teams")
	}}
	svc = NewCalendarService(calDao, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)
	_, err = svc.MemberCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al buscar equipos del usuario")
}

func TestDBError_AdministeredCalendar_DaoErrors(t *testing.T) {
	from, to := time.Now(), time.Now().AddDate(0, 0, 7)

	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return nil, errors.New("boom owner groups")
	}}
	svc := NewCalendarService(nil, groupDao, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.AdministeredCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al buscar grupos administrados")

	groupDao = &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "G1", TeamID: 10}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return nil, errors.New("boom teams")
	}}
	svc = NewCalendarService(nil, groupDao, teamDao, nil, nil, nil, nil, nil, nil)
	_, err = svc.AdministeredCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al buscar equipos del entrenador")

	teamDao = &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return []dbs.Team{{ID: 10, Name: "T10"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findForGroupsInRangeFn: func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom days")
	}}
	svc = NewCalendarService(calDao, groupDao, teamDao, nil, nil, nil, nil, nil, nil)
	_, err = svc.AdministeredCalendar(nil, 1, from, to)
	require.EqualError(t, err, "error al listar calendario")
}

func TestDBError_CalendarSummary_DaoErrors(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return nil, errors.New("boom memberships")
	}}
	svc := NewCalendarService(nil, nil, nil, groupUserDao, nil, nil, nil, nil, nil)
	_, err := svc.CalendarSummary(nil, 1)
	require.EqualError(t, err, "error al buscar grupos del usuario")

	// group nil (deleted) → item skipped sin error.
	groupUserDao = &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return nil, nil
	}}
	svc = NewCalendarService(nil, groupDao, nil, groupUserDao, nil, nil, nil, nil, nil)
	items, err := svc.CalendarSummary(nil, 1)
	require.NoError(t, err)
	assert.Empty(t, items)
}
