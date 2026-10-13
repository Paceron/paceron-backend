package controllers

// --- POST /users/{id}/gamification/seen (gamificacion-core D3, task 2.1).

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
)

type mockGamificationSeenService struct {
	seeFn       func(ctx *gin.Context, targetUserID int64, section string, teamID *int64) error
	calls       int
	gotSection  string
	gotTeamID   *int64
	gotTargetID int64
}

func (m *mockGamificationSeenService) See(ctx *gin.Context, targetUserID int64, section string, teamID *int64) error {
	m.calls++
	m.gotSection, m.gotTeamID, m.gotTargetID = section, teamID, targetUserID
	if m.seeFn != nil {
		return m.seeFn(ctx, targetUserID, section, teamID)
	}
	return nil
}

func doSeenPost(t *testing.T, svc services.GamificationSeenServiceInterface, authUserID int64, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if authUserID > 0 {
			c.Set(utils.AuthUserIDKey, authUserID)
		}
		c.Next()
	})
	router.POST("/api/v1/users/:id/gamification/seen", NewGamificationSeenController(svc).See)

	response := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, req)
	return response
}

func TestSeenController_Unauthorized(t *testing.T) {
	response := doSeenPost(t, &mockGamificationSeenService{}, 0, "/api/v1/users/7/gamification/seen", `{"section":"streaks"}`)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestSeenController_Forbidden_NotSelf_NoServiceCall(t *testing.T) {
	svc := &mockGamificationSeenService{}
	response := doSeenPost(t, svc, 7, "/api/v1/users/99/gamification/seen", `{"section":"streaks"}`)

	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Equal(t, 0, svc.calls, "el guard self-only corta antes del servicio")
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["message"], "no podés consultar los datos de otro usuario")
}

func TestSeenController_BadRequest_ID(t *testing.T) {
	svc := &mockGamificationSeenService{}
	response := doSeenPost(t, svc, 7, "/api/v1/users/0/gamification/seen", `{"section":"streaks"}`)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, 0, svc.calls)
}

func TestSeenController_BadJSON(t *testing.T) {
	svc := &mockGamificationSeenService{}
	response := doSeenPost(t, svc, 7, "/api/v1/users/7/gamification/seen", "{not json")
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, 0, svc.calls)
}

func TestSeenController_MapsService400s(t *testing.T) {
	cases := []struct {
		name     string
		serviceR error
		contains string
	}{
		{"section inválida", services.ErrGamificationInvalidSection, "section inválida"},
		{"team_id mal ubicado", services.ErrGamificationTeamIDInvalid, "team_id solo válido en competitions"},
	}
	for _, tc := range cases {
		svc := &mockGamificationSeenService{seeFn: func(ctx *gin.Context, targetUserID int64, section string, teamID *int64) error {
			return tc.serviceR
		}}
		response := doSeenPost(t, svc, 7, "/api/v1/users/7/gamification/seen", `{"section":"achievements"}`)
		require.Equal(t, http.StatusBadRequest, response.Code, tc.name)

		var errResp apierror.APIError
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
		assert.Contains(t, errResp.Message, tc.contains, tc.name)
	}
}

func TestSeenController_Success_PasaBody(t *testing.T) {
	svc := &mockGamificationSeenService{}
	response := doSeenPost(t, svc, 7, "/api/v1/users/7/gamification/seen", `{"section":"competitions","team_id":44}`)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, svc.calls)
	assert.Equal(t, "competitions", svc.gotSection)
	require.NotNil(t, svc.gotTeamID)
	assert.EqualValues(t, 44, *svc.gotTeamID)
	assert.Equal(t, int64(7), svc.gotTargetID)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "vistas")
}

func TestSeenController_InternalError_NoLeak(t *testing.T) {
	svc := &mockGamificationSeenService{seeFn: func(ctx *gin.Context, targetUserID int64, section string, teamID *int64) error {
		return errors.New("SQLSTATE 23505: detalle crudo del driver")
	}}
	response := doSeenPost(t, svc, 7, "/api/v1/users/7/gamification/seen", `{"section":"streaks"}`)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
	assert.Equal(t, "error interno", errResp["message"])
	assert.NotContains(t, response.Body.String(), "SQLSTATE", "sin leak del driver")
	assert.NotContains(t, response.Body.String(), "23505")
}

// FailingDB no-leak pin (task 2.5): DELETE reality check con Postgres real,
// el fallo del write sale como 500 genérico sin detalle del driver.
func TestSeenController_FailingDB_NoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool {
		return op == "delete"
	})
	svc := services.NewGamificationSeenService(daos.NewGamificationDao(failing))
	response := doSeenPost(t, svc, 7, "/api/v1/users/7/gamification/seen", `{"section":"streaks"}`)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "error interno", httpResponseMessage(t, response))
	assert.NotContains(t, response.Body.String(), "inyectado", "sin leak del error de DB de test")
	assert.NotContains(t, response.Body.String(), "FailingDB", "sin leak del error de DB de test")
}

func httpResponseMessage(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &errResp))
	msg, ok := errResp["message"].(string)
	require.True(t, ok)
	return msg
}
