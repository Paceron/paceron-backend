package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type mockActivityComparisonService struct {
	getFn  func(ctx *gin.Context, callerID, sessionInstanceID int64, athleteUserID *int64) (*comparison.SessionComparisonResponse, error)
	listFn func(ctx *gin.Context, callerID int64, query comparison.ListQuery) (*comparison.ComparisonListResponse, error)
}

func (m *mockActivityComparisonService) GetSessionComparison(ctx *gin.Context, callerID, sessionInstanceID int64, athleteUserID *int64) (*comparison.SessionComparisonResponse, error) {
	return m.getFn(ctx, callerID, sessionInstanceID, athleteUserID)
}

func (m *mockActivityComparisonService) ListComparisons(ctx *gin.Context, callerID int64, query comparison.ListQuery) (*comparison.ComparisonListResponse, error) {
	return m.listFn(ctx, callerID, query)
}

func comparisonRequest(target, sessionParam string, authenticated bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	if sessionParam != "" {
		c.Params = gin.Params{{Key: "sessionInstanceId", Value: sessionParam}}
	}
	if authenticated {
		c.Set(utils.AuthUserIDKey, int64(19))
	}
	return c, w
}

// --- GetSessionComparison ---

func TestActivityComparisonController_GetSessionComparison_Success(t *testing.T) {
	svc := &mockActivityComparisonService{getFn: func(_ *gin.Context, callerID, sessionInstanceID int64, athleteUserID *int64) (*comparison.SessionComparisonResponse, error) {
		assert.Equal(t, int64(19), callerID)
		assert.Equal(t, int64(455), sessionInstanceID)
		if assert.NotNil(t, athleteUserID) {
			assert.Equal(t, int64(21), *athleteUserID)
		}
		return &comparison.SessionComparisonResponse{CompliancePct: 50.1}, nil
	}}
	c, w := comparisonRequest("/api/v1/activities/comparison/455?athlete_user_id=21", "455", true)

	NewActivityComparisonController(svc).GetSessionComparison(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"compliance_pct":50.1`)
}

func TestActivityComparisonController_GetSessionComparison_BadRequestAndAuth(t *testing.T) {
	svc := &mockActivityComparisonService{getFn: func(*gin.Context, int64, int64, *int64) (*comparison.SessionComparisonResponse, error) {
		t.Fatal("no debería llamar al service")
		return nil, nil
	}}
	tests := []struct {
		name, target, param string
		auth                bool
		want                int
	}{
		{"sin autenticar", "/api/v1/activities/comparison/455", "455", false, http.StatusUnauthorized},
		{"id no numérico", "/api/v1/activities/comparison/abc", "abc", true, http.StatusBadRequest},
		{"id cero", "/api/v1/activities/comparison/0", "0", true, http.StatusBadRequest},
		{"athlete inválido", "/api/v1/activities/comparison/455?athlete_user_id=x", "455", true, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := comparisonRequest(tt.target, tt.param, tt.auth)
			NewActivityComparisonController(svc).GetSessionComparison(c)
			assert.Equal(t, tt.want, w.Code)
		})
	}
}

func TestActivityComparisonController_ServiceErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		want     int
		wantBody string
	}{
		{"inválido", fmt.Errorf("%w: kind desconocido", services.ErrComparisonInvalidQuery), http.StatusBadRequest, "INVALID_QUERY"},
		{"prohibido", services.ErrComparisonForbidden, http.StatusForbidden, "Forbidden"},
		{"no encontrado", services.ErrComparisonNotFound, http.StatusNotFound, "Not Found"},
		{"error de DB", errors.New(`pq: relation "x" does not exist`), http.StatusInternalServerError, "error interno"},
	}
	for _, tt := range tests {
		t.Run("detalle "+tt.name, func(t *testing.T) {
			svc := &mockActivityComparisonService{getFn: func(*gin.Context, int64, int64, *int64) (*comparison.SessionComparisonResponse, error) {
				return nil, tt.err
			}}
			c, w := comparisonRequest("/api/v1/activities/comparison/455", "455", true)
			NewActivityComparisonController(svc).GetSessionComparison(c)
			assert.Equal(t, tt.want, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
			assert.NotContains(t, w.Body.String(), "relation")
		})
		t.Run("lista "+tt.name, func(t *testing.T) {
			svc := &mockActivityComparisonService{listFn: func(*gin.Context, int64, comparison.ListQuery) (*comparison.ComparisonListResponse, error) {
				return nil, tt.err
			}}
			c, w := comparisonRequest("/api/v1/activities/comparison?date_from=2026-10-01&date_to=2026-10-13", "", true)
			NewActivityComparisonController(svc).ListComparisons(c)
			assert.Equal(t, tt.want, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}

// --- ListComparisons ---

func TestActivityComparisonController_ListComparisons_PassesQuery(t *testing.T) {
	svc := &mockActivityComparisonService{listFn: func(_ *gin.Context, callerID int64, query comparison.ListQuery) (*comparison.ComparisonListResponse, error) {
		assert.Equal(t, int64(19), callerID)
		assert.Equal(t, "2026-10-01", query.DateFrom)
		assert.Equal(t, "2026-10-13", query.DateTo)
		assert.Equal(t, "liviano", query.Kind)
		assert.Equal(t, int64(16), *query.TeamID)
		assert.Equal(t, int64(17), *query.GroupID)
		assert.Equal(t, int64(21), *query.AthleteUserID)
		return &comparison.ComparisonListResponse{Sessions: []comparison.SessionComparisonSummary{}, Summary: comparison.RangeSummary{SessionsPlanned: 0}}, nil
	}}
	c, w := comparisonRequest("/api/v1/activities/comparison?date_from=2026-10-01&date_to=2026-10-13&kind=liviano&team_id=16&group_id=17&athlete_user_id=21", "", true)

	NewActivityComparisonController(svc).ListComparisons(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"sessions":[]`)
}

func TestActivityComparisonController_ListComparisons_DefaultsAreNil(t *testing.T) {
	svc := &mockActivityComparisonService{listFn: func(_ *gin.Context, _ int64, query comparison.ListQuery) (*comparison.ComparisonListResponse, error) {
		assert.Nil(t, query.TeamID)
		assert.Nil(t, query.GroupID)
		assert.Nil(t, query.AthleteUserID)
		assert.Equal(t, "", query.Kind)
		return &comparison.ComparisonListResponse{}, nil
	}}
	c, w := comparisonRequest("/api/v1/activities/comparison?date_from=2026-10-01&date_to=2026-10-13", "", true)

	NewActivityComparisonController(svc).ListComparisons(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestActivityComparisonController_ListComparisons_BadRequestAndAuth(t *testing.T) {
	svc := &mockActivityComparisonService{listFn: func(*gin.Context, int64, comparison.ListQuery) (*comparison.ComparisonListResponse, error) {
		t.Fatal("no debería llamar al service")
		return nil, nil
	}}
	for _, tt := range []struct {
		name, target string
		auth         bool
		want         int
	}{
		{"sin autenticar", "/api/v1/activities/comparison", false, http.StatusUnauthorized},
		{"team inválido", "/api/v1/activities/comparison?team_id=x", true, http.StatusBadRequest},
		{"group negativo", "/api/v1/activities/comparison?group_id=-1", true, http.StatusBadRequest},
		{"athlete inválido", "/api/v1/activities/comparison?athlete_user_id=0", true, http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, w := comparisonRequest(tt.target, "", tt.auth)
			NewActivityComparisonController(svc).ListComparisons(c)
			assert.Equal(t, tt.want, w.Code)
		})
	}
}
