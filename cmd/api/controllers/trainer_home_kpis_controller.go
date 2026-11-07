package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type TrainerHomeKpisController interface {
	Get(c *gin.Context)
}

type trainerHomeKpisController struct {
	service services.TrainerHomeKpisServiceInterface
}

func NewTrainerHomeKpisController(service services.TrainerHomeKpisServiceInterface) TrainerHomeKpisController {
	return &trainerHomeKpisController{service: service}
}

// Get godoc
// @Summary      KPIs del Home del entrenador
// @Description  Resumen del período del entrenador autenticado: sesiones programadas/realizadas, presencial, cobranzas netas y cuotas pendientes; además de equipos y corredores activos actuales. La identidad se toma del token.
// @Tags         dashboard
// @Produce      json
// @Param        period          query     string  false  "Período calendario (default month)"  Enums(month, week, day)
// @Param        reference_date  query     string  false  "Fecha de referencia YYYY-MM-DD (default fecha del servidor)"
// @Success      200  {object}  dashboard.TrainerHomeKpisResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/dashboard/trainer-home-kpis [get]
func (tc *trainerHomeKpisController) Get(c *gin.Context) {
	trainerID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondDashboardUnauthorized(c)
		return
	}
	query := services.TrainerHomeKpisQuery{Period: c.Query("period"), ReferenceDate: c.Query("reference_date")}
	response, err := tc.service.Get(c, trainerID, query)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

var _ *dashboard.TrainerHomeKpisResponse
var _ *apierror.APIError
