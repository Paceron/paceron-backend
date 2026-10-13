package controllers

// Leaderboard por team (gamificacion-core D7, E3): guards/400 + no-leak.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
)

type mockLeaderboardService struct {
	getFn      func(callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error)
	getCalls   int
	lastMetric string
	lastPeriod string
}

func (m *mockLeaderboardService) GetLeaderboard(ctx *gin.Context, callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error) {
	m.getCalls++
	m.lastMetric = metric
	m.lastPeriod = period
	if m.getFn != nil {
		return m.getFn(callerID, teamID, metric, period, from, to)
	}
	return &gamification.LeaderboardResponse{Rows: []gamification.LeaderboardRow{}}, nil
}

func setupLeaderboardRouter(t *testing.T, svc services.LeaderboardServiceInterface, authUserID int64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if authUserID > 0 {
			c.Set(utils.AuthUserIDKey, authUserID)
		}
		c.Next()
	})
	lc := NewLeaderboardController(svc)
	router.GET("/api/v1/teams/:id/leaderboard", lc.GetLeaderboard)
	return router
}

func doLeaderboardGet(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	require.NoError(t, err)
	router.ServeHTTP(response, req)
	return response
}

func leaderboardResponseField(t *testing.T, response *httptest.ResponseRecorder, field string) string {
	t.Helper()
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
	value, ok := errResp[field].(string)
	require.True(t, ok)
	return value
}

func TestLeaderboardController_Unauthorized(t *testing.T) {
	router := setupLeaderboardRouter(t, &mockLeaderboardService{}, 0)
	response := doLeaderboardGet(t, router, "/api/v1/teams/2/leaderboard?metric=distance&period=all")
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestLeaderboardController_PathIDInvalido(t *testing.T) {
	svc := &mockLeaderboardService{}
	router := setupLeaderboardRouter(t, svc, 7)
	response := doLeaderboardGet(t, router, "/api/v1/teams/abc/leaderboard?metric=distance&period=all")
	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Zero(t, svc.getCalls)
}

func TestLeaderboardController_Success_YSwaggerShape(t *testing.T) {
	svc := &mockLeaderboardService{getFn: func(callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error) {
		require.Equal(t, int64(7), callerID)
		require.Equal(t, int64(2), teamID)
		require.Equal(t, "distance", metric)
		require.Equal(t, "all", period)
		return &gamification.LeaderboardResponse{
			Rows: []gamification.LeaderboardRow{{
				UserID: 3, Name: "Ana", Value: 10000, Rank: 1,
			}},
			ViewerRank: &gamification.LeaderboardViewerRank{UserID: 3, Value: 10000, Rank: 1},
			Record:     &gamification.LeaderboardRecord{UserID: 3, Name: "Ana", Value: 10000, AchievedAt: "2026-10-09"},
		}, nil
	}}
	router := setupLeaderboardRouter(t, svc, 7)

	response := doLeaderboardGet(t, router, "/api/v1/teams/2/leaderboard?metric=distance&period=all")

	require.Equal(t, http.StatusOK, response.Code)
	var resp gamification.LeaderboardResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Len(t, resp.Rows, 1)
	assert.Equal(t, int64(3), resp.Rows[0].UserID)
	assert.Equal(t, "Ana", resp.Rows[0].Name)
	assert.Equal(t, 10000.0, resp.Rows[0].Value)
	assert.Equal(t, 1, resp.Rows[0].Rank)
	require.NotNil(t, resp.ViewerRank)
	assert.Equal(t, 1, resp.ViewerRank.Rank)
	require.NotNil(t, resp.Record)
	assert.Equal(t, "2026-10-09", resp.Record.AchievedAt)
}

func TestLeaderboardService_QueryParamsLleganAlService(t *testing.T) {
	svc := &mockLeaderboardService{}
	router := setupLeaderboardRouter(t, svc, 7)

	doLeaderboardGet(t, router, "/api/v1/teams/2/leaderboard?metric=peak_speed&period=custom&from=2026-10-01&to=2026-10-13")

	require.Equal(t, 1, svc.getCalls)
	assert.Equal(t, "peak_speed", svc.lastMetric)
	assert.Equal(t, "custom", svc.lastPeriod)
}

func TestLeaderboardController_ErroresMapeados(t *testing.T) {
	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{"metric/period invalido", services.ErrLeaderboardInvalid, http.StatusBadRequest, "Bad request"},
		{"ajeno al team", services.ErrLeaderboardForbidden, http.StatusForbidden, "Forbidden"},
	}
	for _, tc := range cases {
		svc := &mockLeaderboardService{getFn: func(callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error) {
			return nil, tc.serviceErr
		}}
		router := setupLeaderboardRouter(t, svc, 7)
		response := doLeaderboardGet(t, router, "/api/v1/teams/2/leaderboard?metric=distance&period=all")
		require.Equal(t, tc.wantStatus, response.Code, tc.name)
		assert.Equal(t, tc.wantCode, leaderboardResponseField(t, response, "code"), tc.name)
	}

	// Error crudo del DAO → 500 con mensaje genérico fijo.
	svc := &mockLeaderboardService{getFn: func(callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error) {
		return nil, errors.New("SQLSTATE 23505 detalle interno")
	}}
	router := setupLeaderboardRouter(t, svc, 7)
	response := doLeaderboardGet(t, router, "/api/v1/teams/2/leaderboard?metric=distance&period=all")
	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "error interno", httpResponseMessage(t, response))
	assert.NotContains(t, response.Body.String(), "SQLSTATE")
}

// FailingDB no-leak pin (D12): el SELECT del guard falla con Postgres real y
// el endpoint responde 500 genérico sin filtrar el error inyectado.
func TestLeaderboardController_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool {
		return op == "select"
	})
	realSvc := services.NewLeaderboardService(daos.NewLeaderboardDao(failing))
	router := setupLeaderboardRouter(t, realSvc, 7)

	response := doLeaderboardGet(t, router, "/api/v1/teams/2/leaderboard?metric=distance&period=all")

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "error interno", httpResponseMessage(t, response))
	assert.NotContains(t, response.Body.String(), "inyectado", "sin leak del error de DB de test")
	assert.NotContains(t, response.Body.String(), "FailingDB")
}
