package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/teamconfiguration"
)

type mockTeamConfigurationService struct {
	getTeamConfigurationFn func(ctx *gin.Context, userID int64) (*teamconfiguration.TeamConfiguration, error)
}

func (m *mockTeamConfigurationService) GetTeamConfiguration(ctx *gin.Context, userID int64) (*teamconfiguration.TeamConfiguration, error) {
	if m.getTeamConfigurationFn != nil {
		return m.getTeamConfigurationFn(ctx, userID)
	}
	return nil, nil
}

func TestTeamConfigurationController_Success(t *testing.T) {
	capturedUserID := int64(0)
	mockSvc := &mockTeamConfigurationService{
		getTeamConfigurationFn: func(ctx *gin.Context, userID int64) (*teamconfiguration.TeamConfiguration, error) {
			capturedUserID = userID
			return &teamconfiguration.TeamConfiguration{MaxMembers: 50, MinimumFee: 20000}, nil
		},
	}
	controller := NewTeamConfigurationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/team-configuration", nil)
	setAuthUserID(c, 7)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, int64(7), capturedUserID)
	var body map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &body)
	assert.Equal(t, float64(50), body["max_members"])
	assert.Equal(t, float64(20000), body["minimum_fee"])
}

func TestTeamConfigurationController_NoAuthUserIDReturnsUnauthorized(t *testing.T) {
	mockSvc := &mockTeamConfigurationService{
		getTeamConfigurationFn: func(ctx *gin.Context, userID int64) (*teamconfiguration.TeamConfiguration, error) {
			return &teamconfiguration.TeamConfiguration{MaxMembers: 10, MinimumFee: 20000}, nil
		},
	}
	controller := NewTeamConfigurationController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/team-configuration", nil)

	controller.GetTeamConfiguration(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}
