package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockStreakService struct {
	getStreaksFn func(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error)
	calls        int
}

func (m *mockStreakService) GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
	m.calls++
	if m.getStreaksFn != nil {
		return m.getStreaksFn(ctx, targetUserID, days)
	}
	return nil, nil
}

func (m *mockStreakService) ComputeCurrentStreaks(ctx *gin.Context, targetUserID int64) (int, int, error) {
	return 0, 0, nil
}

func streakFixture() *gamification.StreaksResponse {
	return &gamification.StreaksResponse{
		Strict:   gamification.StreakSummary{Current: 3, Best: 9},
		Flexible: gamification.StreakSummary{Current: 4, Best: 11},
		Days: []gamification.StreakDay{
			{Date: "2026-10-09", StrictStatus: "completed", FlexibleStatus: "completed"},
			{Date: "2026-10-12", StrictStatus: "failed", FlexibleStatus: "failed"},
			{Date: "2026-10-13", StrictStatus: "pending", FlexibleStatus: "pending"},
		},
		HasUnseen: true,
	}
}

// doStreakGet arma el router con la ruta real (así el parse del :id es el
// producción) y ejecuta el GET. La identidad autenticada la inyecta el
// middleware: acá se setea el mismo key del contexto con un hook propio.
func doStreakGet(t *testing.T, svc services.StreakServiceInterface, authUserID int64, path, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if authUserID > 0 {
			c.Set(utils.AuthUserIDKey, authUserID)
		}
		c.Next()
	})
	router.GET("/api/v1/users/:id/streaks", NewStreakController(svc).GetStreaks)

	response := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, path, nil)
	require.NoError(t, err)
	if rawQuery != "" {
		req.URL.RawQuery = rawQuery
	}
	router.ServeHTTP(response, req)
	return response
}

func TestStreakController_Unauthorized(t *testing.T) {
	response := doStreakGet(t, &mockStreakService{}, 0, "/api/v1/users/7/streaks", "")
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestStreakController_Forbidden_NotSelf_NoDaoTouch(t *testing.T) {
	mock := &mockStreakService{}
	response := doStreakGet(t, mock, 7, "/api/v1/users/99/streaks", "")
	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Equal(t, 0, mock.calls, "el guard self-only debe cortar antes del servicio")

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["message"], "no podés consultar los datos de otro usuario")
}

func TestStreakController_BadRequest_ID(t *testing.T) {
	mock := &mockStreakService{}
	response := doStreakGet(t, mock, 7, "/api/v1/users/0/streaks", "")
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, 0, mock.calls)
}

func TestStreakController_BadRequest_DaysYoRango(t *testing.T) {
	cases := []string{"days=0", "days=-5", "days=366", "days=abc"}
	for _, raw := range cases {
		mock := &mockStreakService{}
		response := doStreakGet(t, mock, 7, "/api/v1/users/7/streaks", raw)
		require.Equal(t, http.StatusBadRequest, response.Code, raw)
		assert.Equal(t, 0, mock.calls, raw)
	}
}

func TestStreakController_Success_DefaultDaysYShape(t *testing.T) {
	var gotDays int
	mock := &mockStreakService{getStreaksFn: func(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
		gotDays = days
		return streakFixture(), nil
	}}
	response := doStreakGet(t, mock, 7, "/api/v1/users/7/streaks", "")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 42, gotDays, "days default 42")

	var resp gamification.StreaksResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Equal(t, 3, resp.Strict.Current)
	assert.Equal(t, 9, resp.Strict.Best)
	assert.Equal(t, 4, resp.Flexible.Current)
	assert.Equal(t, 11, resp.Flexible.Best)
	assert.True(t, resp.HasUnseen)
	require.Len(t, resp.Days, 3)
	assert.Equal(t, "2026-10-09", resp.Days[0].Date)
	assert.Equal(t, "completed", resp.Days[0].StrictStatus)
	assert.Equal(t, "pending", resp.Days[2].FlexibleStatus)
}

func TestStreakController_Success_BoundariesDays(t *testing.T) {
	cases := map[string]int{"days=1": 1, "days=365": 365}
	for raw, want := range cases {
		var gotDays int
		mock := &mockStreakService{getStreaksFn: func(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
			gotDays = days
			return streakFixture(), nil
		}}
		response := doStreakGet(t, mock, 7, "/api/v1/users/7/streaks", raw)

		require.Equal(t, http.StatusOK, response.Code, raw)
		assert.Equal(t, want, gotDays, raw)
	}
}

func TestStreakController_InternalError_NoLeak(t *testing.T) {
	mock := &mockStreakService{getStreaksFn: func(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
		return nil, errors.New("SQLSTATE 23505: detalle crudo del driver")
	}}
	response := doStreakGet(t, mock, 7, "/api/v1/users/7/streaks", "")
	require.Equal(t, http.StatusInternalServerError, response.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
	assert.Equal(t, "error interno", errResp["message"])
	assert.NotContains(t, response.Body.String(), "SQLSTATE", "sin leak del driver")
	assert.NotContains(t, response.Body.String(), "23505")
}
