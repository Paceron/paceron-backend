package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/teambio"
)

type mockTeamSubscriptionService struct {
	getTeamSubscriptionFn func(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error)
}

func (m *mockTeamSubscriptionService) GetTeamSubscription(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error) {
	if m.getTeamSubscriptionFn != nil {
		return m.getTeamSubscriptionFn(ctx, userID, teamID)
	}
	return nil, nil
}

func TestTeamSubscriptionController_GetTeamSubscription_Success(t *testing.T) {
	mockSvc := &mockTeamSubscriptionService{
		getTeamSubscriptionFn: func(ctx *gin.Context, userID, teamID int64) (*teambio.TeamSubscriptionResponse, error) {
			assert.Equal(t, int64(1), userID)
			assert.Equal(t, int64(99), teamID)
			return &teambio.TeamSubscriptionResponse{
				Team: teambio.TeamInfo{ID: teamID, Name: "Equipo X", MembershipFee: 1500},
				Membership: teambio.MembershipInfo{SubscriptionStatus: "active", InitAmount: 1500, PaidInstallments: 1},
				HasDebt:     false,
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