package controllers

// Leaderboard por team (gamificacion-core D7, E3).

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	// Los DTOs del leaderboard solo aparecen en anotaciones swagger: el
	// import en blanco los registra para el parser (patrón join_request).
	_ "simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// LeaderboardController define el handler HTTP del leaderboard de un equipo.
type LeaderboardController interface {
	GetLeaderboard(c *gin.Context)
}

type leaderboardController struct {
	leaderboardService services.LeaderboardServiceInterface
}

func NewLeaderboardController(leaderboardService services.LeaderboardServiceInterface) LeaderboardController {
	return &leaderboardController{leaderboardService: leaderboardService}
}

// GetLeaderboard godoc
// @Summary      Leaderboard del equipo
// @Description  Ranking del team para una métrica (distance, duration, pace, peak_speed, sessions_count, attendance_count) y un período (week, month, year, custom, all), con top 5, viewer_rank y record histórico (solo period=all). Accessible para miembros de algún grupo del team o el owner; los corredores puntúan, el owner no genera fila propia. custom exige from y to (YYYY-MM-DD, from <= to); los demás períodos se derivan de hoy en TZ America/Argentina/Cordoba.
// @Tags         gamification
// @Produce      json
// @Param        id      path  int     true  "ID del equipo"
// @Param        metric  query  string  true  "distance | duration | pace | peak_speed | sessions_count | attendance_count"
// @Param        period  query  string  true  "week | month | year | custom | all"
// @Param        from    query  string  false  "Desde (YYYY-MM-DD, obligatorio en custom)"
// @Param        to      query  string  false  "Hasta (YYYY-MM-DD, obligatorio en custom)"
// @Success      200  {object}  gamification.LeaderboardResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/leaderboard [get]
func (lc *leaderboardController) GetLeaderboard(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	teamID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	var from, to *string
	if raw := c.Query("from"); raw != "" {
		from = &raw
	}
	if raw := c.Query("to"); raw != "" {
		to = &raw
	}

	resp, err := lc.leaderboardService.GetLeaderboard(c, authUserID, teamID, c.Query("metric"), c.Query("period"), from, to)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		message := "error interno"
		switch {
		case errors.Is(err, services.ErrLeaderboardInvalid):
			statusCode = http.StatusBadRequest
			code = "Bad request"
			message = err.Error()
		case errors.Is(err, services.ErrLeaderboardForbidden):
			statusCode = http.StatusForbidden
			code = "Forbidden"
			message = err.Error()
		}
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    message,
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}
