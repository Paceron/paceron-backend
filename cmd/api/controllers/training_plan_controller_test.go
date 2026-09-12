package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockTrainingPlanService struct {
	createFn func(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error)
	getFn    func(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error)
	listFn   func(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error)
	updateFn func(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error)
	deleteFn func(ctx *gin.Context, id, callerID int64) error
	cloneFn  func(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error)
}

func (m *mockTrainingPlanService) Create(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
	return m.createFn(ctx, callerID, req)
}
func (m *mockTrainingPlanService) Get(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error) {
	return m.getFn(ctx, id)
}
func (m *mockTrainingPlanService) List(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error) {
	return m.listFn(ctx, ownerID)
}
func (m *mockTrainingPlanService) Update(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error) {
	return m.updateFn(ctx, id, callerID, req)
}
func (m *mockTrainingPlanService) Delete(ctx *gin.Context, id, callerID int64) error {
	return m.deleteFn(ctx, id, callerID)
}
func (m *mockTrainingPlanService) Clone(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error) {
	return m.cloneFn(ctx, id, callerID)
}

func setupTrainingPlanRouter(svc services.TrainingPlanServiceInterface, authUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(utils.AuthUserIDKey, authUserID)
		c.Next()
	})
	ctrl := NewTrainingPlanController(svc)
	r.POST("/training-plans", ctrl.Create)
	r.GET("/training-plans/:id", ctrl.Get)
	r.GET("/training-plans", ctrl.List)
	r.PUT("/training-plans/:id", ctrl.Update)
	r.DELETE("/training-plans/:id", ctrl.Delete)
	r.POST("/training-plans/:id/clone", ctrl.Clone)
	return r
}

func TestTrainingPlanController_Create_Success(t *testing.T) {
	svc := &mockTrainingPlanService{createFn: func(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
		return &trainingplan.TrainingPlanResponse{ID: 1, Name: req.Name}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	body, _ := json.Marshal(trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "rest"}, {SequenceNo: 2, Kind: "rest"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/training-plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTrainingPlanController_Create_InvalidDayCount(t *testing.T) {
	svc := &mockTrainingPlanService{createFn: func(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
		return nil, services.ErrPlanInvalidDayCount
	}}
	router := setupTrainingPlanRouter(svc, 7)
	body, _ := json.Marshal(trainingplan.TrainingPlanRequest{OwnerID: 7, Name: "Plan", Days: []trainingplan.PlanDayRequest{{SequenceNo: 1, Kind: "rest"}}})
	req := httptest.NewRequest(http.MethodPost, "/training-plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestTrainingPlanController_Update_Partial(t *testing.T) {
	svc := &mockTrainingPlanService{updateFn: func(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error) {
		return &trainingplan.TrainingPlanResponse{ID: id, Name: *req.Name}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	body, _ := json.Marshal(trainingplan.TrainingPlanUpdateRequest{Name: strPtrTPController("Nuevo nombre")})
	req := httptest.NewRequest(http.MethodPut, "/training-plans/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTrainingPlanController_Delete_Success(t *testing.T) {
	svc := &mockTrainingPlanService{deleteFn: func(ctx *gin.Context, id, callerID int64) error { return nil }}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodDelete, "/training-plans/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTrainingPlanController_Get_Success(t *testing.T) {
	svc := &mockTrainingPlanService{getFn: func(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error) {
		return &trainingplan.TrainingPlanResponse{ID: id, Name: "Plan"}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/training-plans/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTrainingPlanController_Get_NotFound(t *testing.T) {
	svc := &mockTrainingPlanService{getFn: func(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error) {
		return nil, services.ErrPlanNotFound
	}}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/training-plans/1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTrainingPlanController_List_Success(t *testing.T) {
	svc := &mockTrainingPlanService{listFn: func(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error) {
		return []trainingplan.TrainingPlanResponse{{ID: 1, OwnerID: ownerID, Name: "Plan"}}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/training-plans?owner_id=7", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTrainingPlanController_List_InvalidOwnerID(t *testing.T) {
	svc := &mockTrainingPlanService{}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodGet, "/training-plans?owner_id=abc", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTrainingPlanController_Clone_Success(t *testing.T) {
	svc := &mockTrainingPlanService{cloneFn: func(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error) {
		return &trainingplan.TrainingPlanResponse{ID: id + 1, Name: "Plan (copia)"}, nil
	}}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPost, "/training-plans/1/clone", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTrainingPlanController_Clone_Forbidden(t *testing.T) {
	svc := &mockTrainingPlanService{cloneFn: func(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error) {
		return nil, services.ErrCatalogForbidden
	}}
	router := setupTrainingPlanRouter(svc, 7)
	req := httptest.NewRequest(http.MethodPost, "/training-plans/1/clone", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func strPtrTPController(s string) *string { return &s }
