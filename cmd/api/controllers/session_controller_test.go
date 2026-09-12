package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockSessionService struct {
	createFn func(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	getFn    func(ctx *gin.Context, id int64) (*session.SessionResponse, error)
	listFn   func(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error)
	updateFn func(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	deleteFn func(ctx *gin.Context, id, callerID int64) error
	cloneFn  func(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error)
}

func (m *mockSessionService) Create(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	return m.createFn(ctx, callerID, req)
}
func (m *mockSessionService) Get(ctx *gin.Context, id int64) (*session.SessionResponse, error) {
	return m.getFn(ctx, id)
}
func (m *mockSessionService) List(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error) {
	return m.listFn(ctx, ownerID)
}
func (m *mockSessionService) Update(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	return m.updateFn(ctx, id, callerID, req)
}
func (m *mockSessionService) Delete(ctx *gin.Context, id, callerID int64) error {
	return m.deleteFn(ctx, id, callerID)
}
func (m *mockSessionService) Clone(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error) {
	return m.cloneFn(ctx, id, callerID)
}

func setupSessionRouter(svc services.SessionServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewSessionController(svc)
	r.POST("/sessions", ctrl.Create)
	r.GET("/sessions/:id", ctrl.Get)
	r.GET("/sessions", ctrl.List)
	r.PUT("/sessions/:id", ctrl.Update)
	r.DELETE("/sessions/:id", ctrl.Delete)
	r.POST("/sessions/:id/clone", ctrl.Clone)
	return r
}

func TestSessionController_Create_Success(t *testing.T) {
	svc := &mockSessionService{createFn: func(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
		return &session.SessionResponse{ID: 1, Name: req.Name}, nil
	}}
	router := setupSessionRouter(svc, 7)
	body, _ := json.Marshal(session.SessionRequest{OwnerID: 7, Name: "Sesión", Exercises: []session.SessionExerciseRequest{
		{ExerciseID: 1, Role: "warmup"}, {ExerciseID: 2, Role: "main"}, {ExerciseID: 3, Role: "cooldown"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestSessionController_Create_MissingRole(t *testing.T) {
	svc := &mockSessionService{createFn: func(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
		return nil, services.ErrSessionMissingRole
	}}
	router := setupSessionRouter(svc, 7)
	body, _ := json.Marshal(session.SessionRequest{OwnerID: 7, Name: "X", Exercises: []session.SessionExerciseRequest{}})
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestSessionController_Delete_Success(t *testing.T) {
	svc := &mockSessionService{deleteFn: func(ctx *gin.Context, id, callerID int64) error { return nil }}
	router := setupSessionRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/sessions/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
