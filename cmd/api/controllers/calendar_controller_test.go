package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockCalendarService struct {
	getRangeFn        func(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error)
	upsertDayFn       func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error)
	deleteDayFn       func(ctx *gin.Context, groupID, callerID int64, date time.Time) error
	stampFn           func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error)
	bulkFn            func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) (calendar.CalendarMutationResponse, error)
	bulkClearFn       func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error
	shiftFn           func(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) (calendar.CalendarMutationResponse, error)
	nextSessionFn     func(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error)
	calendarSummaryFn func(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error)
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
func (m *mockCalendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
	return m.stampFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) (calendar.CalendarMutationResponse, error) {
	return m.bulkFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	return m.bulkClearFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) (calendar.CalendarMutationResponse, error) {
	return m.shiftFn(ctx, groupID, callerID, req)
}
func (m *mockCalendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	return m.nextSessionFn(ctx, userID)
}
func (m *mockCalendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	return m.calendarSummaryFn(ctx, userID)
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
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{}, services.ErrCalendarStampConflict
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCalendarController_Stamp_ConflictIncludesDates(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{}, fmt.Errorf("%w: 2026-10-01, 2026-10-03", services.ErrCalendarStampConflict)
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "2026-10-01")
	assert.Contains(t, rec.Body.String(), "2026-10-03")
}

func TestCalendarController_Stamp_ClosedDayReturns422(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{}, fmt.Errorf("%w: 2026-09-19", services.ErrCalendarDayClosed)
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-09-19"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "2026-09-19")
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

func TestCalendarController_NextSession_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/abc/next-session", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_NextSession_Success(t *testing.T) {
	svc := &mockCalendarService{nextSessionFn: func(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
		return &calendar.NextSessionResponse{Date: "2026-10-01"}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/7/next-session", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_Bulk_Success(t *testing.T) {
	svc := &mockCalendarService{bulkFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{Days: []calendar.CalendarDayResponse{{ID: 1, GroupID: groupID}}}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.BulkRequest{Dates: []string{"2026-10-01"}, Kind: "rest"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_Bulk_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.BulkRequest{Dates: []string{"2026-10-01"}, Kind: "rest"})
	req := httptest.NewRequest(http.MethodPost, "/groups/abc/calendar/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_Bulk_InvalidPayload(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/bulk", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_Bulk_ServiceError(t *testing.T) {
	svc := &mockCalendarService{bulkFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{}, services.ErrCalendarInvalidKind
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.BulkRequest{Dates: []string{"2026-10-01"}, Kind: "invalid"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCalendarController_BulkClear_Success(t *testing.T) {
	svc := &mockCalendarService{bulkClearFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
		return nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.BulkClearRequest{Dates: []string{"2026-10-01"}})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/bulk-clear", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCalendarController_BulkClear_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.BulkClearRequest{Dates: []string{"2026-10-01"}})
	req := httptest.NewRequest(http.MethodPost, "/groups/abc/calendar/bulk-clear", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_BulkClear_InvalidPayload(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/bulk-clear", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_BulkClear_ServiceError(t *testing.T) {
	svc := &mockCalendarService{bulkClearFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
		return services.ErrCalendarForbidden
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.BulkClearRequest{Dates: []string{"2026-10-01"}})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/bulk-clear", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCalendarController_Shift_Success(t *testing.T) {
	svc := &mockCalendarService{shiftFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{Days: []calendar.CalendarDayResponse{{ID: 1, GroupID: groupID}}}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.ShiftRequest{FromDate: "2026-10-01", Days: 1})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/shift", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_Shift_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.ShiftRequest{FromDate: "2026-10-01", Days: 1})
	req := httptest.NewRequest(http.MethodPost, "/groups/abc/calendar/shift", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_Shift_InvalidPayload(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/shift", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_Shift_Collision(t *testing.T) {
	svc := &mockCalendarService{shiftFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{}, services.ErrCalendarShiftCollision
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.ShiftRequest{FromDate: "2026-10-01", Days: 1})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/shift", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCalendarController_CalendarSummary_Success(t *testing.T) {
	svc := &mockCalendarService{calendarSummaryFn: func(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
		return []calendar.CalendarSummaryItem{{GroupID: 1, GroupName: "Grupo A"}}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/7/calendar-summary", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCalendarController_CalendarSummary_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/abc/calendar-summary", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_CalendarSummary_OtherUserForbidden(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/99/calendar-summary", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCalendarController_CalendarSummary_ServiceError(t *testing.T) {
	svc := &mockCalendarService{calendarSummaryFn: func(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
		return nil, services.ErrCalendarGroupNotFound
	}}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/users/7/calendar-summary", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCalendarController_PutDay_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.CalendarDayRequest{Kind: "rest"})
	req := httptest.NewRequest(http.MethodPut, "/groups/abc/calendar/2026-10-01", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_PutDay_InvalidDate(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.CalendarDayRequest{Kind: "rest"})
	req := httptest.NewRequest(http.MethodPut, "/groups/1/calendar/not-a-date", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_PutDay_InvalidPayload(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPut, "/groups/1/calendar/2026-10-01", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_PutDay_FieldMismatch(t *testing.T) {
	svc := &mockCalendarService{upsertDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
		return nil, services.ErrCalendarFieldMismatch
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.CalendarDayRequest{Kind: "rest"})
	req := httptest.NewRequest(http.MethodPut, "/groups/1/calendar/2026-10-01", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCalendarController_DeleteDay_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/groups/abc/calendar/2026-10-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_DeleteDay_InvalidDate(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/groups/1/calendar/not-a-date", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_DeleteDay_ServiceError(t *testing.T) {
	svc := &mockCalendarService{deleteDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time) error {
		return services.ErrCalendarPlanNotFound
	}}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/groups/1/calendar/2026-10-01", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCalendarController_Stamp_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/abc/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_Stamp_InvalidPayload(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_Stamp_PlanForbidden(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{}, services.ErrCalendarPlanForbidden
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCalendarController_GetRange_InvalidFromDate(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar?from=not-a-date&to=2026-10-31", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_GetRange_InvalidToDate(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/1/calendar?from=2026-10-01&to=not-a-date", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_GetRange_InvalidID(t *testing.T) {
	svc := &mockCalendarService{}
	router := setupCalendarRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/groups/abc/calendar?from=2026-10-01&to=2026-10-31", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMapCalendarError_DefaultInternalError(t *testing.T) {
	status, message := mapCalendarError(errors.New("something unexpected"))

	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "error interno", message)
}

func TestMapCalendarError_InvalidCancelTransition(t *testing.T) {
	status, _ := mapCalendarError(services.ErrCalendarInvalidCancelTransition)

	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

func TestMapCalendarError_ClosedDay(t *testing.T) {
	status, message := mapCalendarError(services.ErrCalendarDayClosed)

	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Contains(t, message, "cerrado")
}

func TestMapCalendarError_SessionExerciseNotFound(t *testing.T) {
	status, _ := mapCalendarError(services.ErrSessionExerciseNotFound)

	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

// D8: el wrapper de stamp/bulk/shift viaja como objeto {days, same_team_warnings}.
func TestCalendarController_Stamp_WrapperShape(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{
			Days:             []calendar.CalendarDayResponse{{ID: 1, GroupID: groupID, Date: "2026-10-01"}},
			SameTeamWarnings: []calendar.PresencialConflict{{GroupID: 2, GroupName: "otro", TeamID: 1, TeamName: "team", Date: "2026-10-01", PresencialTimeFrom: "09:00", PresencialTimeTo: "10:00"}},
		}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 1, StartDate: "2026-10-01"})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	assert.Contains(t, parsed, "days")
	assert.Contains(t, parsed, "same_team_warnings")
}

// 3.5 — stamp con exclude_dates que deja el rango válido → 201 con el wrapper
//
//	{days, same_team_warnings}: el endpoint expone el caso "colisión solo en
//	fecha excluida" como Created, no solo como valor de retorno sintético.
func TestCalendarController_Stamp_ExcludeDatesDejaRangoValidoRetorna201(t *testing.T) {
	svc := &mockCalendarService{stampFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) (calendar.CalendarMutationResponse, error) {
		require.Len(t, req.ExcludeDates, 1)
		assert.Equal(t, "2026-10-01", req.ExcludeDates[0])
		return calendar.CalendarMutationResponse{
			Days: []calendar.CalendarDayResponse{{ID: 3, GroupID: groupID, Date: "2026-10-02", Kind: "training", IsPresencial: true}},
		}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.StampRequest{PlanID: 5, StartDate: "2026-10-01", ExcludeDates: []string{"2026-10-01"}})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/stamp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var parsed calendar.CalendarMutationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	require.Len(t, parsed.Days, 1)
	assert.Equal(t, "2026-10-02", parsed.Days[0].Date)
	assert.True(t, parsed.Days[0].IsPresencial)
	assert.Empty(t, parsed.SameTeamWarnings)
}

// D4/D8: sin superposición same-team el campo no aparece (omitempty).
func TestCalendarController_Shift_WrapperOmiteWarningsVacios(t *testing.T) {
	svc := &mockCalendarService{shiftFn: func(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) (calendar.CalendarMutationResponse, error) {
		return calendar.CalendarMutationResponse{Days: []calendar.CalendarDayResponse{{ID: 1, GroupID: groupID}}}, nil
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.ShiftRequest{FromDate: "2026-10-01", Days: 1})
	req := httptest.NewRequest(http.MethodPost, "/groups/1/calendar/shift", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var parsed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	assert.Contains(t, parsed, "days")
	assert.NotContains(t, parsed, "same_team_warnings")
}

// D4: colisión presencial → 409 con body {message, conflicts}.
func TestCalendarController_PutDay_PresencialCollision409(t *testing.T) {
	svc := &mockCalendarService{upsertDayFn: func(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
		return nil, services.ErrCalendarPresencialCollision
	}}
	router := setupCalendarRouter(svc, 7)
	body, _ := json.Marshal(calendar.CalendarDayRequest{Kind: "rest"})
	req := httptest.NewRequest(http.MethodPut, "/groups/1/calendar/2026-10-01", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	var parsed presencialCollisionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
	assert.Equal(t, "colisión presencial con otro equipo", parsed.Message)
	assert.NotNil(t, parsed.Conflicts)
}
