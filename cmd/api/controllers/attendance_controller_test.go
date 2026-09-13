package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/services"
)

type mockAttendanceService struct {
	generateQRFn func(ctx *gin.Context, teamID, sessionID int64) (*attendance.QRResponse, error)
	registerFn   func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error)
	searchFn     func(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error)
}

func (m *mockAttendanceService) GenerateQR(ctx *gin.Context, teamID, sessionID int64) (*attendance.QRResponse, error) {
	if m.generateQRFn != nil {
		return m.generateQRFn(ctx, teamID, sessionID)
	}
	return nil, nil
}

func (m *mockAttendanceService) Register(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, userID, teamID, sessionID)
	}
	return false, nil
}

func (m *mockAttendanceService) Search(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, authUserID, filters)
	}
	return nil, nil
}

func TestAttendanceController_GenerateQR_Success(t *testing.T) {
	mock := &mockAttendanceService{
		generateQRFn: func(ctx *gin.Context, teamID, sessionID int64) (*attendance.QRResponse, error) {
			assert.Equal(t, int64(5), teamID)
			assert.Equal(t, int64(9), sessionID)
			return &attendance.QRResponse{QRCodeBase64: "cG5n", URLEncoded: "http://localhost:8080/api/v1/attendance/team/5/session/9"}, nil
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/qr?team_id=5&training_session_id=9", nil)
	setAuthUserID(c, 1)

	controller.GenerateQR(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "cG5n")
}

func TestAttendanceController_GenerateQR_MissingParams(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/qr", nil)
	setAuthUserID(c, 1)

	controller.GenerateQR(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestAttendanceController_GenerateQR_InvalidParam(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/qr?team_id=0&training_session_id=9", nil)
	setAuthUserID(c, 1)

	controller.GenerateQR(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestAttendanceController_GenerateQR_Unauthorized(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/qr?team_id=5&training_session_id=9", nil)

	controller.GenerateQR(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestAttendanceController_RegisterAttendance_Created(t *testing.T) {
	mock := &mockAttendanceService{
		registerFn: func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error) {
			assert.Equal(t, int64(7), userID)
			assert.Equal(t, int64(5), teamID)
			assert.Equal(t, int64(9), sessionID)
			return true, nil
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/team/5/session/9", strings.NewReader(""))
	c.Params = []gin.Param{{Key: "team_id", Value: "5"}, {Key: "training_session_id", Value: "9"}}
	setAuthUserID(c, 7)

	controller.RegisterAttendance(c)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Contains(t, response.Body.String(), attendance.MessageRegistered)
}

func TestAttendanceController_RegisterAttendance_AlreadyExists(t *testing.T) {
	mock := &mockAttendanceService{
		registerFn: func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error) {
			return false, nil
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/team/5/session/9", strings.NewReader(""))
	c.Params = []gin.Param{{Key: "team_id", Value: "5"}, {Key: "training_session_id", Value: "9"}}
	setAuthUserID(c, 7)

	controller.RegisterAttendance(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), attendance.MessageAlreadyExists)
}

func TestAttendanceController_RegisterAttendance_InvalidPathParam(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/team/abc/session/9", strings.NewReader(""))
	c.Params = []gin.Param{{Key: "team_id", Value: "abc"}, {Key: "training_session_id", Value: "9"}}
	setAuthUserID(c, 7)

	controller.RegisterAttendance(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestAttendanceController_RegisterAttendance_Unauthorized(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/team/5/session/9", strings.NewReader(""))
	c.Params = []gin.Param{{Key: "team_id", Value: "5"}, {Key: "training_session_id", Value: "9"}}

	controller.RegisterAttendance(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestAttendanceController_RegisterAttendance_InternalError(t *testing.T) {
	mock := &mockAttendanceService{
		registerFn: func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error) {
			return false, errors.New("db caída")
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/attendance/team/5/session/9", strings.NewReader(""))
	c.Params = []gin.Param{{Key: "team_id", Value: "5"}, {Key: "training_session_id", Value: "9"}}
	setAuthUserID(c, 7)

	controller.RegisterAttendance(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestAttendanceController_Search_Success(t *testing.T) {
	mock := &mockAttendanceService{
		searchFn: func(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error) {
			assert.Equal(t, int64(1), authUserID)
			return []dbs.Attendance{{ID: 1, TeamID: 5, TrainingSessionID: 9, UserID: 1}}, nil
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search", nil)
	setAuthUserID(c, 1)

	controller.Search(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "\"data\"")
}

func TestAttendanceController_Search_InvalidParam(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search?team_id=-1", nil)
	setAuthUserID(c, 1)

	controller.Search(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestAttendanceController_Search_Forbidden(t *testing.T) {
	mock := &mockAttendanceService{
		searchFn: func(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error) {
			return nil, services.ErrForbiddenAttendance
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search?user_id=99", nil)
	setAuthUserID(c, 1)

	controller.Search(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestAttendanceController_Search_NotFound(t *testing.T) {
	mock := &mockAttendanceService{
		searchFn: func(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error) {
			return nil, services.ErrTeamNotFound
		},
	}
	controller := NewAttendanceController(mock)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search?team_id=999", nil)
	setAuthUserID(c, 1)

	controller.Search(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}
