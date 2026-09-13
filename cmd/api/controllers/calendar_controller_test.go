package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockCalendarService struct {
	getRangeFn        func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error)
	upsertDayFn       func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error)
	deleteDayFn       func(ctx *gin.Context, groupID, callerID int64, date time.Time) error
	stampFn           func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error)
	bulkFn            func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error)
	bulkClearFn       func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error
	shiftFn           func(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error)
	nextSessionFn     func(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error)
	calendarSummaryFn func(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error)
	assignedGroupsFn  func(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error)
}

func (m *mockCalendarService) GetRange(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
	return m.getRangeFn(ctx, groupID, callerID, from, to)
}
func (m *mockCalendarService) UpsertDay(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
	return m.upsertDayFn(ctx, groupID, callerID, date, req)
}
func (m *mockCalendarService) DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error {
	return m.deleteDayFn(ctx, groupID, callerID, date)
}
func (m *mockCalendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
	return m.stampFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error) {
	return m.bulkFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	return m.bulkClearFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error) {
	return m.shiftFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	return m.nextSessionFn(ctx, userID)
}
func (m *mockCalendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	return m.calendarSummaryFn(ctx, userID)
}
func (m *mockCalendarService) AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
	return m.assignedGroupsFn(ctx, sessionID)
}

func setupCalendarRouter(svc services.CalendarServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewCalendarController(svc)
	r.GET("/groups/:id/calendar", ctrl.GetRange)
	r.PUT("/groups/:id/calendar/:date", ctrl.PutDay)
	r.DELETE("/groups/:id/calendar/:date", ctrl.DeleteDay)
	r.POST("/groups/:id/calendar/stamp", ctrl.Stamp)
	r.POST("/groups/:id/calendar/bulk", ctrl.Bulk)
	r.POST("/groups/:id/calendar/bulk-clear", ctrl.BulkClear)
	r.POST("/groups/:id/calendar/shift", ctrl.Shift)
	r.GET("/users/:id/next-session", ctrl.NextSession)
	r.GET("/users/:id/calendar-summary", ctrl.CalendarSummary)
	return r
}

func TestCalendarController_GetRange_MissingDatesReturns400(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_GetRange_Success(t *testing.T) {
	svc := &mockCalendarService{getRangeFn: func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
		return []calendar.CalendarDayResponse{}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar?from=2026-10-01&to=2026-10-31", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_GetRange_Forbidden(t *testing.T) {
	svc := &mockCalendarService{getRangeFn: func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
		return nil, services.ErrCalendarForbidden
	}}
	router := setupCalendarRouter(svc, 99)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar?from=2026-10-01&to=2026-10-31", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCalendarController_PutDay_Success(t *testing.T) {
	svc := &mockCalendarService{upsertDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
		return &calendar.CalendarDayResponse{ID: 1, GroupID: groupID, Kind: req.Kind}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.CalendarDayRequest{Kind: "rest"})
	req := httptest.NewRequest(http.MethodPut, "/groups/1/calendar/2026-10-01", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_DeleteDay_Success(t *testing.T) {
	svc := &mockCalendarService{deleteDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time) error { return nil }}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/groups/1/calendar/2026-10-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCalendarController_Stamp_Conflict(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
		return nil, services.ErrCalendarStampConflict
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCalendarController_NextSession_NoContent(t *testing.T) {
	svc := &mockCalendarService{nextSessionFn: func(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) { return nil, nil }}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/7/next-session", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCalendarController_NextSession_OtherUserForbidden(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/99/next-session", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
