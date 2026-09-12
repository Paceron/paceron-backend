package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
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
