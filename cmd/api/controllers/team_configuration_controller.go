package controllers

import (
	"errors"
	"net/http"
	"strconv"

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
// @Description  Returns the max members and minimum membership fee allowed for the trainer's tier, validated against a team they own.
// @Tags         teams
// @Produce      json
// @Param        user_id query int true "User ID (the trainer)"
// @Param        team_id query int true "Team ID"
// @Success      200     {object}  teamconfiguration.TeamConfiguration
// @Failure      400     {object}  apierror.APIError
// @Failure      403     {object}  apierror.APIError
// @Failure      404     {object}  apierror.APIError
// @Failure      500     {object}  apierror.APIError
// @Router       /api/v1/team-configuration [get]
func (tcc *teamConfigurationController) GetTeamConfiguration(c *gin.Context) {
	userID, ok := parseUserIDQuery(c)
	if !ok {
		return
	}
	teamID, ok := parseTeamIDQuery(c)
	if !ok {
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelfSubscription(c)
		return
	}

	var response *teamconfiguration.TeamConfiguration
	var err error
	response, err = tcc.teamConfigurationService.GetTeamConfiguration(c, userID, teamID)
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

func parseUserIDQuery(c *gin.Context) (int64, bool) {
	return parseRequiredIDQuery(c, "user_id")
}

func parseTeamIDQuery(c *gin.Context) (int64, bool) {
	return parseRequiredIDQuery(c, "team_id")
}

func parseRequiredIDQuery(c *gin.Context, param string) (int64, bool) {
	raw := c.Query(param)
	if raw == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "el parámetro " + param + " es requerido",
		})
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    param + " debe ser un número válido",
		})
		return 0, false
	}
	return id, true
}

// mapTeamConfigurationError traduce los errores del service a status/code.
func mapTeamConfigurationError(err error) (statusCode int, code string) {
	switch {
	case errors.Is(err, services.ErrTeamNotFound):
		return http.StatusNotFound, "Not Found"
	case errors.Is(err, services.ErrTeamNotOwner):
		return http.StatusForbidden, "Forbidden"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}
