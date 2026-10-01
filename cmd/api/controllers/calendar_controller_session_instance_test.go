package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/instance"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

func setupInstanceDetailRouter(svc services.CalendarServiceInterface, withAuth bool, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withAuth {
		r.Use(func(c *gin.Context) {
			c.Set(utils.AuthUserIDKey, authUserID)
			c.Next()
		})
	}
	ctrl := NewCalendarController(svc)
	r.GET("/session-instances/:id", ctrl.SessionInstanceDetail)
	return r
}

func TestCalendarController_SessionInstanceDetail_Success(t *testing.T) {
	resp := &instance.SessionInstanceResponse{ID: 88, Name: "Series de velocidad"}
	var gotID, gotCaller int64
	svc := &mockCalendarService{
		sessionInstanceDetailFn: func(ctx *gin.Context, id, callerID int64) (*instance.SessionInstanceResponse, error) {
			gotID, gotCaller = id, callerID
			return resp, nil
		},
	}
	router := setupInstanceDetailRouter(svc, true, 7)
	req := httptest.NewRequest(http.MethodGet, "/session-instances/88", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(88), gotID)
	assert.Equal(t, int64(7), gotCaller)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, float64(88), body["id"])
	assert.Equal(t, "Series de velocidad", body["name"])
}

func TestCalendarController_SessionInstanceDetail_Unauthorized(t *testing.T) {
	svc := &mockCalendarService{
		sessionInstanceDetailFn: func(ctx *gin.Context, id, callerID int64) (*instance.SessionInstanceResponse, error) {
			return nil, assert.AnError
		},
	}
	router := setupInstanceDetailRouter(svc, false, 0)
	req := httptest.NewRequest(http.MethodGet, "/session-instances/88", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCalendarController_SessionInstanceDetail_InvalidID(t *testing.T) {
	svc := &mockCalendarService{
		sessionInstanceDetailFn: func(ctx *gin.Context, id, callerID int64) (*instance.SessionInstanceResponse, error) {
			return nil, nil
		},
	}
	router := setupInstanceDetailRouter(svc, true, 7)
	req := httptest.NewRequest(http.MethodGet, "/session-instances/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCalendarController_SessionInstanceDetail_NotFound(t *testing.T) {
	svc := &mockCalendarService{
		sessionInstanceDetailFn: func(ctx *gin.Context, id, callerID int64) (*instance.SessionInstanceResponse, error) {
			return nil, services.ErrCalendarInstanceNotFound
		},
	}
	router := setupInstanceDetailRouter(svc, true, 7)
	req := httptest.NewRequest(http.MethodGet, "/session-instances/999999999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCalendarController_SessionInstanceDetail_Forbidden(t *testing.T) {
	svc := &mockCalendarService{
		sessionInstanceDetailFn: func(ctx *gin.Context, id, callerID int64) (*instance.SessionInstanceResponse, error) {
			return nil, services.ErrCalendarForbidden
		},
	}
	router := setupInstanceDetailRouter(svc, true, 7)
	req := httptest.NewRequest(http.MethodGet, "/session-instances/88", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
