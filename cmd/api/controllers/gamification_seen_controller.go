package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// GamificationSeenController define el handler del escape del punto rojo
// (gamificacion-core D3). Self-only: solo se marcan las novedades propias.
type GamificationSeenController interface {
	See(c *gin.Context)
}

type gamificationSeenController struct {
	seenService services.GamificationSeenServiceInterface
}

func NewGamificationSeenController(seenService services.GamificationSeenServiceInterface) GamificationSeenController {
	return &gamificationSeenController{seenService: seenService}
}

// See godoc
// @Summary      Marcar novedades de gamificación como vistas
// @Description  Borra las filas unseen del usuario autenticado para la sección indicada. section es streaks|achievements|competitions; team_id solo se admite con competitions (marca vistas solo filas de ese equipo; sin team_id limpia todas, incluidas las por-equipo). Cero filas es éxito no-op.
// @Tags         gamification
// @Accept       json
// @Produce      json
// @Param        id    path  int                     true  "ID del usuario (debe ser el autenticado)"
// @Param        body  body  gamification.SeenRequest  true  "Sección y team_id opcional (solo competitions)"
// @Success      200  {object}  gamification.SeenResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users/{id}/gamification/seen [post]
func (sc *gamificationSeenController) See(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	targetID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	if respondHistoryTargetError(c, targetID, authUserID) {
		return
	}

	var req gamification.SeenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	if err := sc.seenService.See(c, targetID, req.Section, req.TeamID); err != nil {
		switch {
		case errors.Is(err, services.ErrGamificationInvalidSection):
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    err.Error(),
			})
		case errors.Is(err, services.ErrGamificationTeamIDInvalid):
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    err.Error(),
			})
		default:
			// Error interno de DAO: 500 genérico sin filtrar detalles del driver.
			c.JSON(http.StatusInternalServerError, apierror.APIError{
				StatusCode: http.StatusInternalServerError,
				Code:       "Internal Server Error",
				Message:    "error interno",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gamification.SeenResponse{Message: "novedades marcadas como vistas"})
}
