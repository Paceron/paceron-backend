package controllers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
)

type trainerHomeKpisServiceStub struct {
	trainerID int64
	query     services.TrainerHomeKpisQuery
	response  *dashboard.TrainerHomeKpisResponse
	err       error
}

func (s *trainerHomeKpisServiceStub) Get(_ *gin.Context, trainerID int64, query services.TrainerHomeKpisQuery) (*dashboard.TrainerHomeKpisResponse, error) {
	s.trainerID, s.query = trainerID, query
	return s.response, s.err
}

func TestTrainerHomeKpisController_GetUsesAuthenticatedUserAndQuery(t *testing.T) {
	service := &trainerHomeKpisServiceStub{
		response: &dashboard.TrainerHomeKpisResponse{
			Period: "week", Sessions: dashboard.TrainerHomeSessions{Scheduled: 7, Realized: 4},
			Structure:   dashboard.TrainerHomeStructure{ActiveTeams: 2, ActiveRunners: 10},
			Collections: dashboard.TrainerHomeCollections{CurrencyID: "ARS", NetCollected: 1200},
		},
	}
	ctx, recorder := dashboardTestContext(t, http.MethodGet,
		"/api/v1/dashboard/trainer-home-kpis?period=week&reference_date=2026-11-05", "", true)

	NewTrainerHomeKpisController(service).Get(ctx)

	assert.EqualValues(t, 42, service.trainerID)
	assert.Equal(t, services.TrainerHomeKpisQuery{Period: "week", ReferenceDate: "2026-11-05"}, service.query)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"presencial_scheduled":0`)
	assert.Contains(t, recorder.Body.String(), `"active_runners":10`)
}

func TestTrainerHomeKpisController_GetRequiresAuthentication(t *testing.T) {
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/trainer-home-kpis", "", false)
	NewTrainerHomeKpisController(&trainerHomeKpisServiceStub{}).Get(ctx)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestTrainerHomeKpisController_GetMapsServiceError(t *testing.T) {
	service := &trainerHomeKpisServiceStub{err: errors.New("database unavailable")}
	ctx, recorder := dashboardTestContext(t, http.MethodGet, "/api/v1/dashboard/trainer-home-kpis", "", true)
	NewTrainerHomeKpisController(service).Get(ctx)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
