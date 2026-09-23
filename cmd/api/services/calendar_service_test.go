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
	upsertFn                  func(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	findByGroupAndDateFn      func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	findByGroupAndRangeFn     func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	findForGroupsInRangeFn    func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	findPresencialForGroupsFn func(ctx *gin.Context, groupIDs []int64, dates []time.Time) ([]dbs.GroupCalendarDay, error)
	deleteFn                  func(ctx *gin.Context, groupID int64, date time.Time) error
	deleteByDatesFn           func(ctx *gin.Context, groupID int64, dates []time.Time) error
	findNextForGroupsByKindFn func(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error)
	findNextPresencialFn      func(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error)
	clearSourcePlanFn         func(ctx *gin.Context, planID int64) error
	updateDatesForShiftFn     func(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
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
func (m *mockGroupCalendarDao) FindForGroupsInRange(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
	if m.findForGroupsInRangeFn != nil {
		return m.findForGroupsInRangeFn(ctx, groupIDs, from, to)
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
func (m *mockGroupCalendarDao) FindNextForGroupsByKind(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
	if m.findNextForGroupsByKindFn != nil {
		return m.findNextForGroupsByKindFn(ctx, groupIDs, kind, today, nowHHMM)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) FindNextPresencialForGroups(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
	if m.findNextPresencialFn != nil {
		return m.findNextPresencialFn(ctx, groupIDs, today, nowHHMM)
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

func TestCalendarService_NextSession_BothBanners(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "Grupo 1"}}, nil
	}}
	nextDate, _ := time.Parse("2006-01-02", "2026-10-10")
	var capturedKinds []string
	var capturedToday time.Time
	calDao := &mockGroupCalendarDao{findNextForGroupsByKindFn: func(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		capturedKinds = append(capturedKinds, kind)
		capturedToday = today
		if kind == "cancelled" {
			return &dbs.GroupCalendarDay{GroupID: 1, Date: nextDate, Kind: "cancelled"}, nil
		}
		return &dbs.GroupCalendarDay{GroupID: 1, Date: nextDate, Kind: "training"}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp.NextCancelled)
	require.NotNil(t, resp.NextTraining)
	assert.Equal(t, "Grupo 1", resp.NextCancelled.GroupName)
	assert.Equal(t, "Grupo 1", resp.NextTraining.GroupName)
	assert.ElementsMatch(t, []string{"cancelled", "training"}, capturedKinds)

	// Regression para el bug de truncado: time.Now().Truncate(24*time.Hour)
	// trunca a medianoche UTC/epoch, no a medianoche local — en un huso
	// horario negativo (ej. America/Argentina/Cordoba, UTC-3) eso corre la
	// comparación un día. La fecha pasada a la DAO debe ser exactamente
	// medianoche en la zona horaria local (mismo patrón que isCalendarDayClosed
	// en calendar_service.go), no un truncado UTC.
	now := time.Now()
	expectedToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	assert.True(t, capturedToday.Equal(expectedToday), "today debe ser medianoche local, no un truncado UTC/epoch")
	assert.Equal(t, now.Location(), capturedToday.Location(), "today debe conservar la zona horaria local")
}

func TestCalendarService_NextSession_OnlyTraining(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "Grupo 1"}}, nil
	}}
	nextDate, _ := time.Parse("2006-01-02", "2026-10-10")
	calDao := &mockGroupCalendarDao{findNextForGroupsByKindFn: func(ctx *gin.Context, groupIDs []int64, kind string, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		if kind == "training" {
			return &dbs.GroupCalendarDay{GroupID: 1, Date: nextDate, Kind: "training", SessionInstanceID: nil}, nil
		}
		return nil, nil
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	assert.Nil(t, resp.NextCancelled)
	require.NotNil(t, resp.NextTraining)
}

func TestCalendarService_NextSession_NoneReturnsBothNull(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp, "siempre responde — nunca nil/204")
	assert.Nil(t, resp.NextCancelled)
	assert.Nil(t, resp.NextTraining)
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
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

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
	require.NotNil(t, resp.NextTraining)
	assert.Equal(t, group.ID, resp.NextTraining.GroupID)
	assert.Equal(t, today.Format("2006-01-02"), resp.NextTraining.Date)
	assert.Equal(t, group.Name, resp.NextTraining.GroupName)
}

// La membresía inactiva (date_end ya pasado) no participa del banner.
func TestCalendarService_NextSession_InactiveMembershipExcluded(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-nextsession-expired@test.com", DNI: "50000104", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo next-session expirada", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo next-session expirada", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	expiredEnd := time.Now().AddDate(0, 0, -1)
	groupUser := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now().AddDate(0, 0, -10), DateEnd: &expiredEnd}
	require.NoError(t, db.Create(groupUser).Error)

	future := time.Now().AddDate(0, 0, 5)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training"}))

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.NextTraining, "la membresía con date_end vencido no debe aportar banner")
	assert.Nil(t, resp.NextCancelled)
}

// La membresía SOFT-DELETED (deleted_at seteado) no participa del banner —
// complementa el caso date_end vencido: FindByUserID filtra ambas vías de
// inactividad, este test cubre la de deleted_at end-to-end contra Postgres real.
func TestCalendarService_NextSession_SoftDeletedMembershipExcluded(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-nextsession-softdeleted@test.com", DNI: "50000106", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo next-session soft-deleted", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo next-session soft-deleted", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	groupUser := &dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now().AddDate(0, 0, -10)}
	require.NoError(t, db.Create(groupUser).Error)
	require.NoError(t, db.Delete(groupUser).Error)

	future := time.Now().AddDate(0, 0, 5)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training"}))

	resp, err := svc.NextSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.NextTraining, "la membresía soft-deleted no debe aportar banner")
	assert.Nil(t, resp.NextCancelled)
}

// "Hoy cuenta" end-to-end contra Postgres real (mismo criterio que
// isCalendarDayClosed): hoy presencial ya arrancado NO aparece; hoy presencial
// por arrancar y hoy asincrónico SÍ.
func TestCalendarService_NextSession_TodayCounts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-nextsession-todaycounts@test.com", DNI: "50000105", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo next-session hoy-counts", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo next-session hoy-counts", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)

	now := time.Now()
	// El presencial "por arrancar" es fijo 23:59 UTC; si ya es el último
	// minuto del día no hay horario por arrancar posible.
	if now.Hour() == 23 && now.Minute() == 59 {
		t.Skip("último minuto del día local")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Caso 1: hoy presencial ya arrancado → no cuenta.
	startedFrom := utcTimeHHMM("00:00")
	startedTo := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: startedFrom, PresencialTimeTo: &startedTo}))
	resp, err := svc.NextSession(nil, owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.NextTraining, "hoy presencial ya arrancado no cuenta")

	// Caso 2: hoy presencial por arrancar → cuenta.
	pendingFrom := utcTimeHHMM("23:59")
	pendingTo := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: pendingFrom, PresencialTimeTo: &pendingTo}))
	resp, err = svc.NextSession(nil, owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.NextTraining, "hoy presencial por arrancar cuenta")
	require.NotNil(t, resp.NextTraining.PresencialTimeFrom)
	assert.Equal(t, pendingFrom.UTC().Format("15:04"), *resp.NextTraining.PresencialTimeFrom)
	assert.True(t, resp.NextTraining.IsPresencial)

	// Caso 3: hoy asincrónico → cuenta.
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training"}))
	resp, err = svc.NextSession(nil, owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.NextTraining, "hoy asincrónico cuenta")
	assert.False(t, resp.NextTraining.IsPresencial)
	assert.Nil(t, resp.NextTraining.PresencialTimeFrom)
}

// utcTimeHHMM arma un *time.Time de HOY a la HH:MM dada, en UTC (convención
// de persistencia de los horarios presenciales, ver dbs.GroupCalendarDay).
func utcTimeHHMM(hhmm string) *time.Time {
	hm, err := time.Parse("15:04", hhmm)
	if err != nil {
		return nil
	}
	now := time.Now().UTC()
	t := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), 0, 0, time.UTC)
	return &t
}

// Banner del entrenador (design.md D7): administra grupos de 2 equipos; solo
// el grupo de un equipo tiene presencial próximo → ese día, con team_id/team_name
// de su equipo (batch de teams, no N+1).
func TestCalendarService_NextPresencialSession_NearestAcrossTeams(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-nextpresencial-multiteam@test.com", DNI: "50000107", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	teamA := &dbs.Team{Name: "Equipo A presencial", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamA).Error)
	teamB := &dbs.Team{Name: "Equipo B presencial", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamB).Error)
	groupA := &dbs.Group{Name: "Grupo A presencial", TeamID: teamA.ID, IsMain: true}
	require.NoError(t, db.Create(groupA).Error)
	groupB := &dbs.Group{Name: "Grupo B presencial", TeamID: teamB.ID, IsMain: true}
	require.NoError(t, db.Create(groupB).Error)

	far := time.Now().AddDate(0, 0, 7)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupA.ID, Date: far, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("09:00"), PresencialTimeTo: utcTimeHHMM("10:00")}))
	near := time.Now().AddDate(0, 0, 2)
	location := `{"lat":-31.4,"lng":-64.2,"label":"pista"}`
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupB.ID, Date: near, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeHHMM("08:00"), PresencialTimeTo: utcTimeHHMM("09:00"), PresencialLocation: &location}))

	resp, err := svc.NextPresencialSession(nil, owner.ID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, groupB.ID, resp.GroupID, "elige la más cercana sin importar equipo")
	assert.Equal(t, groupB.Name, resp.GroupName)
	assert.Equal(t, teamB.ID, resp.TeamID)
	assert.Equal(t, teamB.Name, resp.TeamName)
	assert.Equal(t, near.Format("2006-01-02"), resp.Date)
	require.NotNil(t, resp.PresencialTimeFrom)
	assert.Equal(t, "08:00", *resp.PresencialTimeFrom)
	require.NotNil(t, resp.PresencialLocation)
	require.NotNil(t, resp.PresencialLocation.Label)
	assert.Equal(t, "pista", *resp.PresencialLocation.Label)
}

// Solo training+presencial es candidato: async y cancelled quedan fuera
// (mock del DAO — el filtro por kind/presencial vive en la query).
func TestCalendarService_NextPresencialSession_OnlyTrainingPresencial(t *testing.T) {
	var requestedGroupIDs []int64
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "Grupo 1", TeamID: 10}, {ID: 2, Name: "Grupo 2", TeamID: 11}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return []dbs.Team{{ID: 10, Name: "Equipo 10"}, {ID: 11, Name: "Equipo 11"}}, nil
	}}
	calDao := &mockGroupCalendarDao{findNextPresencialFn: func(ctx *gin.Context, groupIDs []int64, today time.Time, nowHHMM string) (*dbs.GroupCalendarDay, error) {
		requestedGroupIDs = groupIDs
		return &dbs.GroupCalendarDay{GroupID: 2, Date: today.AddDate(0, 0, 1), Kind: "training", IsPresencial: true}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.NextPresencialSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []int64{1, 2}, requestedGroupIDs, "busca entre todos los grupos administrados")
	assert.Equal(t, int64(2), resp.GroupID)
	assert.Equal(t, "Grupo 2", resp.GroupName)
	assert.Equal(t, int64(11), resp.TeamID)
	assert.Equal(t, "Equipo 11", resp.TeamName)
}

// Owner sin grupos administrados → nil (controller responde 204).
func TestCalendarService_NextPresencialSession_NoGroupsReturnsNil(t *testing.T) {
	groupDao := &mockGroupDao{findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
		return nil, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.NextPresencialSession(nil, 42)

	require.NoError(t, err)
	assert.Nil(t, resp)
}

// "Hoy cuenta" end-to-end contra Postgres real: hoy presencial ya arrancado NO
// aparece; hoy presencial por arrancar SÍ; async/cancelled nunca son candidatos.
func TestCalendarService_NextPresencialSession_TodayCounts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "calendar-nextpresencial-todaycounts@test.com", DNI: "50000108", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo presencial hoy-counts", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo presencial hoy-counts", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)

	now := time.Now()
	if now.Hour() == 23 && now.Minute() == 59 {
		t.Skip("último minuto del día local")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	future := today.AddDate(0, 0, 3)

	// Caso 1: hoy presencial ya arrancado → no cuenta.
	startedFrom := utcTimeHHMM("00:00")
	startedTo := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: startedFrom, PresencialTimeTo: &startedTo}))
	resp, err := svc.NextPresencialSession(nil, owner.ID)
	require.NoError(t, err)
	assert.Nil(t, resp, "hoy presencial ya arrancado no cuenta")

	// Caso 2: hoy presencial por arrancar → cuenta.
	pendingFrom := utcTimeHHMM("23:59")
	pendingTo := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, time.UTC)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: pendingFrom, PresencialTimeTo: &pendingTo}))
	resp, err = svc.NextPresencialSession(nil, owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp, "hoy presencial por arrancar cuenta")
	assert.Equal(t, today.Format("2006-01-02"), resp.Date)

	// Caso 3: el presencial futuro más cercano pierde contra el de hoy; un
	// async y un cancelled en la misma fecha no desplazan al candidato válido.
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today.AddDate(0, 0, 1), Kind: "cancelled", IsPresencial: true, PresencialTimeFrom: pendingFrom, PresencialTimeTo: &pendingTo}))
	resp, err = svc.NextPresencialSession(nil, owner.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, today.Format("2006-01-02"), resp.Date, "ni async ni cancelled son candidatos")
}

// MemberCalendar con mocks (D8): memberships de 2 grupos → 1 query de días
// con AMBOS group_ids; los nombres salen en batch (1 groups + 1 teams);
// sin días el resultado es un slice vacío no-nil.
func TestCalendarService_MemberCalendar_MergesBothGroups(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}, {GroupID: 2, UserID: userID}}, nil
	}}
	groupDao := &mockGroupDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
		return []dbs.Group{{ID: 1, Name: "Grupo 1", TeamID: 10}, {ID: 2, Name: "Grupo 2", TeamID: 11}}, nil
	}}
	teamDao := &mockTeamDao{findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Team, error) {
		return []dbs.Team{{ID: 10, Name: "Equipo 10"}, {ID: 11, Name: "Equipo 11"}}, nil
	}}
	var capturedGroupIDs []int64
	from, _ := time.Parse("2006-01-02", "2026-10-01")
	to, _ := time.Parse("2006-01-02", "2026-10-31")
	d1, _ := time.Parse("2006-01-02", "2026-10-05")
	d2, _ := time.Parse("2006-01-02", "2026-10-06")
	calDao := &mockGroupCalendarDao{findForGroupsInRangeFn: func(ctx *gin.Context, groupIDs []int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		capturedGroupIDs = groupIDs
		return []dbs.GroupCalendarDay{
			{GroupID: 1, Date: d1, Kind: "training"},
			{GroupID: 2, Date: d2, Kind: "rest"},
		}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.MemberCalendar(nil, 42, from, to)

	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, capturedGroupIDs, "consulta los días con todos los group_ids de las membresías activas")
	require.Len(t, resp, 2)
	assert.Equal(t, "2026-10-05", resp[0].Date)
	assert.Equal(t, int64(1), resp[0].GroupID)
	assert.Equal(t, "Grupo 1", resp[0].GroupName)
	assert.Equal(t, int64(10), resp[0].TeamID)
	assert.Equal(t, "Equipo 10", resp[0].TeamName)
	assert.Equal(t, "2026-10-06", resp[1].Date)
	assert.Equal(t, int64(2), resp[1].GroupID)
	assert.Equal(t, "Grupo 2", resp[1].GroupName)
	assert.Equal(t, int64(11), resp[1].TeamID)
	assert.Equal(t, "Equipo 11", resp[1].TeamName)
}

func TestCalendarService_MemberCalendar_EmptyRangeReturnsEmptySlice(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return []dbs.GroupUser{{GroupID: 1, UserID: userID}}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.MemberCalendar(nil, 42, time.Now(), time.Now().AddDate(0, 0, 7))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp)
}

func TestCalendarService_MemberCalendar_NoMembershipsReturnsEmptySlice(t *testing.T) {
	groupUserDao := &mockGroupUserDao{findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
		return nil, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.MemberCalendar(nil, 42, time.Now(), time.Now().AddDate(0, 0, 7))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp)
}

// MemberCalendar end-to-end contra Postgres real (escenario de la spec
// "Calendario agregado del corredor"): usuario miembro de 2 grupos en 2
// equipos con días en el mismo rango → un solo response con los días de
// ambos, nombres de grupo y equipo resueltos server-side, ordenado por fecha;
// los días fuera del rango y de memberships inactivas quedan fuera.
func TestCalendarService_MemberCalendar_TwoGroupsSameRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "member-calendar-owner@test.com", DNI: "50000109", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	runner := &dbs.User{Name: "Test", Surname: "Runner", Email: "member-calendar-runner@test.com", DNI: "50000110", BirthDate: time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(runner).Error)
	teamA := &dbs.Team{Name: "Equipo A member-cal", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamA).Error)
	teamB := &dbs.Team{Name: "Equipo B member-cal", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamB).Error)
	groupA := &dbs.Group{Name: "Grupo A member-cal", TeamID: teamA.ID, IsMain: true}
	require.NoError(t, db.Create(groupA).Error)
	groupB := &dbs.Group{Name: "Grupo B member-cal", TeamID: teamB.ID, IsMain: true}
	require.NoError(t, db.Create(groupB).Error)
	groupOther := &dbs.Group{Name: "Grupo other member-cal", TeamID: teamB.ID, IsMain: false}
	require.NoError(t, db.Create(groupOther).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: groupA.ID, UserID: runner.ID, DateStart: time.Now()}).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: groupB.ID, UserID: runner.ID, DateStart: time.Now()}).Error)
	// Membresía del tercer grupo, inactiva (date_end vencido): sus días no cuentan.
	expiredEnd := time.Now().AddDate(0, 0, -1)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: groupOther.ID, UserID: runner.ID, DateStart: time.Now().AddDate(0, 0, -10), DateEnd: &expiredEnd}).Error)

	inRange := time.Now().AddDate(0, 0, 3)
	inRange2 := time.Now().AddDate(0, 0, 4)
	outOfRange := time.Now().AddDate(0, 0, 40)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupA.ID, Date: inRange, Kind: "training"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupB.ID, Date: inRange2, Kind: "rest"}))
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupOther.ID, Date: inRange, Kind: "training"}))

	from := time.Now().AddDate(0, 0, 1)
	to := time.Now().AddDate(0, 0, 10)
	resp, err := svc.MemberCalendar(nil, runner.ID, from, to)

	require.NoError(t, err)
	require.Len(t, resp, 2, "solo los días de las 2 membresías activas, en el rango")
	assert.Equal(t, inRange.Format("2006-01-02"), resp[0].Date, "ordenado por fecha")
	assert.Equal(t, groupA.ID, resp[0].GroupID)
	assert.Equal(t, groupA.Name, resp[0].GroupName)
	assert.Equal(t, teamA.ID, resp[0].TeamID)
	assert.Equal(t, teamA.Name, resp[0].TeamName)
	assert.Equal(t, inRange2.Format("2006-01-02"), resp[1].Date)
	assert.Equal(t, groupB.ID, resp[1].GroupID)
	assert.Equal(t, groupB.Name, resp[1].GroupName)
	assert.Equal(t, teamB.ID, resp[1].TeamID)
	assert.Equal(t, teamB.Name, resp[1].TeamName)
	_ = outOfRange
}

// Rango sin días en Postgres real → 200 [] (slice vacío, no null).
func TestCalendarService_MemberCalendar_EmptyRangePostgres(t *testing.T) {
	db := testutils.SetupTestDB(t)
	groupUserDao := daos.NewGroupUserDao(db)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	svc := NewCalendarService(calendarDao, daos.NewGroupDao(db), daos.NewTeamDao(db), groupUserDao, nil, nil, nil, nil, db)

	owner := &dbs.User{Name: "Test", Surname: "Owner", Email: "member-calendar-empty@test.com", DNI: "50000111", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	team := &dbs.Team{Name: "Equipo member-cal empty", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Grupo member-cal empty", TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: owner.ID, DateStart: time.Now()}).Error)
	require.NoError(t, calendarDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Now().AddDate(0, 0, 60), Kind: "training"}))

	resp, err := svc.MemberCalendar(nil, owner.ID, time.Now().AddDate(0, 0, 1), time.Now().AddDate(0, 0, 10))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp)
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
