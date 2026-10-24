package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// StreakController define los handlers HTTP del módulo de rachas.
type StreakController interface {
	GetStreaks(c *gin.Context)
}

type streakController struct {
	streakService services.StreakServiceInterface
}

func NewStreakController(streakService services.StreakServiceInterface) StreakController {
	return &streakController{streakService: streakService}
}

// GetStreaks godoc
// @Summary      Rachas del usuario
// @Description  Devuelve las rachas estricta (A) y flexible (B) del propio usuario autenticado, con el array window de días (default 42, tope 365) y has_unseen de la sección streaks. El día de hoy nunca corta una racha: la ventana de cálculo termina en ayer (TZ America/Argentina/Cordoba).
// @Tags         gamification
// @Produce      json
// @Param        id    path  int  true  "ID del usuario (debe ser el autenticado)"
// @Param        days  query  int  false  "Tamaño de ventana (default 42, entre 1 y 365)"
// @Success      200  {object}  gamification.StreaksResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users/{id}/streaks [get]
func (sc *streakController) GetStreaks(c *gin.Context) {
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

	days, err := parseStreaksDays(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	resp, err := sc.streakService.GetStreaks(c, targetID, days)
	if err != nil {
		// Error interno de DAO: 500 genérico sin filtrar detalles del driver.
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    "error interno",
		})
		return
	}

	c.JSON(http.StatusOK, toStreaksResponse(resp))
}

// toStreaksResponse mantiene visible el type del response para el swagger: la
// shape es exactamente la del service, sin cambios.
func toStreaksResponse(resp *gamification.StreaksResponse) *gamification.StreaksResponse {
	return resp
}

// parseStreaksDays valida el query days: default 42, tope 365, fuera de
// rango (<1 o >365) → error (400).
func parseStreaksDays(c *gin.Context) (int, error) {
	const defaultStreaksDays = 42
	raw := c.Query("days")
	if raw == "" {
		return defaultStreaksDays, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 || days > 365 {
		return 0, fmt.Errorf("days debe ser un número entero entre 1 y 365")
	}
	return days, nil
}
