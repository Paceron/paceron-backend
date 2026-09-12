package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/teamconfiguration"
	"simple-arq-golang/cmd/api/services"
)

type mockTeamConfigurationService struct {
	getTeamConfigurationFn func(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error)
}

func (m *mockTeamConfigurationService) GetTeamConfiguration(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error) {
	if m.getTeamConfigurationFn != nil {
		return m.getTeamConfigurationFn(ctx, userID, teamID)
	}
	return nil, nil
}

func setupTeamConfigurationRequest(response *httptest.ResponseRecorder, url string, authUserID int64) *gin.Context {
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, url, nil)
	if authUserID > 0 {
		setAuthUserID(c, authUserID)
	}
	return c
}

func TestTeamConfigurationController_Success(t *testing.T) {
	mockSvc := &mockTeamConfigurationService{
		getTeamConfigurationFn: func(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error) {
			return &teamconfiguration.TeamConfiguration{MaxMembers: 50, MinimumFee: 20000}, nil
		},
	}
	controller := NewTeamConfigurationController(mockSvc)
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?user_id=7&team_id=3", 7)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var body map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &body)
	assert.Equal(t, float64(50), body["max_members"])
	assert.Equal(t, float64(20000), body["minimum_fee"])
}

func TestTeamConfigurationController_MissingUserID(t *testing.T) {
	controller := NewTeamConfigurationController(&mockTeamConfigurationService{})
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?team_id=3", 1)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamConfigurationController_MissingTeamID(t *testing.T) {
	controller := NewTeamConfigurationController(&mockTeamConfigurationService{})
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?user_id=1", 1)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamConfigurationController_InvalidUserID(t *testing.T) {
	controller := NewTeamConfigurationController(&mockTeamConfigurationService{})
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?user_id=abc&team_id=3", 1)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestTeamConfigurationController_SelfOnlyForbidden(t *testing.T) {
	controller := NewTeamConfigurationController(&mockTeamConfigurationService{})
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?user_id=7&team_id=3", 99)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTeamConfigurationController_TeamNotFound(t *testing.T) {
	mockSvc := &mockTeamConfigurationService{
		getTeamConfigurationFn: func(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error) {
			return nil, services.ErrTeamNotFound
		},
	}
	controller := NewTeamConfigurationController(mockSvc)
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?user_id=1&team_id=999", 1)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestTeamConfigurationController_NotOwner(t *testing.T) {
	mockSvc := &mockTeamConfigurationService{
		getTeamConfigurationFn: func(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error) {
			return nil, services.ErrTeamNotOwner
		},
	}
	controller := NewTeamConfigurationController(mockSvc)
	response := httptest.NewRecorder()
	c := setupTeamConfigurationRequest(response, "/api/v1/team-configuration?user_id=1&team_id=3", 1)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}
