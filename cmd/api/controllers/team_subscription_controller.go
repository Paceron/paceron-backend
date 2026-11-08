package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/teambio"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// TeamSubscriptionControllerInterface define el endpoint de suscripción de equipo.
type TeamSubscriptionControllerInterface interface {
	GetTeamSubscription(c *gin.Context)
	GetTeamSubscriptions(c *gin.Context)
}

type teamSubscriptionController struct {
	service services.TeamSubscriptionServiceInterface
}

func NewTeamSubscriptionController(svc services.TeamSubscriptionServiceInterface) TeamSubscriptionControllerInterface {
	return &teamSubscriptionController{service: svc}
}

// GetTeamSubscription godoc
// @Summary      Get team subscription status
// @Description  Returns the subscription status for a user in a team (membership, next installment, debt status, checkout data).
// @Tags         team-subscriptions
// @Accept       json
// @Produce      json
// @Param        id       path      int  true  "User ID"
// @Param        team_id  path      int  true  "Team ID"
// @Success      200      {object}  teambio.TeamSubscriptionResponse
// @Failure      401      {object}  apierror.APIError
// @Failure      404      {object}  apierror.APIError
// @Failure      500      {object}  apierror.APIError
// @Router       /api/v1/users/{id}/teams/{team_id}/subscription [get]
func (c *teamSubscriptionController) GetTeamSubscription(ctx *gin.Context) {
	userID, ok := utils.GetAuthUserID(ctx)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "no autenticado",
		})
		return
	}

	// team_id desde URL
	teamIDStr := ctx.Param("team_id")
	if teamIDStr == "" {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id requerido",
		})
		return
	}

	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id inválido",
		})
		return
	}

	var resp *teambio.TeamSubscriptionResponse
	resp, err = c.service.GetTeamSubscription(ctx, userID, teamID)
	if err != nil {
		statusCode, code := mapTeamSubscriptionError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetTeamSubscriptions godoc
// @Summary      Get team subscriptions batch
// @Description  Returns the subscription status for all active runners of a team. Only the team owner or an active team trainer can access it.
// @Tags         team-subscriptions
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Team ID"
// @Success      200  {object}  teambio.TeamSubscriptionsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/subscriptions [get]
func (c *teamSubscriptionController) GetTeamSubscriptions(ctx *gin.Context) {
	callerID, ok := utils.GetAuthUserID(ctx)
	if !ok || callerID == 0 {
		ctx.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "no autenticado",
		})
		return
	}

	teamIDStr := ctx.Param("id")
	if teamIDStr == "" {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id requerido",
		})
		return
	}

	teamID, err := strconv.ParseInt(teamIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id inválido",
		})
		return
	}

	resp, err := c.service.GetTeamSubscriptions(ctx, callerID, teamID)
	if err != nil {
		statusCode, code := mapTeamSubscriptionError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// mapTeamSubscriptionError traduce los errores del servicio a status/code de apierror.
func mapTeamSubscriptionError(err error) (int, string) {
	errMsg := err.Error()

	switch {
	case errors.Is(err, services.ErrTeamSubscriptionForbidden):
		return http.StatusForbidden, "Forbidden"
	case errMsg == "equipo no encontrado":
		return http.StatusNotFound, "Not Found"
	case errMsg == "membresía no encontrada":
		return http.StatusNotFound, "Not Found"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}
