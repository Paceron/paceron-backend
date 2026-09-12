package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockExerciseService struct {
	createFn func(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	getFn    func(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error)
	listFn   func(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error)
	updateFn func(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	deleteFn func(ctx *gin.Context, id, callerID int64) error
	cloneFn  func(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error)
}

func (m *mockExerciseService) Create(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	return m.createFn(ctx, callerID, req)
}
func (m *mockExerciseService) Get(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error) {
	return m.getFn(ctx, id)
}
func (m *mockExerciseService) List(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error) {
	return m.listFn(ctx, ownerID)
}
func (m *mockExerciseService) Update(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	return m.updateFn(ctx, id, callerID, req)
}
func (m *mockExerciseService) Delete(ctx *gin.Context, id, callerID int64) error {
	return m.deleteFn(ctx, id, callerID)
}
func (m *mockExerciseService) Clone(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error) {
	return m.cloneFn(ctx, id, callerID)
}

func setupExerciseRouter(svc services.ExerciseServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewExerciseController(svc)
	r.POST("/exercises", ctrl.Create)
	r.GET("/exercises/:id", ctrl.Get)
	r.GET("/exercises", ctrl.List)
	r.PUT("/exercises/:id", ctrl.Update)
	r.DELETE("/exercises/:id", ctrl.Delete)
	r.POST("/exercises/:id/clone", ctrl.Clone)
	return r
}

func TestExerciseController_Create_Success(t *testing.T) {
	svc := &mockExerciseService{createFn: func(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
		return &exercise.ExerciseResponse{ID: 1, Name: req.Name}, nil
	}}
	router := setupExerciseRouter(svc, 7)
	body, _ := json.Marshal(exercise.ExerciseRequest{OwnerID: 7, Name: "Trote", Kind: "jogging"})
	req := httptest.NewRequest(http.MethodPost, "/exercises", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestExerciseController_Create_Forbidden(t *testing.T) {
	svc := &mockExerciseService{createFn: func(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
		return nil, services.ErrCatalogForbidden
	}}
	router := setupExerciseRouter(svc, 7)
	body, _ := json.Marshal(exercise.ExerciseRequest{OwnerID: 99, Name: "X", Kind: "running"})
	req := httptest.NewRequest(http.MethodPost, "/exercises", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestExerciseController_Get_NotFound(t *testing.T) {
	svc := &mockExerciseService{getFn: func(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error) {
		return nil, services.ErrExerciseNotFound
	}}
	router := setupExerciseRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/exercises/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestExerciseController_Delete_Success(t *testing.T) {
	svc := &mockExerciseService{deleteFn: func(ctx *gin.Context, id, callerID int64) error { return nil }}
	router := setupExerciseRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/exercises/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
