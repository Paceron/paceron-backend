package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/teamconfiguration"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// TeamConfigurationControllerInterface expone la configuración de equipos del
// entrenador según su tier.
type TeamConfigurationControllerInterface interface {
	GetTeamConfiguration(c *gin.Context)
}

type teamConfigurationController struct {
	teamConfigurationService services.TeamConfigurationServiceInterface
}

func NewTeamConfigurationController(teamConfigurationService services.TeamConfigurationServiceInterface) TeamConfigurationControllerInterface {
	return &teamConfigurationController{
		teamConfigurationService: teamConfigurationService,
	}
}

// GetTeamConfiguration godoc
// @Summary      Get team configuration by trainer tier
// @Description  Returns the max members and minimum membership fee allowed for the trainer's tier. Identity comes from the access token.
// @Tags         teams
// @Produce      json
// @Success      200     {object}  teamconfiguration.TeamConfiguration
// @Failure      500     {object}  apierror.APIError
// @Router       /api/v1/team-configuration [get]
func (tcc *teamConfigurationController) GetTeamConfiguration(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "usuario no autenticado",
		})
		return
	}

	var response *teamconfiguration.TeamConfiguration
	var err error
	response, err = tcc.teamConfigurationService.GetTeamConfiguration(c, authUserID)
	if err != nil {
		statusCode, code := mapTeamConfigurationError(err)
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// mapTeamConfigurationError traduce los errores del service a status/code.
func mapTeamConfigurationError(err error) (statusCode int, code string) {
	return http.StatusInternalServerError, "Internal Server Error"
}
