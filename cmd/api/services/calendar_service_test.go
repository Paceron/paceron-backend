package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type mockGroupCalendarDao struct {
	upsertFn                      func(ctx *gin.Context, day *dbs.GroupCalendarDay) error
	findByGroupAndDateFn          func(ctx *gin.Context, groupID int64, date time.Time) (*dbs.GroupCalendarDay, error)
	findByGroupAndRangeFn         func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error)
	deleteFn                      func(ctx *gin.Context, groupID int64, date time.Time) error
	deleteByDatesFn               func(ctx *gin.Context, groupID int64, dates []time.Time) error
	findNextSessionForGroupsFn    func(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error)
	findDistinctGroupsBySessionFn func(ctx *gin.Context, sessionID int64) ([]int64, error)
	clearSourcePlanFn             func(ctx *gin.Context, planID int64) error
	repointSessionForGroupsFn     func(ctx *gin.Context, groupIDs []int64, oldSessionID, newSessionID int64) error
	updateDatesForShiftFn         func(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error
	findBySessionIDFn             func(ctx *gin.Context, sessionID int64) ([]dbs.GroupCalendarDay, error)
	repointDaysByIDFn             func(ctx *gin.Context, dayIDs []int64, newSessionID int64) error
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
func (m *mockGroupCalendarDao) FindDistinctGroupsBySession(ctx *gin.Context, sessionID int64) ([]int64, error) {
	if m.findDistinctGroupsBySessionFn != nil {
		return m.findDistinctGroupsBySessionFn(ctx, sessionID)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) ClearSourcePlan(ctx *gin.Context, planID int64) error {
	if m.clearSourcePlanFn != nil {
		return m.clearSourcePlanFn(ctx, planID)
	}
	return nil
}
func (m *mockGroupCalendarDao) RepointSessionForGroups(ctx *gin.Context, groupIDs []int64, oldSessionID, newSessionID int64) error {
	if m.repointSessionForGroupsFn != nil {
		return m.repointSessionForGroupsFn(ctx, groupIDs, oldSessionID, newSessionID)
	}
	return nil
}
func (m *mockGroupCalendarDao) UpdateDatesForShift(ctx *gin.Context, groupID int64, oldDate, newDate time.Time) error {
	if m.updateDatesForShiftFn != nil {
		return m.updateDatesForShiftFn(ctx, groupID, oldDate, newDate)
	}
	return nil
}
func (m *mockGroupCalendarDao) FindBySessionID(ctx *gin.Context, sessionID int64) ([]dbs.GroupCalendarDay, error) {
	if m.findBySessionIDFn != nil {
		return m.findBySessionIDFn(ctx, sessionID)
	}
	return nil, nil
}
func (m *mockGroupCalendarDao) RepointDaysByID(ctx *gin.Context, dayIDs []int64, newSessionID int64) error {
	if m.repointDaysByIDFn != nil {
		return m.repointDaysByIDFn(ctx, dayIDs, newSessionID)
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
		return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", SessionID: &sessionID}, nil
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

func TestCalendarService_Stamp_Success(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	planDao := &mockTrainingPlanDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.TrainingPlan, error) {
		return &dbs.TrainingPlan{ID: id, OwnerID: 7}, nil
	}}
	sessionID := int64(3)
	dayDao := &mockPlanDayDao{findByPlanFn: func(ctx *gin.Context, planID int64) ([]dbs.PlanDay, error) {
		return []dbs.PlanDay{
			{SequenceNo: 1, Kind: "rest"},
			{SequenceNo: 2, Kind: "training", SessionID: &sessionID},
		}, nil
	}}
	calDao := &mockGroupCalendarDao{findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
		return nil, nil
	}}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, planDao, dayDao, nil, nil)

	resp, err := svc.Stamp(nil, 1, 7, calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})

	require.NoError(t, err)
	assert.Len(t, resp, 2)
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

func TestCalendarService_Bulk_Success(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	svc := NewCalendarService(&mockGroupCalendarDao{}, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.Bulk(nil, 1, 7, calendar.BulkRequest{Dates: []string{"2026-10-01", "2026-10-02"}, Kind: "rest"})

	require.NoError(t, err)
	assert.Len(t, resp, 2)
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

func TestCalendarService_Shift_NoCollision(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 1}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7}, nil
	}}
	fromDate, _ := time.Parse("2006-01-02", "2026-10-01")
	calDao := &mockGroupCalendarDao{
		findByGroupAndRangeFn: func(ctx *gin.Context, groupID int64, from, to time.Time) ([]dbs.GroupCalendarDay, error) {
			return []dbs.GroupCalendarDay{{GroupID: groupID, Date: fromDate, Kind: "rest"}}, nil
		},
	}
	svc := NewCalendarService(calDao, groupDao, teamDao, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.Shift(nil, 1, 7, calendar.ShiftRequest{FromDate: "2026-10-01", Days: 2})

	require.NoError(t, err)
	assert.Len(t, resp, 1)
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
	sessionID := int64(3)
	nextDate, _ := time.Parse("2006-01-02", "2026-10-10")
	calDao := &mockGroupCalendarDao{findNextSessionForGroupsFn: func(ctx *gin.Context, groupIDs []int64, fromDate time.Time) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{GroupID: 1, Date: nextDate, Kind: "training", SessionID: &sessionID}, nil
	}}
	svc := NewCalendarService(calDao, &mockGroupDao{}, &mockTeamDao{}, groupUserDao, nil, nil, nil, nil, nil)

	resp, err := svc.NextSession(nil, 42)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.GroupID)
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
	presencialTime := "18:30"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial, PresencialTime: &presencialTime,
		PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	}

	resp, err := svc.UpsertDay(nil, 1, 7, time.Now(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.IsPresencial)
	require.NotNil(t, resp.PresencialTime)
	assert.Equal(t, "18:30", *resp.PresencialTime)
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
	presencialTime := "not-a-time"
	req := calendar.CalendarDayRequest{
		Kind: "rest", IsPresencial: &isPresencial, PresencialTime: &presencialTime,
		PresencialLocation: &trainingplan.Location{Lat: -34.6, Lng: -58.4},
	}

	_, err := svc.UpsertDay(nil, 1, 7, time.Now(), req)

	require.Error(t, err)
}

func TestCalendarService_AssignedGroups_Distinct(t *testing.T) {
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, Name: fmt.Sprintf("Grupo %d", id)}, nil
	}}
	calDao := &mockGroupCalendarDao{findDistinctGroupsBySessionFn: func(ctx *gin.Context, sessionID int64) ([]int64, error) {
		return []int64{1, 2}, nil
	}}
	svc := NewCalendarService(calDao, groupDao, &mockTeamDao{}, &mockGroupUserDao{}, nil, nil, nil, nil, nil)

	resp, err := svc.AssignedGroups(nil, 99)

	require.NoError(t, err)
	assert.Len(t, resp, 2)
}
