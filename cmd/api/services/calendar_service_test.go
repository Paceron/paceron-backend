package services

import (
	"fmt"
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

type mockGroupCalendarDao struct {
	upsertFn                   func(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	findByGroupAndDateFn       func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	findByGroupAndRangeFn      func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	findPresencialForGroupsFn  func(ctx *gin.Context, groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error)
	deleteFn                   func(ctx *gin.Context, groupID int64, date time.Time) error
	deleteByDatesFn            func(ctx *gin.Context, groupID int64, dates []time.Time) error
	findNextSessionForGroupsFn func(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error)
	clearSourcePlanFn          func(ctx *gin.Context, planID int64) error
	updateDatesForShiftFn      func(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
}

func (m *mockGroupCalendarDao) Upsert(ctx *gin.Context, day *dbs.GroupCalendarDay) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, day)
	}
	day.ID = 1
	return nil
}
func (m *mockGroupCalendarDao) FindByGroupAndDate(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
	if m.findByGroupAndDateFn != nil {
		return m.findByGroupAndDateFn(ctx, groupID, date)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) FindByGroupAndRange(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
	if m.findByGroupAndRangeFn != nil {
		return m.findByGroupAndRangeFn(ctx, groupID, from, to)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) FindPresencialForGroupsInRange(ctx *gin.Context, groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error) {
	if m.findPresencialForGroupsFn != nil {
		return m.findPresencialForGroupsFn(ctx, groupIDs, dates)
	}
	return nil, nil
}

func (m *mockGroupCalendarDao) Delete(ctx *gin.Context, groupID int64, date time.Time) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, groupID, date)
	}
	return nil
}
func (m *mockGroupCalendarDao) DeleteByDates(ctx *gin.Context, groupID int64, dates []time.Time) error {
	if m.deleteByDatesFn != nil {
		return m.deleteByDatesFn(ctx, groupID, dates)
	}
	return nil
}
func (m *mockGroupCalendarDao) FindNextSessionForGroups(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error) {
	if m.findNextSessionForGroupsFn != nil {
		return m.findNextSessionForGroupsFn(ctx, groupIDs, fromDate)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) ClearSourcePlan(ctx *gin.Context, planID int64) error {
	if m.clearSourcePlanFn != nil {
		return m.clearSourcePlanFn(ctx, planID)
	}
	return nil
}
func (m *mockGroupCalendarDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	if m.updateDatesForShiftFn != nil {
		return m.updateDatesForShiftFn(ctx, groupID, oldDate, newDate)
	}
	return nil
}

// mockGroupDao and mockGroupUserDao are already declared in
// group_service_test.go / group_user_service_test.go (same package,
// functionally identical shape) — reused here, not redeclared.

func TestCalendarService_GetRange_OwnerAllowed(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	calDao := &mockGroupCalendarDao{}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.GetRange(nil, 1, 7, time.Now(), time.Now())

	require.NoError(t, err)
}

func TestCalendarService_GetRange_ForbiddenForOutsider(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	groupUserDao := &mockGroupUserDao{findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) { return nil, nil }}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)

	_, err := svc.GetRange(nil, 1, 99, time.Now(), time.Now())

	assert.ErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_UpsertDay_NonOwnerForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 99, time.Now(), calendar.CalendarDayRequest{Kind: "rest"})

	assert.ErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_UpsertDay_OtherRequiresOtherName(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "other"})

	assert.ErrorIs(t, err, ErrCalendarFieldMismatch)
}

func TestCalendarService_UpsertDay_CancelFromRestRejected(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "rest"}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	reason := "lluvia"

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason})

	assert.ErrorIs(t, err, ErrCalendarInvalidCancelTransition)
}

func TestCalendarService_UpsertDay_CancelFromTrainingAccepted(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	sessionID := int64(3)
	calDao := &mockGroupCalendarDao{findByGroupAndDateFn: func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", SessionInstanceID: &sessionID}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	reason := "lluvia"

	resp, err := svc.UpsertDay(nil, 1, 7, time.Now(), calendar.CalendarDayRequest{Kind: "cancelled", CancelledReason: &reason})

	require.NoError(t, err)
	assert.Equal(t, "cancelled", resp.Kind)
}

func TestCalendarService_DeleteDay_NonOwnerForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 99, time.Now())

	assert.ErrorIs(t, err, ErrCalendarForbidden)
}

// TestCalendarService_Stamp_Success prueba contra Postgres real: el loop de
// escritura de Stamp corre dentro de s.db.Transaction (ver Fix 1 del audit de
// calidad final), así que pasar un *gorm.DB nil (como hacían las versiones
// mockeadas de este test) haría panic en gorm.(*DB).Transaction.
func TestCalendarService_Stamp_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupDao := daos.NewGroupDao(db)
	teamDao := daos.NewTeamDao(db)
	groupUserDao := daos.NewGroupUserDao(db)
	trainingPlanDao := daos.NewTrainingPlanDao(db)
	planDayDao := daos.NewPlanDayDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, groupDao, teamDao, groupUserDao, nil, trainingPlanDao, planDayDao, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-stamp-owner@test.com", DNI: "50000100", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo stamp", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo stamp", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "Ejercicio stamp", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	session := &dbs.Session{OwnerID: owner.ID, Name: "Sesión stamp"}
	require.NoError(t, db.Create(session).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: session.ID, ExerciseID: exercise.ID, Role: "main"}).Error)

	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan stamp"}
	require.NoError(t, db.Create(plan).Error)
	sessionID := session.ID
	planDays := []dbs.PlanDay{
		{PlanID: plan.ID, SequenceNo: 1, Kind: "rest"},
		{PlanID: plan.ID, SequenceNo: 2, Kind: "training", SessionID: &sessionID},
	}
	require.NoError(t, db.Create(&planDays).Error)

	resp, err := svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: "2026-10-01"})

	require.NoError(t, err)
	assert.Len(t, resp.Days, 2)

	stampedDays, err := calendarDao.FindByGroupAndRange(nil, group.ID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Len(t, stampedDays, 2, "las 2 filas del plan deben haber quedado commiteadas por la transacción")
}

func TestCalendarService_Stamp_PlanFromOtherOwnerForbidden(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 99}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, &mockPlanDayDao{}, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})

	assert.ErrorIs(t, err, ErrCalendarPlanForbidden)
}

// TestCalendarService_Bulk_Success prueba contra Postgres real: el loop de
// escritura de Bulk corre dentro de s.db.Transaction (ver Fix 1 del audit de
// calidad final), así que pasar un *gorm.DB nil haría panic en
// gorm.(*DB).Transaction.
func TestCalendarService_Bulk_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupDao := daos.NewGroupDao(db)
	teamDao := daos.NewTeamDao(db)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-bulk-owner@test.com", DNI: "50000101", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo bulk", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo bulk", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	resp, err := svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{"2026-10-01", "2026-10-02"}, Kind: "rest"})

	require.NoError(t, err)
	assert.Len(t, resp.Days, 2)

	rows, err := calendarDao.FindByGroupAndRange(nil, group.ID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Len(t, rows, 2, "las 2 fechas del bulk deben haber quedado commiteadas por la transacción")
}

func TestCalendarService_BulkClear_Success(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	deleted := false
	calDao := &mockGroupCalendarDao{deleteByDatesFn: func(ctx *gin.Context, groupID int64, dates []time.Time) error {
		deleted = true
		return nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.BulkClear(nil, 1, 7, calendar.BulkClearRequest{Dates: []string{"2026-10-01"}})

	require.NoError(t, err)
	assert.True(t, deleted)
}

// TestCalendarService_Shift_NoCollision prueba contra Postgres real: el loop
// de escritura de Shift corre dentro de s.db.Transaction (ver Fix 1 del audit
// de calidad final), así que pasar un *gorm.DB nil haría panic en
// gorm.(*DB).Transaction.
func TestCalendarService_Shift_NoCollision(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupDao := daos.NewGroupDao(db)
	teamDao := daos.NewTeamDao(db)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-shift-owner@test.com", DNI: "50000102", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo shift", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo shift", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	fromDate, _ := time.Parse("2006-01-02", "2026-10-01")
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: fromDate, Kind: "rest"}))

	resp, err := svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: "2026-10-01", Days: 2})

	require.NoError(t, err)
	assert.Len(t, resp.Days, 1)

	shiftedDate := fromDate.AddDate(0, 0, 2)
	shifted, err := calendarDao.FindByGroupAndDate(nil, group.ID, shiftedDate)
	require.NoError(t, err)
	require.NotNil(t, shifted, "la fila debe haber quedado commiteada en la nueva fecha por la transacción")

	original, err := calendarDao.FindByGroupAndDate(nil, group.ID, fromDate)
	require.NoError(t, err)
	assert.Nil(t, original, "la fila original debe haberse movido, no duplicado")
}

func TestCalendarService_Stamp_ConflictWithoutForce(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{{SequenceNo: 1, Kind: "rest"}}, nil
	}}
	occupiedDate, _ := time.Parse("2006-01-02", "2026-10-01")
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return []dbs.GroupCalendarDay{{GroupID: groupID, Date: occupiedDate, Kind: "rest"}}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	_, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})

	assert.ErrorIs(t, err, ErrCalendarStampConflict)
}

func TestCalendarService_NextSession_Found(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	nextDate, _ := time.Parse("2006-01-02", "2026-10-10")
	var capturedFromDate time.Time
	calDao := &mockGroupCalendarDao{findNextSessionForGroupsFn: func(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error) {
		capturedFromDate = fromDate
		return &dbs.GroupCalendarDay{GroupID: 1, Date: nextDate, Kind: "training"}, nil
	}}
	svc := NewCalendarService(calDao, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.GroupID)

	// Regression para el bug de truncado: time.Now().Truncate(24*time.Hour)
	// trunca a medianoche UTC/epoch, no a medianoche local — en un huso
	// horario negativo (ej. America/Argentina/Cordoba, UTC-3) eso corre la
	// comparación un día. La fecha pasada a la DAO debe ser exactamente
	// medianoche en la zona horaria local (mismo patrón que isCalendarDayClosed
	// en calendar_service.go), no un truncado UTC.
	now := time.Now()
	expectedToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	assert.True(t, capturedFromDate.Equal(expectedToday), "fromDate debe ser medianoche local, no un truncado UTC/epoch")
	assert.Equal(t, now.Location(), capturedFromDate.Location(), "fromDate debe conservar la zona horaria local")
}

// TestCalendarService_NextSession_FindsTodaysSession prueba contra Postgres
// real que una sesión programada para HOY se encuentra — regresión directa
// del bug de time.Now().Truncate(24*time.Hour), que trunca a medianoche
// UTC/epoch en vez de medianoche local (America/Argentina/Cordoba, UTC-3 en
// este repo) y podía excluir la sesión de hoy dependiendo de la hora del día
// en que corriera la request.
func TestCalendarService_NextSession_FindsTodaysSession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-nextsession-today@test.com", DNI: "50000103", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo next-session hoy", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo next-session hoy", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	groupUser := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}
	require.NoError(t, db.Create(groupUser).Error)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training"}))

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp, "la sesión de hoy debe encontrarse, no excluirse por el truncado a UTC")
	assert.Equal(t, group.ID, resp.GroupID)
	assert.Equal(t, today.Format("2006-01-02"), resp.Date)
}

func TestCalendarService_NextSession_NoneReturnsNil(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	assert.Nil(t, resp)
}

func TestCalendarService_CalendarSummary_ListsGroups(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}, {GroupID: 2, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, Name: fmt.Sprintf("Grupo %d", id)}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.CalendarSummary(nil, 42)

	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestCalendarService_DeleteDay_OwnerSuccess(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	deleted := false
	calDao := &mockGroupCalendarDao{deleteFn: func(ctx *gin.Context, groupID int64, date time.Time) error {
		deleted = true
		return nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 7, time.Now())

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestCalendarService_DeleteDay_DaoErrorWrapped(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	calDao := &mockGroupCalendarDao{deleteFn: func(ctx *gin.Context, groupID int64, date time.Time) error {
		return fmt.Errorf("boom")
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	err := svc.DeleteDay(nil, 1, 7, time.Now())

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCalendarForbidden)
}

func TestCalendarService_UpsertDay_PresencialSuccessRoundTrip(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	isPresencial := true
	timeFrom := "18:30"
	timeTo := "19:30"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial, PresencialTimeFrom: &timeFrom, PresencialTimeTo: &timeTo,
		PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	}

	resp, err := svc.UpsertDay(nil, 1, 7, time.Now().AddDate(0, 0, 1), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.IsPresencial)
	require.NotNil(t, resp.PresencialTimeFrom)
	assert.Equal(t, "18:30", *resp.PresencialTimeFrom)
	require.NotNil(t, resp.PresencialTimeTo)
	assert.Equal(t, "19:30", *resp.PresencialTimeTo)
	require.NotNil(t, resp.PresencialLocation)
	assert.Equal(t, -34.6, resp.PresencialLocation.Lat)
}

func TestCalendarService_UpsertDay_PresencialInvalidTimeFormat(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	isPresencial := true
	timeFrom := "not-a-time"
	timeTo := "19:30"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial, PresencialTimeFrom: &timeFrom, PresencialTimeTo: &timeTo,
		PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	}

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), req)

	require.Error(t, err)
}

func TestCalendarService_UpsertDay_PresencialTimeToBeforeFrom(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)
	isPresencial := true
	timeFrom := "19:30"
	timeTo := "18:30"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial, PresencialTimeFrom: &timeFrom, PresencialTimeTo: &timeTo,
		PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	}

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), req)

	assert.ErrorIs(t, err, ErrCalendarInvalidTimeRange)
}
