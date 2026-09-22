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

// Asignar con sesión de catálogo soft-borrada → 422 y la tx no deja instancias.
func TestCobertura_UpsertDaySesionCatalogoSoftDeletada(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobsoft")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "cobsoft")
	require.NoError(t, daos.NewSessionDao(db).SoftDelete(nil, session.ID))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 3)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarSessionNotFound)
	var instanceCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	assert.Zero(t, instanceCount, "el rollback no deja instancias de la operación fallida")
}

// Link de session_exercises cuyo ejercicio ya no existe (borrado en la tabla):
// instantiateSession corta con ErrSessionExerciseNotFound y la tx hace rollback.
func TestCobertura_UpsertDayLinkEjercicioInexistente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "coblink")
	session, exercise := referenciaCatalogSession(t, db, owner.ID, "coblink")
	require.NoError(t, db.Delete(exercise).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 3)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSessionExerciseNotFound)
	var exerciseInstanceCount int64
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exerciseInstanceCount).Error)
	assert.Zero(t, exerciseInstanceCount, "el rollback no deja instancias de ejercicios creados antes del error")
}

// validateDayFields, solo branch de horarios: from>=to (y from == to) →
// ErrCalendarInvalidTimeRange; presencial completo válido pasa. El resto de
// las variantes vive en TestCalendarService_Task5_ValidateDayFields_Direct.
func TestCobertura_ValidateDayFieldsRangoHorario(t *testing.T) {
	svc := NewCalendarService(nil, nil, nil, nil, nil, nil, nil, nil, nil).(*calendarService)
	isPresencial := true
	loc := &trainingplan.Location{Lat: -34.6, Lng: -58.4}
	from := "10:00"
	to := "09:00"
	tooEarly := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial,
		PresencialTimeFrom: &from, PresencialTimeTo: &to, PresencialLocation: loc,
	}
	assert.ErrorIs(t, svc.validateDayFields(tooEarly, "", false), ErrCalendarInvalidTimeRange)

	equal := "10:00"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial,
		PresencialTimeFrom: &equal, PresencialTimeTo: &equal, PresencialLocation: loc,
	}
	assert.ErrorIs(t, svc.validateDayFields(req, "", false), ErrCalendarInvalidTimeRange, "from == to también es inválido")

	validTo := "11:00"
	req.PresencialTimeTo = &validTo
	require.NoError(t, svc.validateDayFields(req, "", false))
}

// Banner next_training/next_cancelled con SessionInstanceID huérfana y grupo
// sin nombre: session_name null, group_name vacío, sin error.
func TestCobertura_NextSessionBannerInstanciaHuerfanaYGrupoSinNombre(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobbanner")
	require.NoError(t, db.Model(&dbs.Group{}).Where("id = ?", group.ID).Update("name", "").Error)
	require.NoError(t, db.Model(&dbs.Team{}).Where("id = ?", group.TeamID).Update("name", "").Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	future := time.Now().AddDate(0, 0, 4)
	missing := int64(987654321)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionInstanceID: &missing}))
	cancelledDate := future.AddDate(0, 0, 1)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: cancelledDate, Kind: "cancelled", SessionInstanceID: &missing}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp.NextTraining)
	assert.Empty(t, resp.NextTraining.GroupName, "grupo sin nombre responde group_name vacío")
	assert.Nil(t, resp.NextTraining.SessionName, "instancia huérfana deja session_name null sin error")
	assert.False(t, resp.NextTraining.IsPresencial, "día asincrónico no marca presencial")
	assert.Nil(t, resp.NextTraining.PresencialTimeFrom)
	assert.Nil(t, resp.NextTraining.PresencialTimeTo)
	assert.Nil(t, resp.NextTraining.PresencialLocation)
	require.NotNil(t, resp.NextCancelled)
	assert.Equal(t, cancelledDate.Format("2006-01-02"), resp.NextCancelled.Date)
	assert.Empty(t, resp.NextCancelled.GroupName)
	assert.Nil(t, resp.NextCancelled.SessionName)
	// NextSessionBannerItem (base de NextCancelled) no lleva campos presenciales.
}

// Faltante en la doc de referencia: a diferencia del banner, GetRange SÍ rompe
// con error explícito cuando la instancia embebida no existe.
func TestCobertura_GetRangeInstanciaHuerfanaRompe(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobgetr")
	future := time.Now().AddDate(0, 0, 4)
	missing := int64(987654321)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionInstanceID: &missing}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.GetRange(nil, group.ID, owner.ID, future, future)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
}

// ---------------------------------------------------------------------------
// Paths mock (s.db == nil) restantes
// ---------------------------------------------------------------------------

// Shift sin días cerrados en el camino mock: la única salida posible es el
// error de falta de DB. La lista de filas afectada llega invertida — el camino
// mock no reordena (solo el transaccional sortea desc para el shift multi-fila).
func TestCobertura_Mock_ShiftSinDiasCerradosNoHayDB(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	d1, _ := time.Parse("2006-01-02", "2999-10-05")
	d2, _ := time.Parse("2006-01-02", "2999-10-08")
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{{GroupID: groupID, Date: d2, Kind: "rest"}, {GroupID: groupID, Date: d1, Kind: "rest"}}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: d1.Format("2006-01-02"), Days: 2})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no hay DB disponible para correr fechas")
}

// Stamp mock con plan entregado fuera del orden de secuencia: cubre el ajuste
// de min/max de targetDates; sin día cerrado ni DB aborta. El dao de calendario
// roto en la query de conflictos también devuelve su error envuelto.
func TestCobertura_Mock_StampDiasFueraDeOrdenYErrorAlValidarConflictos(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	reversedDays := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{SequenceNo: 3, Kind: "rest"}, {SequenceNo: 1, Kind: "rest"}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, reversedDays, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no hay DB disponible para estampar plan")

	errDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom")
	}}
	errorSvc := NewCalendarService(errDao, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, reversedDays, nil, nil)

	_, err = errorSvc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2999-10-01"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al validar conflictos")
}

func TestCobertura_Mock_UpsertDayUpsertRompe(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{upsertFn: func(ctx *gin.Context, day *dbs.GroupCalendarDay) error { return errors.New("boom") }}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, time.Now().AddDate(0, 0, 3), calendar.CalendarDayRequest{Kind: "rest"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al guardar día")
}

// DeleteDay mock sobre un día abierto borra y devuelve nil; si el dao de
// borrado rompe, el error queda envuelto en vez de propagarse crudo.
func TestCobertura_Mock_DeleteDayCaminoExitosoYError(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	deleted := false
	future := time.Now().AddDate(0, 0, 3)
	calDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			return &dbs.GroupCalendarDay{GroupID: groupID, Date: future, Kind: "rest"}, nil
		},
		deleteFn: func(ctx *gin.Context, groupID int64, date time.Time) error { deleted = true; return nil },
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 7, future)
	require.NoError(t, err)
	assert.True(t, deleted)

	errDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			return &dbs.GroupCalendarDay{GroupID: groupID, Date: future, Kind: "rest"}, nil
		},
		deleteFn: func(ctx *gin.Context, groupID int64, date time.Time) error { return errors.New("boom") },
	}
	errorSvc := NewCalendarService(errDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err = errorSvc.DeleteDay(nil, 1, 7, future)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al borrar día")
}

// Bulk mock: escritura de fechas nuevas, validación sobre un día existente,
// conservación implícita de instancia en training sin session_id, y un upsert
// roto que propaga su error crudo. (BulkRequest no lleva cancelled_reason:
// cancelado por Bulk es imposible.)
func TestCobertura_Mock_BulkEscritura(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	d1 := time.Now().AddDate(0, 0, 3)
	d2 := time.Now().AddDate(0, 0, 4)
	upsertCalls := 0
	instanceID := int64(33)
	var savedInstanceID *int64
	calDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			if date.Format("2006-01-02") == d2.Format("2006-01-02") {
				return &dbs.GroupCalendarDay{GroupID: groupID, Date: d2, Kind: "training", SessionInstanceID: &instanceID}, nil
			}
			return nil, nil
		},
		upsertFn: func(ctx *gin.Context, day *dbs.GroupCalendarDay) error {
			savedInstanceID = day.SessionInstanceID
			upsertCalls++
			return nil
		},
	}
	t.Run("rest sobre fechas nuevas se escribe", func(t *testing.T) {
		upsertCalls = 0
		svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		resp, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02")}, Kind: "rest"})
		require.NoError(t, err)
		require.Len(t, resp.Days, 2)
		assert.Equal(t, 2, upsertCalls)
		assert.Equal(t, "rest", resp.Days[0].Kind)
	})

	t.Run("rest sobre dia existente pasa la validacion", func(t *testing.T) {
		upsertCalls = 0
		savedInstanceID = nil
		svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		resp, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{d2.Format("2006-01-02")}, Kind: "rest"})
		require.NoError(t, err)
		require.Len(t, resp.Days, 1)
		assert.Equal(t, "rest", resp.Days[0].Kind)
		assert.Nil(t, savedInstanceID, "rest no conserva la instancia: solo training sin session_id lo hace")
	})

	t.Run("upsert de la escritura rompe y propaga el error", func(t *testing.T) {
		upsertCalls = 0
		errDao := &mockGroupCalendarDao{upsertFn: func(ctx *gin.Context, day *dbs.GroupCalendarDay) error { return errors.New("boom") }}
		svc := NewCalendarService(errDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{d1.Format("2006-01-02")}, Kind: "rest"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}

func TestCobertura_Mock_BulkClearBusquedaRompe(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.BulkClear(nil, 1, 7, calendar.BulkClearRequest{Dates: []string{"2999-10-01"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// ---------------------------------------------------------------------------
// Conflictos de stamp con Postgres real
// ---------------------------------------------------------------------------

// Stamp force=false con varias fechas ocupadas → 409 listando TODAS las fechas.
func TestCobertura_StampConflictoListaTodasLasFechas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cob409")
	plan := exclPlan(t, db, owner.ID, "plan cob409", []string{"rest", "rest"}, nil)
	start := time.Now().AddDate(0, 0, 3)
	d1, d2 := start, start.AddDate(0, 0, 1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d2, Kind: "other", OtherName: calStrPtr("taller")}))
	svc := exclSvc(db)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: d1.Format("2006-01-02")})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarStampConflict)
	assert.Contains(t, err.Error(), d1.Format("2006-01-02"))
	assert.Contains(t, err.Error(), d2.Format("2006-01-02"))
	var instanceCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	assert.Zero(t, instanceCount)
}

// Mezcla de conflicto y día cerrado: 422 gana sobre el 409 (los conflictos
// solo se reportan cuando nada está cerrado) y force NO saltea el guard.
func TestCobertura_StampConflictMezclaCerradoGana(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "cobmix")
	plan := exclPlan(t, db, owner.ID, "plan cobmix", []string{"rest", "rest"}, nil)
	today := time.Now()
	d1, d2 := today, today.AddDate(0, 0, 1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	// Day 2 ocupado (conflicto) + day 1 async hoy (cerrado).
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d2, Kind: "rest"}))
	svc := exclSvc(db)
	startStr := d1.Format("2006-01-02")

	for _, force := range []bool{false, true} {
		_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: startStr, Force: force})
		require.Error(t, err, "force=%v", force)
		assert.ErrorIs(t, err, ErrCalendarDayClosed, "force=%v", force)
		assert.Contains(t, err.Error(), startStr)
		kept, findErr := calendarDao.FindByGroupAndDate(nil, group.ID, d2)
		require.NoError(t, findErr)
		require.NotNil(t, kept, "rollback deja la fila existente intacta")
		assert.Equal(t, "rest", kept.Kind)
		var instanceCount int64
		require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
		assert.Zero(t, instanceCount, "force=%v", force)
	}
}
