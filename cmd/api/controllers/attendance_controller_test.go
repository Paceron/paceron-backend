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
	"simple-arq-golang/cmd/api/utils"
)

type mockAttendanceService struct {
	generateQRFn     func(ctx *gin.Context, authUserID, teamID, sessionID int64) (*attendance.QRResponse, error)
	registerFn       func(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error)
	searchFn         func(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error)
	listSessionsFn   func(ctx *gin.Context, authUserID, teamID, groupID int64) (*attendance.SessionAttendanceListResponse, error)
	getSessionGridFn func(ctx *gin.Context, authUserID, teamID, groupID, sessionInstanceID int64) (*attendance.SessionAttendanceResponse, error)
	bulkSaveFn       func(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error)
	deleteFn         func(ctx *gin.Context, authUserID, teamID, attendanceID int64) error
}

func (m *mockAttendanceService) GenerateQR(ctx *gin.Context, authUserID, teamID, sessionID int64) (*attendance.QRResponse, error) {
	if m.generateQRFn != nil {
		return m.generateQRFn(ctx, authUserID, teamID, sessionID)
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

func (m *mockAttendanceService) ListAttendanceSessions(ctx *gin.Context, authUserID, teamID, groupID int64) (*attendance.SessionAttendanceListResponse, error) {
	if m.listSessionsFn != nil {
		return m.listSessionsFn(ctx, authUserID, teamID, groupID)
	}
	return &attendance.SessionAttendanceListResponse{}, nil
}

func (m *mockAttendanceService) GetSessionAttendance(ctx *gin.Context, authUserID, teamID, groupID, sessionInstanceID int64) (*attendance.SessionAttendanceResponse, error) {
	if m.getSessionGridFn != nil {
		return m.getSessionGridFn(ctx, authUserID, teamID, groupID, sessionInstanceID)
	}
	return &attendance.SessionAttendanceResponse{}, nil
}

func (m *mockAttendanceService) BulkSaveAttendance(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error) {
	if m.bulkSaveFn != nil {
		return m.bulkSaveFn(ctx, authUserID, teamID, sessionInstanceID, userIDs)
	}
	return &attendance.BulkSaveResult{}, nil
}

func (m *mockAttendanceService) DeleteAttendance(ctx *gin.Context, authUserID, teamID, attendanceID int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, authUserID, teamID, attendanceID)
	}
	return nil
}

func TestAttendanceController_GenerateQR_Success(t *testing.T) {
	mock := &mockAttendanceService{
		generateQRFn: func(ctx *gin.Context, authUserID, teamID, sessionID int64) (*attendance.QRResponse, error) {
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

func TestAttendanceController_Search_MandatoryParam(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search", nil)
	setAuthUserID(c, 1)

	controller.Search(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestAttendanceController_Search_MissingTeamID(t *testing.T) {
	controller := NewAttendanceController(&mockAttendanceService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search?training_session_id=9", nil)
	setAuthUserID(c, 1)

	controller.Search(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
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
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search?team_id=5", nil)
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
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/attendance/search?team_id=5&user_id=99", nil)
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

// attendanceRouteHelper arma un router con el handler y ejecuta la request, para
// probar la matriz de status codes sin levantar la app entera.
func attendanceRouteHelper(t *testing.T, method, path, target string, authUserID *int64, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Handle(method, target, func(c *gin.Context) {
		if authUserID != nil {
			c.Set(utils.AuthUserIDKey, *authUserID)
		}
		handler(c)
	})
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAttendanceController_ListAttendanceSessions_StatusMatrix(t *testing.T) {
	auth := int64(7)

	tests := []struct {
		name       string
		path       string
		authUserID *int64
		svcErr     error
		wantStatus int
	}{
		{
			name: "sin auth es 401", path: "/api/v1/groups/7/attendance-sessions?team_id=5",
			authUserID: nil, wantStatus: http.StatusUnauthorized,
		},
		{
			name: "sin team_id es 400", path: "/api/v1/groups/7/attendance-sessions",
			authUserID: &auth, wantStatus: http.StatusBadRequest,
		},
		{
			name: "team_id no numerico es 400", path: "/api/v1/groups/7/attendance-sessions?team_id=abc",
			authUserID: &auth, wantStatus: http.StatusBadRequest,
		},
		{
			name: "grupo de otro equipo es 404", path: "/api/v1/groups/7/attendance-sessions?team_id=5",
			authUserID: &auth, svcErr: services.ErrAttendanceGroupNotFound, wantStatus: http.StatusNotFound,
		},
		{
			name: "no es entrenador es 403", path: "/api/v1/groups/7/attendance-sessions?team_id=5",
			authUserID: &auth, svcErr: services.ErrForbiddenAttendance, wantStatus: http.StatusForbidden,
		},
		{
			name: "ok", path: "/api/v1/groups/7/attendance-sessions?team_id=5",
			authUserID: &auth, wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAttendanceService{
				listSessionsFn: func(ctx *gin.Context, authUserID, teamID, groupID int64) (*attendance.SessionAttendanceListResponse, error) {
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &attendance.SessionAttendanceListResponse{Sessions: []attendance.SessionAttendanceOption{}}, nil
				},
			}
			ctrl := NewAttendanceController(mock)

			rec := attendanceRouteHelper(t, http.MethodGet, tt.path, "/api/v1/groups/:id/attendance-sessions", tt.authUserID, ctrl.ListAttendanceSessions)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAttendanceController_GetSessionAttendance_StatusMatrix(t *testing.T) {
	auth := int64(7)
	target := "/api/v1/attendance/session/:session_instance_id"

	tests := []struct {
		name       string
		path       string
		authUserID *int64
		svcErr     error
		wantStatus int
	}{
		{
			name: "sin auth es 401", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: nil, wantStatus: http.StatusUnauthorized,
		},
		{
			name: "sin team_id es 400", path: "/api/v1/attendance/session/42?group_id=7",
			authUserID: &auth, wantStatus: http.StatusBadRequest,
		},
		{
			name: "sin group_id es 400", path: "/api/v1/attendance/session/42?team_id=5",
			authUserID: &auth, wantStatus: http.StatusBadRequest,
		},
		{
			name: "session id invalido es 400", path: "/api/v1/attendance/session/abc?team_id=5&group_id=7",
			authUserID: &auth, wantStatus: http.StatusBadRequest,
		},
		{
			name: "no es entrenador es 403", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: &auth, svcErr: services.ErrForbiddenAttendance, wantStatus: http.StatusForbidden,
		},
		{
			name: "equipo inexistente es 404", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: &auth, svcErr: services.ErrTeamNotFound, wantStatus: http.StatusNotFound,
		},
		{
			name: "sesion no presencial es 422", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: &auth, svcErr: services.ErrAttendanceSessionNotPresencial, wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "sesion inexistente es 422", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: &auth, svcErr: services.ErrAttendanceSessionNotFound, wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "sesion de otro equipo es 422", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: &auth, svcErr: services.ErrAttendanceSessionWrongTeam, wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "group_id que no es el de la sesion es 422", path: "/api/v1/attendance/session/42?team_id=5&group_id=9",
			authUserID: &auth, svcErr: services.ErrAttendanceGroupMismatch, wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "ok", path: "/api/v1/attendance/session/42?team_id=5&group_id=7",
			authUserID: &auth, wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAttendanceService{
				getSessionGridFn: func(ctx *gin.Context, authUserID, teamID, groupID, sessionInstanceID int64) (*attendance.SessionAttendanceResponse, error) {
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					return &attendance.SessionAttendanceResponse{}, nil
				},
			}
			ctrl := NewAttendanceController(mock)

			rec := attendanceRouteHelper(t, http.MethodGet, tt.path, target, tt.authUserID, ctrl.GetSessionAttendance)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func newBulkRouter(mock *mockAttendanceService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/attendance/bulk", func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, int64(3))
		c.Next()
	}, NewAttendanceController(mock).BulkSaveAttendance)
	return r
}

func TestAttendanceController_BulkSaveAttendance(t *testing.T) {
	post := func(t *testing.T, body string) *httptest.ResponseRecorder {
		t.Helper()
		mock := &mockAttendanceService{
			bulkSaveFn: func(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error) {
				return &attendance.BulkSaveResult{Created: len(userIDs), Updated: 0}, nil
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/attendance/bulk", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newBulkRouter(mock).ServeHTTP(w, req)
		return w
	}

	t.Run("200 con los contadores", func(t *testing.T) {
		w := post(t, `{"team_id":5,"training_session_id":42,"entries":[{"user_id":12},{"user_id":13}]}`)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"created":2,"updated":0}`, w.Body.String())
	})

	t.Run("entries vacio es un no-op 200, no un 400", func(t *testing.T) {
		// La spec lo define explicitamente como no-op: un lote vacio no es un body
		// invalido, es una peticion que no tiene nada que escribir.
		w := post(t, `{"team_id":5,"training_session_id":42,"entries":[]}`)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"created":0,"updated":0}`, w.Body.String())
	})

	t.Run("400 con team_id invalido", func(t *testing.T) {
		w := post(t, `{"team_id":0,"training_session_id":42,"entries":[{"user_id":12}]}`)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "team_id")
	})

	t.Run("400 con un user_id invalido en entries", func(t *testing.T) {
		w := post(t, `{"team_id":5,"training_session_id":42,"entries":[{"user_id":12},{"user_id":0}]}`)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "entries[1].user_id")
	})

	t.Run("400 con body no json", func(t *testing.T) {
		w := post(t, `no soy json`)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("422 lista los user_id que no eran miembros", func(t *testing.T) {
		mock := &mockAttendanceService{
			bulkSaveFn: func(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error) {
				return nil, &services.ErrBulkInvalidUsers{UserIDs: []int64{999}}
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/attendance/bulk", strings.NewReader(`{"team_id":5,"training_session_id":42,"entries":[{"user_id":12},{"user_id":999}]}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newBulkRouter(mock).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		// El frontend necesita los ids concretos para marcar en la grilla solo las
		// filas que no corresponden, no un mensaje generico.
		assert.Contains(t, w.Body.String(), "999")
		assert.Contains(t, w.Body.String(), "details")
	})

	t.Run("403 si no es entrenador", func(t *testing.T) {
		mock := &mockAttendanceService{
			bulkSaveFn: func(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error) {
				return nil, services.ErrForbiddenAttendance
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/attendance/bulk", strings.NewReader(`{"team_id":5,"training_session_id":42,"entries":[{"user_id":12}]}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newBulkRouter(mock).ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("422 si la sesion no es presencial", func(t *testing.T) {
		mock := &mockAttendanceService{
			bulkSaveFn: func(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error) {
				return nil, services.ErrAttendanceSessionNotPresencial
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/attendance/bulk", strings.NewReader(`{"team_id":5,"training_session_id":42,"entries":[{"user_id":12}]}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newBulkRouter(mock).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestAttendanceController_DeleteAttendance(t *testing.T) {
	routerFor := func(mock *mockAttendanceService) *gin.Engine {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.DELETE("/api/v1/attendance/:attendance_id", func(c *gin.Context) {
			c.Set(utils.AuthUserIDKey, int64(3))
			c.Next()
		}, NewAttendanceController(mock).DeleteAttendance)
		return r
	}

	do := func(t *testing.T, url string, svcErr error) *httptest.ResponseRecorder {
		t.Helper()
		mock := &mockAttendanceService{
			deleteFn: func(ctx *gin.Context, authUserID, teamID, attendanceID int64) error {
				return svcErr
			},
		}
		req := httptest.NewRequest(http.MethodDelete, url, nil)
		w := httptest.NewRecorder()
		routerFor(mock).ServeHTTP(w, req)
		return w
	}

	t.Run("204 sin body", func(t *testing.T) {
		w := do(t, "/api/v1/attendance/88?team_id=5", nil)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("404 si no existe", func(t *testing.T) {
		w := do(t, "/api/v1/attendance/99999?team_id=5", services.ErrAttendanceNotFound)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("403 si es de otro equipo", func(t *testing.T) {
		w := do(t, "/api/v1/attendance/88?team_id=5", services.ErrAttendanceForbiddenTeam)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("400 si falta team_id", func(t *testing.T) {
		w := do(t, "/api/v1/attendance/88", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "team_id es obligatorio")
	})

	t.Run("400 si team_id no es numerico", func(t *testing.T) {
		w := do(t, "/api/v1/attendance/88?team_id=abc", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("400 si attendance_id no es numerico", func(t *testing.T) {
		w := do(t, "/api/v1/attendance/abc?team_id=5", nil)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "attendance_id")
	})
}
