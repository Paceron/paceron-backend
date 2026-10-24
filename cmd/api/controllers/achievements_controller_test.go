package controllers

// --- Endpoints de logros y estampas (gamificacion-core D5, task 3.3/3.6) ---

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
)

type mockAchievementsService struct {
	getCatalogFn   func(ctx *gin.Context) []gamification.AchievementResponse
	getUserFn      func(ctx *gin.Context, targetUserID int64) (*gamification.UserAchievementsResponse, error)
	getStampsFn    func(ctx *gin.Context, targetUserID int64) (*gamification.StampsResponse, error)
	getUserCalls   int
	getStampsCalls int
}

func (m *mockAchievementsService) GetCatalog(ctx *gin.Context) []gamification.AchievementResponse {
	if m.getCatalogFn != nil {
		return m.getCatalogFn(ctx)
	}
	return nil
}

func (m *mockAchievementsService) GetUserAchievements(ctx *gin.Context, targetUserID int64) (*gamification.UserAchievementsResponse, error) {
	m.getUserCalls++
	if m.getUserFn != nil {
		return m.getUserFn(ctx, targetUserID)
	}
	return &gamification.UserAchievementsResponse{}, nil
}

func (m *mockAchievementsService) GetUserStamps(ctx *gin.Context, targetUserID int64) (*gamification.StampsResponse, error) {
	m.getStampsCalls++
	if m.getStampsFn != nil {
		return m.getStampsFn(ctx, targetUserID)
	}
	return &gamification.StampsResponse{}, nil
}

func (m *mockAchievementsService) EvaluateEvent(ctx *gin.Context, userID int64, trigger services.AchievementTrigger) error {
	return nil
}

func (m *mockAchievementsService) OnEvent(ctx *gin.Context, userID int64, trigger services.AchievementTrigger) {
}

func setupAchievementsRouter(t *testing.T, svc services.AchievementsServiceInterface, authUserID int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if authUserID > 0 {
			c.Set(utils.AuthUserIDKey, authUserID)
		}
		c.Next()
	})
	ac := NewAchievementsController(svc)
	router.GET("/api/v1/achievements/catalog", ac.GetCatalog)
	router.GET("/api/v1/users/:id/achievements", ac.GetUserAchievements)
	router.GET("/api/v1/users/:id/stamps", ac.GetUserStamps)
	return router
}

func doAchievementsGet(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	require.NoError(t, err)
	router.ServeHTTP(response, req)
	return response
}

func TestAchievementsController_Unauthorized(t *testing.T) {
	router := setupAchievementsRouter(t, &mockAchievementsService{}, 0)
	for _, path := range []string{"/api/v1/achievements/catalog", "/api/v1/users/7/achievements", "/api/v1/users/7/stamps"} {
		response := doAchievementsGet(t, router, path)
		assert.Equal(t, http.StatusUnauthorized, response.Code, path)
	}
}

func TestAchievementsController_Catalog_SinOwnership(t *testing.T) {
	svc := &mockAchievementsService{getCatalogFn: func(ctx *gin.Context) []gamification.AchievementResponse {
		return []gamification.AchievementResponse{{ID: "first_session", Family: "consistency", Threshold: 1, Unit: "sessions"}}
	}}
	router := setupAchievementsRouter(t, svc, 7)

	// El catálogo es compartido: cualquier usuario autenticado pasa.
	response := doAchievementsGet(t, router, "/api/v1/achievements/catalog")

	require.Equal(t, http.StatusOK, response.Code)
	var resp gamification.AchievementCatalogResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "first_session", resp.Data[0].ID)
}

func TestAchievementsController_Achievements_Success(t *testing.T) {
	now := fixedUnlockedAt()
	svc := &mockAchievementsService{getUserFn: func(ctx *gin.Context, targetUserID int64) (*gamification.UserAchievementsResponse, error) {
		require.Equal(t, int64(7), targetUserID)
		return &gamification.UserAchievementsResponse{
			Data:      []gamification.UserAchievementItem{{AchievementID: "first_session", UnlockedAt: &now, LastUnlockedAt: &now}},
			HasUnseen: true,
		}, nil
	}}
	router := setupAchievementsRouter(t, svc, 7)

	response := doAchievementsGet(t, router, "/api/v1/users/7/achievements")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, svc.getUserCalls)
	var resp gamification.UserAchievementsResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.True(t, resp.HasUnseen)
	require.NotNil(t, resp.Data[0].UnlockedAt)
}

func TestAchievementsController_Achievements_SelfOnly_NoServiceCall(t *testing.T) {
	svc := &mockAchievementsService{}
	router := setupAchievementsRouter(t, svc, 7)

	response := doAchievementsGet(t, router, "/api/v1/users/99/achievements")

	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Equal(t, 0, svc.getUserCalls, "el guard self-only corta antes del servicio")
	assert.NotContains(t, response.Body.String(), "99")
}

func TestAchievementsController_Stamps_SelfOnly_Y_Vacia(t *testing.T) {
	svc := &mockAchievementsService{}
	router := setupAchievementsRouter(t, svc, 7)

	response := doAchievementsGet(t, router, "/api/v1/users/7/stamps")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, svc.getStampsCalls)
	var resp gamification.StampsResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data)

	ajena := doAchievementsGet(t, router, "/api/v1/users/99/stamps")
	assert.Equal(t, http.StatusForbidden, ajena.Code)
	assert.Equal(t, 1, svc.getStampsCalls, "la vitrina ajena no toca el servicio")
}

func TestAchievementsController_BadRequest_ID(t *testing.T) {
	svc := &mockAchievementsService{}
	router := setupAchievementsRouter(t, svc, 7)
	response := doAchievementsGet(t, router, "/api/v1/users/0/achievements")
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, 0, svc.getUserCalls)
}

func TestAchievementsController_InternalError_NoLeak(t *testing.T) {
	svc := &mockAchievementsService{getUserFn: func(ctx *gin.Context, targetUserID int64) (*gamification.UserAchievementsResponse, error) {
		return nil, errors.New("SQLSTATE 23505: detalle crudo del driver")
	}}
	router := setupAchievementsRouter(t, svc, 7)

	response := doAchievementsGet(t, router, "/api/v1/users/7/achievements")

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "error interno", httpResponseMessage(t, response))
	assert.NotContains(t, response.Body.String(), "SQLSTATE")
	assert.NotContains(t, response.Body.String(), "23505")
}

// FailingDB no-leak pin: SELECT falla con Postgres real de testtools y el
// endpoint responde 500 genérico sin filtrar el inyectado.
func TestAchievementsController_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool {
		return op == "select"
	})
	realSvc := services.NewAchievementsService(daos.NewAchievementDao(failing), daos.NewGamificationDao(failing), nil, nil, nil)
	router := setupAchievementsRouter(t, realSvc, 7)

	response := doAchievementsGet(t, router, "/api/v1/users/7/achievements")

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "error interno", httpResponseMessage(t, response))
	assert.NotContains(t, response.Body.String(), "inyectado", "sin leak del error de DB de test")
	assert.NotContains(t, response.Body.String(), "FailingDB")
}

func fixedUnlockedAt() time.Time {
	return time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
}
