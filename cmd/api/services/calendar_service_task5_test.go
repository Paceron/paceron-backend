package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/testutils"
)

func task5BaseAuthDaos() (daos.GroupDaoInterface, daos.TeamDaoInterface) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	return groupDao, teamDao
}

// ---------------------------------------------------------------------------
// Mock-level tests (db == nil): guards, validación y errores de DAO
// ---------------------------------------------------------------------------

func TestCalendarService_Task5_NewDayOnClosedDateRejected(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	past := time.Now().AddDate(0, 0, -1)
	wrote := false
	calDao := &mockGroupCalendarDao{
		findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) { return nil, nil },
		upsertFn:             func(ctx *gin.Context, day *dbs.GroupCalendarDay) error { wrote = true; return nil },
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, past, calendar.CalendarDayRequest{Kind: "rest"})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	assert.Contains(t, err.Error(), past.Format("2006-01-02"), "el error debe listar la fecha cerrada")
	assert.False(t, wrote, "no debe escribirse nada sobre un día cerrado")
}

func TestCalendarService_Task5_IsGroupOwner_FailureVariants(t *testing.T) {
	baseGroupDao := func() daos.GroupDaoInterface {
		return &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 1}, nil
		}}
	}

	t.Run("grupo no encontrado", func(t *testing.T) {
		groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return nil, nil }}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "rest"})
		assert.ErrorIs(t, err, ErrCalendarGroupNotFound)
	})

	t.Run("dao de grupo rompe", func(t *testing.T) {
		groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) { return nil, errors.New("boom") }}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "rest"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error al buscar grupo")
		assert.NotErrorIs(t, err, ErrCalendarForbidden)
	})

	t.Run("equipo inexistente queda prohibido", func(t *testing.T) {
		groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 999}, nil
		}}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "rest"})
		assert.ErrorIs(t, err, ErrCalendarForbidden)
	})

	t.Run("dao de equipo rompe", func(t *testing.T) {
		svc := NewCalendarService(&mockGroupCalendarDao{}, baseGroupDao(), &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("boom")
		}}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		err := svc.(*calendarService).isGroupOwner(nil, 1, 7)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error al buscar equipo")
	})

	t.Run("miembro del grupo puede leer", func(t *testing.T) {
		groupUserDao := &mockGroupUserDao{findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{GroupID: groupID, UserID: userID}, nil
		}}
		groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 1}, nil
		}}
		teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return nil, nil }}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)
		_, err := svc.GetRange(nil, 1, 42, time.Now(), time.Now())
		require.NoError(t, err)
	})

	t.Run("dao de membresía rompe en lectura", func(t *testing.T) {
		groupUserDao := &mockGroupUserDao{findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, errors.New("boom")
		}}
		groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: id, TeamID: 1}, nil
		}}
		teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return nil, nil }}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)
		_, err := svc.GetRange(nil, 1, 42, time.Now(), time.Now())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error al validar membresía")
	})
}

func TestCalendarService_Task5_GetRange_DaoErrorWrapped(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.GetRange(nil, 1, 7, time.Now(), time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar calendario")
}

func TestCalendarService_Task5_UpsertDay_FindErrorWrapped(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return nil, errors.New("boom")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "rest"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar día de calendario")
	assert.NotErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_Task5_Stamp_MockErrorBranches(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	req := calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"}

	t.Run("plan inexistente", func(t *testing.T) {
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, &mockTrainingPlanDao{}, &mockPlanDayDao{}, nil, nil)
		_, err := svc.Stamp(nil, 1, 7, req)
		assert.ErrorIs(t, err, ErrCalendarPlanNotFound)
	})

	t.Run("dao del plan rompe", func(t *testing.T) {
		planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			return nil, errors.New("boom")
		}}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, &mockPlanDayDao{}, nil, nil)
		_, err := svc.Stamp(nil, 1, 7, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error al buscar plan")
		assert.NotErrorIs(t, err, ErrCalendarPlanNotFound)
	})

	t.Run("dao de días del plan rompe", func(t *testing.T) {
		planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
		}}
		dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
			return nil, errors.New("boom")
		}}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)
		_, err := svc.Stamp(nil, 1, 7, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error al buscar días del plan")
	})

	t.Run("start_date con formato inválido", func(t *testing.T) {
		planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
		}}
		dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
			return []dbs.PlanDay{{SequenceNo: 1, Kind: "rest"}}, nil
		}}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)
		_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "not-a-date"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start_date debe tener formato")
	})

	t.Run("plan sin días se rechaza", func(t *testing.T) {
		planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
		}}
		dayDao := &mockPlanDayDao{}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)
		_, err := svc.Stamp(nil, 1, 7, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "el plan no tiene días")
	})

	t.Run("sin conflictos y sin DB el estampado no se commitea", func(t *testing.T) {
		planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
			return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
		}}
		dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
			return []dbs.PlanDay{{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 2, Kind: "rest"}}, nil
		}}
		svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)
		_, err := svc.Stamp(nil, 1, 7, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no hay DB disponible para estampar plan")
	})
}

func TestCalendarService_Task5_Bulk_MockErrorBranches(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	t.Run("fecha inválida", func(t *testing.T) {
		_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"not-a-date"}, Kind: "rest"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fecha inválida en dates")
	})

	t.Run("cancelar día sin contenido se rechaza por combinación de campos", func(t *testing.T) {
		_, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"2999-10-01"}, Kind: "cancelled"})
		assert.ErrorIs(t, err, ErrCalendarFieldMismatch)
	})

	t.Run("el error lista todas las fechas cerradas", func(t *testing.T) {
		past, pastTwo := time.Now().AddDate(0, 0, -2), time.Now().AddDate(0, 0, -1)
		future := time.Now().AddDate(0, 0, 5)
		calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
			return nil, nil // fechas nuevas: cerradas por regla de fecha si ya pasaron
		}}
		dbSvc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		_, err := dbSvc.Bulk(nil, 1, 7, calendar.BulkRequest{
			Dates: []string{past.Format("2006-01-02"), pastTwo.Format("2006-01-02"), future.Format("2006-01-02")},
			Kind:  "rest",
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCalendarDayClosed)
		assert.Contains(t, err.Error(), past.Format("2006-01-02"))
		assert.Contains(t, err.Error(), pastTwo.Format("2006-01-02"))
		assert.NotContains(t, err.Error(), future.Format("2006-01-02"))
	})
}

func TestCalendarService_Task5_BulkClear_InvalidDate(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.BulkClear(nil, 1, 7, calendar.BulkClearRequest{Dates: []string{"not-a-date"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fecha inválida en dates")
}

func TestCalendarService_Task5_Shift_MockErrorBranches(t *testing.T) {
	groupDao, teamDao := task5BaseAuthDaos()
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	t.Run("from_date inválida", func(t *testing.T) {
		_, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: "not-a-date", Days: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_date debe tener formato")
	})

	t.Run("days no positivo", func(t *testing.T) {
		_, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: "2026-10-01", Days: 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "days debe ser un entero positivo")
	})

	t.Run("dao rompe buscando filas afectadas", func(t *testing.T) {
		calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
			return nil, errors.New("boom")
		}}
		dbSvc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
		_, err := dbSvc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: "2026-10-01", Days: 1})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error al buscar filas a correr")
	})
}

func TestCalendarService_Task5_ValidateDayFields_Direct(t *testing.T) {
	svc := NewCalendarService(&mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil).(*calendarService)
	reason := "lluvia"
	sessionID := int64(3)
	otherName := "gym"

	assert.ErrorIs(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "mystery"}, "", false), ErrCalendarInvalidKind)
	assert.ErrorIs(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "other"}, "", false), ErrCalendarFieldMismatch)
	assert.ErrorIs(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "training"}, "", false), ErrCalendarFieldMismatch)
	assert.ErrorIs(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "cancelled"}, "", false), ErrCalendarFieldMismatch)
	assert.ErrorIs(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason}, "other", false), ErrCalendarInvalidCancelTransition)

	require.NoError(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "other", OtherName: &otherName}, "", false))
	require.NoError(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "training", SessionID: &sessionID}, "", false))
	require.NoError(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "training"}, "", true))
	require.NoError(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason}, "training", false))

	isPresencial := true
	assert.ErrorIs(t, svc.validateDayFields(calendar.CalendarDayRequest{Kind: "rest", IsPresencial: &isPresencial}, "", false), ErrCalendarFieldMismatch)

	timeFrom := "19:00"
	timeTo := "not-a-time"
	badTo := calendar.CalendarDayRequest{Kind: "rest", IsPresencial: &isPresencial, PresencialTimeFrom: &timeFrom, PresencialTimeTo: &timeTo, PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4}}
	assert.ErrorIs(t, svc.validateDayFields(badTo, "", false), ErrCalendarInvalidTimeFormat)
}

func TestCalendarService_Task5_IsCalendarDayClosed_Variants(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	assert.True(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today.AddDate(0, 0, -1)}, now), "el pasado siempre está cerrado")
	assert.False(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today.AddDate(0, 0, 1)}, now), "el futuro nunca está cerrado")
	assert.False(t, isCalendarDayClosed(dbs.GroupCalendarDay{Date: today, IsPresencial: true}, now), "hoy presencial sin horario aún no está cerrado")
}

func TestCalendarService_Task5_PlanDayLocationMapping(t *testing.T) {
	badJSON := "not-json"
	_, err := calendarRequestFromPlanDay(dbs.PlanDay{Kind: "rest", DefaultPresencial: true, DefaultLocation: &badJSON})
	assert.ErrorIs(t, err, ErrCalendarFieldMismatch)

	req, err := calendarRequestFromPlanDay(dbs.PlanDay{Kind: "rest", DefaultPresencial: true})
	require.NoError(t, err)
	require.NotNil(t, req.IsPresencial)
	assert.True(t, *req.IsPresencial)
	assert.Nil(t, req.PresencialLocation)
}

func TestCalendarService_Task5_JsonUnmarshalLocation_Error(t *testing.T) {
	loc, err := jsonUnmarshalLocation("not-json")
	assert.Error(t, err)
	assert.Nil(t, loc)
}

func TestCalendarService_Task5_NewCalendarClosedDaysError_Empty(t *testing.T) {
	err := newCalendarClosedDaysError([]string{})
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	assert.Equal(t, ErrCalendarDayClosed.Error(), err.Error())
}

// ---------------------------------------------------------------------------
// Tests con Postgres real: instanciación, reasignación y guards transaccionales
// ---------------------------------------------------------------------------

func TestCalendarService_Task5_UpsertDayTraining_SessionErrorsStayTyped(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "ups5a")
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 3)

	t.Run("sesión inexistente", func(t *testing.T) {
		missing := int64(987654)
		_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &missing})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCalendarSessionNotFound)
	})

	t.Run("ejercicio de la sesión inexistente", func(t *testing.T) {
		other := date.AddDate(0, 0, 1)
		session := &dbs.Session{OwnerID: owner.ID, Name: "sesión huérfana"}
		require.NoError(t, db.Create(session).Error)
		require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: 987654, Role: "main"}).Error)

		_, err := svc.UpsertDay(nil, group.ID, owner.ID, other, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSessionExerciseNotFound)

		var instanceCount int64
		require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
		assert.Zero(t, instanceCount, "el rollback de la transacción no debe dejar instancias de la operación fallida")
	})
}

func TestCalendarService_Task5_CancelClosedTrainingDayKeepsInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "can5b")
	oldSession := &dbs.SessionInstance{Name: "instancia pasada"}
	require.NoError(t, db.Create(oldSession).Error)
	past := time.Now().AddDate(0, 0, -1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	oldID := oldSession.ID
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "training", SessionInstanceID: &oldID}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	reason := "clima"

	resp, err := svc.UpsertDay(nil, group.ID, owner.ID, past, calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason})

	require.NoError(t, err)
	assert.Equal(t, "cancelled", resp.Kind)
	day, findErr := calendarDao.FindByGroupAndDate(nil, group.ID, past)
	require.NoError(t, findErr)
	require.NotNil(t, day)
	require.NotNil(t, day.SessionInstanceID)
	assert.Equal(t, oldSession.ID, *day.SessionInstanceID, "cancelar conserva la instancia existente como contexto")
	kept, instErr := daos.NewSessionInstanceDao(db).FindByID(nil, oldSession.ID)
	require.NoError(t, instErr)
	assert.NotNil(t, kept)
}

func TestCalendarService_Task5_DeleteClosedDayKeepsDayAndInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "del5c")
	sess := &dbs.SessionInstance{Name: "instancia pasada"}
	require.NoError(t, db.Create(sess).Error)
	past := time.Now().AddDate(0, 0, -1)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	id := sess.ID
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "training", SessionInstanceID: &id}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	err := svc.DeleteDay(nil, group.ID, owner.ID, past)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	day, findErr := calendarDao.FindByGroupAndDate(nil, group.ID, past)
	require.NoError(t, findErr)
	require.NotNil(t, day, "el día cerrado no debe borrarse")
	kept, instErr := daos.NewSessionInstanceDao(db).FindByID(nil, sess.ID)
	require.NoError(t, instErr)
	assert.NotNil(t, kept, "la instancia del día cerrado tampoco debe borrarse")
}

func TestCalendarService_Task5_DeleteFutureDayDeletesInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "del5f")
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "del exercise", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "del session"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	date := time.Now().AddDate(0, 0, 2)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &catalogSession.ID})
	require.NoError(t, err)

	err = svc.DeleteDay(nil, group.ID, owner.ID, date)
	require.NoError(t, err)

	deleted, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Nil(t, deleted, "el día debe borrarse")
	var instanceCount, exerciseCount, linkCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	require.NoError(t, db.Model(&dbs.ExerciseInstance{}).Count(&exerciseCount).Error)
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkCount).Error)
	assert.Zero(t, instanceCount, "la instancia sin feedback se borra físicamente junto con el día")
	assert.Zero(t, exerciseCount)
	assert.Zero(t, linkCount)
}

func TestCalendarService_Task5_ReassignmentKeepsInstanceWithExerciseOnlyFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "fee5ex")
	oldExercise := &dbs.ExerciseInstance{Name: "old ejercicio", Kind: "running"}
	require.NoError(t, db.Create(oldExercise).Error)
	oldSession := &dbs.SessionInstance{Name: "old sesión"}
	require.NoError(t, db.Create(oldSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{SessionInstanceID: oldSession.ID, ExerciseInstanceID: oldExercise.ID, Role: "main"}).Error)
	date := time.Now().AddDate(0, 0, 2)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	oldID := oldSession.ID
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &oldID}))
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/task5.jpg"}))
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		AssignedSessionID:   999999, // ningún feedback apunta a la sesión: solo al ejercicio
		AssignedExerciseID:  oldExercise.ID,
		AthleteUserID:       owner.ID,
		FeedbackOwnerUserID: owner.ID,
		ReportSource:        "atleta",
		SessionDate:         date,
		MediaURLs:           media,
	}).Error)
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "new exercise", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "new session"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &catalogSession.ID})
	require.NoError(t, err)

	keptSession, sessErr := daos.NewSessionInstanceDao(db).FindByID(nil, oldSession.ID)
	require.NoError(t, sessErr)
	assert.NotNil(t, keptSession, "con feedback sobre el ejercicio, la sesión vieja se conserva huérfana")
	keptExercise, exErr := daos.NewExerciseInstanceDao(db).FindByID(nil, oldExercise.ID)
	require.NoError(t, exErr)
	assert.NotNil(t, keptExercise, "el ejercicio con feedback no debe borrarse")

	day, dayErr := calendarDao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, dayErr)
	require.NotNil(t, day.SessionInstanceID)
	assert.NotEqual(t, oldSession.ID, *day.SessionInstanceID, "el día sí se repuntea a la instancia nueva")
}

func TestCalendarService_Task5_BulkTrainingCreatesOneInstancePerDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "bult5d")
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "trote bulk", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "sesión bulk"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	d1 := time.Now().AddDate(0, 0, 4)
	d2 := d1.AddDate(0, 0, 1)
	d3 := d1.AddDate(0, 0, 2)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{
		Dates: []string{d1.Format("2006-01-02"), d2.Format("2006-01-02"), d3.Format("2006-01-02")},
		Kind:  "training", SessionID: &catalogSession.ID,
	})
	require.NoError(t, err)
	require.Len(t, resp.Days, 3)

	var instances []dbs.SessionInstance
	require.NoError(t, db.Where("name = ?", catalogSession.Name).Find(&instances).Error)
	require.Len(t, instances, 3, "una instancia independiente por fecha, sin deduplicar (D4)")

	instantiatedIDs := map[int64]bool{}
	for _, dayResp := range resp.Days {
		require.NotNil(t, dayResp.SessionInstance)
		instantiatedIDs[dayResp.SessionInstance.ID] = true
		assert.Equal(t, catalogSession.Name, dayResp.SessionInstance.Name)
		require.Len(t, dayResp.SessionInstance.Exercises, 1)
		assert.Equal(t, "trote bulk", dayResp.SessionInstance.Exercises[0].Name)
	}
	assert.Len(t, instantiatedIDs, 3)

	var linkCount int64
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkCount).Error)
	assert.Equal(t, int64(3), linkCount)
}

func TestCalendarService_Task5_BulkClosedDatesReferencedInError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "bult5c")
	past := time.Now().AddDate(0, 0, -1)
	future := time.Now().AddDate(0, 0, 5)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "rest"}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{
		Dates: []string{future.Format("2006-01-02"), past.Format("2006-01-02")},
		Kind:  "rest",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	assert.Contains(t, err.Error(), past.Format("2006-01-02"), "el error del lote lista la fecha cerrada")

	day, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, future)
	require.NoError(t, findErr)
	assert.Nil(t, day, "ninguna fecha del lote debe escribirse si alguna está cerrada")
}

func TestCalendarService_Task5_BulkClearDeletesSupersededInstances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "bc5")
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "clear exercise", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "clear session"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	date := time.Now().AddDate(0, 0, 3)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &catalogSession.ID})
	require.NoError(t, err)

	err = svc.BulkClear(nil, group.ID, owner.ID, calendar.BulkClearRequest{Dates: []string{date.Format("2006-01-02")}})
	require.NoError(t, err)

	day, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Nil(t, day)
	var instanceCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	assert.Zero(t, instanceCount, "bulk-clear también borra las instancias superadas sin feedback")
}

func TestCalendarService_Task5_ShiftClosedWithRealDBListsClosedDates(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "shif5")
	past := time.Now().AddDate(0, 0, -1)
	future := time.Now().AddDate(0, 0, 6)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "rest"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "rest"}))
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: past.Format("2006-01-02"), Days: 2})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	assert.Contains(t, err.Error(), past.Format("2006-01-02"))

	moved, movedErr := calendarDao.FindByGroupAndDate(nil, group.ID, future.AddDate(0, 0, 2))
	require.NoError(t, movedErr)
	assert.Nil(t, moved, "si alguna fila afectada está cerrada no se mueve ni una")
	kept, keptErr := calendarDao.FindByGroupAndDate(nil, group.ID, future)
	require.NoError(t, keptErr)
	assert.NotNil(t, kept)
}

func TestCalendarService_Task5_StampCreatesIndependentInstancesForEveryTrainingDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "stmp5")
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "stamp exercise", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "stamp session"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main", RepeatCount: 2, RestMinutes: 1}).Error)
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan multi"}
	require.NoError(t, db.Create(plan).Error)
	sessionID := session.ID
	require.NoError(t, db.Create([]dbs.PlanDay{
		{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &sessionID},
		{PlanID: plan.ID, SequenceNo: 2, Kind: "training", SessionID: &sessionID},
		{PlanID: plan.ID, SequenceNo: 3, Kind: "rest"},
	}).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, db)
	start := time.Now().AddDate(0, 0, 7)

	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: start.Format("2006-01-02")})
	require.NoError(t, err)
	require.Len(t, resp.Days, 3)

	var instances []dbs.SessionInstance
	require.NoError(t, db.Where("name = ?", session.Name).Find(&instances).Error)
	require.Len(t, instances, 2, "una instancia por cada día training, sin compartir (D4)")
	instanceIDs := map[int64]bool{}
	for _, inst := range instances {
		instanceIDs[inst.ID] = true
	}
	assert.Len(t, instanceIDs, 2)

	trainingResponses := 0
	respondedInstanceIDs := map[int64]bool{}
	for _, dayResp := range resp.Days {
		if dayResp.Kind == "training" {
			trainingResponses++
			require.NotNil(t, dayResp.SessionInstance)
			respondedInstanceIDs[dayResp.SessionInstance.ID] = true
			assert.Equal(t, session.Name, dayResp.SessionInstance.Name)
			require.Len(t, dayResp.SessionInstance.Exercises, 1)
			assert.Equal(t, 2, dayResp.SessionInstance.Exercises[0].RepeatCount)
			require.NotNil(t, dayResp.SourcePlanID)
			assert.Equal(t, plan.ID, *dayResp.SourcePlanID)
		} else {
			assert.Nil(t, dayResp.SessionInstance, "los días rest/other no tienen instancia")
		}
	}
	assert.Equal(t, 2, trainingResponses)
	assert.Equal(t, instanceIDs, respondedInstanceIDs)

	var linkCount int64
	require.NoError(t, db.Model(&dbs.SessionExerciseInstance{}).Count(&linkCount).Error)
	assert.Equal(t, int64(2), linkCount)
}

func TestCalendarService_Task5_StampConflictWithoutForceRealDBKeepsOriginal(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "stmp5k")
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "plan ex", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "plan session"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main"}).Error)
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "plan conflicto"}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, db.Create(&dbs.PlanDay{PlanID: plan.ID, SequenceNo: 1, Kind: "training", SessionID: &session.ID}).Error)
	occupiedDate := time.Now().AddDate(0, 0, 8)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: occupiedDate, Kind: "rest"}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, daos.NewTrainingPlanDao(db), daos.NewPlanDayDao(db), nil, db)

	_, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: occupiedDate.Format("2006-01-02")})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarStampConflict)
	assert.Contains(t, err.Error(), occupiedDate.Format("2006-01-02"))

	var instanceCount int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).Count(&instanceCount).Error)
	assert.Zero(t, instanceCount, "sin force no debe crearse ninguna instancia")

	day, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, occupiedDate)
	require.NoError(t, findErr)
	require.NotNil(t, day)
	assert.Equal(t, "rest", day.Kind, "el día existente no debe modificarse")
}

func TestCalendarService_Task5_NextSessionEmbedsFrozenInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "next5")
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "next exercise", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "próxima sesión"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main"}).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 2)
	sessionID := catalogSession.ID
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &sessionID})
	require.NoError(t, err)

	resp, err := svc.NextSession(nil, owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.SessionInstance)
	assert.Equal(t, catalogSession.Name, resp.SessionInstance.Name)
	require.Len(t, resp.SessionInstance.Exercises, 1)
	assert.Equal(t, "next exercise", resp.SessionInstance.Exercises[0].Name)
}

func TestCalendarService_Task5_NextSession_MissingInstanceSurfaces(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "next5b")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	date := time.Now().AddDate(0, 0, 2)
	missing := int64(987654321)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", SessionInstanceID: &missing}))
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	_, err := svc.NextSession(nil, owner.ID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encontrada")
}

func TestCalendarService_Task5_DeleteSupersededInstance_ZeroIDIsNoop(t *testing.T) {
	db := testutils.SetupTestDB(t)
	task3OwnerGroup(t, db, "noop5") // garantiza owner+group autogenerados aunque esta prueba no los use
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	kept := &dbs.SessionInstance{Name: "no debe borrarse"}
	require.NoError(t, db.Create(kept).Error)

	noopErr := svc.(*calendarService).deleteSupersededInstance(nil, db, 0)
	require.NoError(t, noopErr)

	found, findErr := daos.NewSessionInstanceDao(db).FindByID(nil, kept.ID)
	require.NoError(t, findErr)
	assert.NotNil(t, found)
}

// El test siguiente simula escritura concurrente: la fila destino existía
// físicamente en la tabla al momento de correr el shift, pero queda fuera de
// las SELECTs del service (condición global en el handle de gorm usado como
// s.db) — la ventana exacta de un INSERT concurrente entre el fetch y los
// UPDATEs: el shift choca contra el unique index al escribir el destino.
func calStrPtr(s string) *string { return &s }

func TestCalendarService_Task5_ShiftDestinationOccupiedReturnsCollisionAndRollsBack(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "shif5x")
	calendarDao := daos.NewGroupCalendarDayDao(db)
	from := time.Now().AddDate(0, 0, 5)
	destination := from.AddDate(0, 0, 2)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: from, Kind: "rest"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: destination, Kind: "other", OtherName: calStrPtr("ocupado")}))
	svcDB := db.Where("date <> ?", destination)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, svcDB)

	_, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: from.Format("2006-01-02"), Days: 2})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarShiftCollision, "la colisión debe mapear al sentinel, no a un 500")

	keptFrom, ferr := calendarDao.FindByGroupAndDate(nil, group.ID, from)
	require.NoError(t, ferr)
	assert.NotNil(t, keptFrom, "el rollback no debe mover ninguna fila")
	keptDest, derr := calendarDao.FindByGroupAndDate(nil, group.ID, destination)
	require.NoError(t, derr)
	require.NotNil(t, keptDest)
	assert.Equal(t, "ocupado", *keptDest.OtherName, "la fila destino no debe ser tocada")
}

// Escenario explícito de la spec (group-calendar §"independencia del catálogo"):
// editar la Session/Exercise de catálogo después de asignar no muta la instancia.
func TestCalendarService_Task5_GetRangeFrozenAfterCatalogEdits(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "frozen5")
	catalogExercise := &dbs.Exercise{OwnerID: owner.ID, Name: "trote original", Kind: "running"}
	require.NoError(t, db.Create(catalogExercise).Error)
	catalogSession := &dbs.Session{OwnerID: owner.ID, Name: "sesión original"}
	require.NoError(t, db.Create(catalogSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: catalogSession.ID, ExerciseID: catalogExercise.ID, Role: "main", RepeatCount: 3}).Error)
	date := time.Now().AddDate(0, 0, 3)
	sessionID := catalogSession.ID
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "training", SessionID: &sessionID})
	require.NoError(t, err)

	// Edits de catálogo vía DAO directo, después de la asignación
	frozenSessionName := catalogSession.Name
	frozenExerciseName := catalogExercise.Name
	editedSession := &dbs.Session{ID: catalogSession.ID, OwnerID: owner.ID, Name: "sesión editada"}
	require.NoError(t, daos.NewSessionDao(db).Update(nil, editedSession))
	editedExercise := &dbs.Exercise{ID: catalogExercise.ID, OwnerID: owner.ID, Name: "trote editado", Kind: "running"}
	require.NoError(t, daos.NewExerciseDao(db).Update(nil, editedExercise))

	resp, err := svc.GetRange(nil, group.ID, owner.ID, date, date)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	require.NotNil(t, resp[0].SessionInstance)
	assert.Equal(t, frozenSessionName, resp[0].SessionInstance.Name, "la instancia conserva el nombre previo al edit del catálogo")
	require.Len(t, resp[0].SessionInstance.Exercises, 1)
	assert.Equal(t, frozenExerciseName, resp[0].SessionInstance.Exercises[0].Name, "el ejercicio instanciado conserva el nombre previo al edit")
	assert.Equal(t, 3, resp[0].SessionInstance.Exercises[0].RepeatCount, "los params instanciados tampoco siguen al catálogo")
}
