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

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/teambio"
	"simple-arq-golang/cmd/api/services"
)

type mockTeamSubscriptionService struct {
	getTeamSubscriptionFn  func(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error)
	getTeamSubscriptionsFn func(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error)
}

func (m *mockTeamSubscriptionService) GetTeamSubscription(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error) {
	if m.getTeamSubscriptionFn != nil {
		return m.getTeamSubscriptionFn(ctx, userID, teamID)
	}
	return nil, nil
}

func (m *mockTeamSubscriptionService) GetTeamSubscriptions(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error) {
	if m.getTeamSubscriptionsFn != nil {
		return m.getTeamSubscriptionsFn(ctx, callerID, teamID)
	}
	return nil, nil
}

func TestTeamSubscriptionController_GetTeamSubscription_Success(t *testing.T) {
	mockSvc := &mockTeamSubscriptionService{
		getTeamSubscriptionFn: func(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error) {
			assert.Equal(t, int64(1), userID)
			assert.Equal(t, int64(99), teamID)
			return &teambio.TeamSubscriptionResponse{
				Team:       teambio.TeamInfo{ID: teamID, Name: "Equipo X", MembershipFee: 1500},
				Membership: teambio.MembershipInfo{SubscriptionStatus: "active", InitAmount: 1500, PaidInstallments: 1},
				HasDebt:    false,
			}, nil
		},
	}

	controller := NewTeamSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users/1/teams/99/subscription", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "team_id", Value: "99"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscription(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result teambio.TeamSubscriptionResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(99), result.Team.ID)
	assert.Equal(t, "active", result.Membership.SubscriptionStatus)
}

func TestTeamSubscriptionController_GetTeamSubscription_Unauthorized(t *testing.T) {
	controller := NewTeamSubscriptionController(&mockTeamSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users/1/teams/99/subscription", nil)

	controller.GetTeamSubscription(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscription_MissingTeamID(t *testing.T) {
	controller := NewTeamSubscriptionController(&mockTeamSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users/1/subscription", nil)
	setAuthUserID(c, 1)

	controller.GetTeamSubscription(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscription_InvalidTeamID(t *testing.T) {
	controller := NewTeamSubscriptionController(&mockTeamSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users/1/teams/abc/subscription", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "team_id", Value: "abc"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscription(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscription_NotFound(t *testing.T) {
	mockSvc := &mockTeamSubscriptionService{
		getTeamSubscriptionFn: func(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users/1/teams/99/subscription", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}, {Key: "team_id", Value: "99"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscription(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscriptions_Success(t *testing.T) {
	mockSvc := &mockTeamSubscriptionService{
		getTeamSubscriptionsFn: func(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error) {
			assert.Equal(t, int64(1), callerID)
			assert.Equal(t, int64(99), teamID)
			return &teambio.TeamSubscriptionsResponse{
				TeamID:        teamID,
				MembershipFee: 1500,
				Members: []teambio.TeamMemberSubscriptionInfo{
					{UserID: 2, Status: "active", SubscriptionStatus: "active", HasDebt: false, InitAmount: 1500, PaidInstallments: 1},
				},
			}, nil
		},
	}

	controller := NewTeamSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/teams/99/subscriptions", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscriptions(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result teambio.TeamSubscriptionsResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	assert.Equal(t, int64(99), result.TeamID)
	require.Len(t, result.Members, 1)
	assert.Equal(t, int64(2), result.Members[0].UserID)
}

func TestTeamSubscriptionController_GetTeamSubscriptions_Forbidden(t *testing.T) {
	mockSvc := &mockTeamSubscriptionService{
		getTeamSubscriptionsFn: func(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error) {
			return nil, services.ErrTeamSubscriptionForbidden
		},
	}

	controller := NewTeamSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/teams/99/subscriptions", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscriptions(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
	var apiErr apierror.APIError
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &apiErr))
	assert.Equal(t, "Forbidden", apiErr.Code)
	assert.Equal(t, "no autorizado", apiErr.Message)
}

func TestTeamSubscriptionController_GetTeamSubscriptions_TeamNotFound(t *testing.T) {
	mockSvc := &mockTeamSubscriptionService{
		getTeamSubscriptionsFn: func(ctx *gin.Context, callerID, teamID int64) (*teambio.TeamSubscriptionsResponse, error) {
			return nil, errors.New("equipo no encontrado")
		},
	}

	controller := NewTeamSubscriptionController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/teams/99/subscriptions", nil)
	c.Params = []gin.Param{{Key: "id", Value: "99"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscriptions(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscriptions_InvalidID(t *testing.T) {
	controller := NewTeamSubscriptionController(&mockTeamSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/teams/abc/subscriptions", nil)
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}
	setAuthUserID(c, 1)

	controller.GetTeamSubscriptions(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscriptions_MissingID(t *testing.T) {
	controller := NewTeamSubscriptionController(&mockTeamSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/teams/subscriptions", nil)
	setAuthUserID(c, 1)

	controller.GetTeamSubscriptions(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamSubscriptionController_GetTeamSubscriptions_Unauthorized(t *testing.T) {
	controller := NewTeamSubscriptionController(&mockTeamSubscriptionService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/teams/99/subscriptions", nil)

	controller.GetTeamSubscriptions(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}
