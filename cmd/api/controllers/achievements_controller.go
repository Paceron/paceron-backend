package controllers

// Endpoints de logros y estampas (gamificacion-core D5, E2).

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// AchievementsController define los handlers HTTP del catálogo de logros y la
// vitrina (self-only, mismo guard self-only que rachas).
type AchievementsController interface {
	GetCatalog(c *gin.Context)
	GetUserAchievements(c *gin.Context)
	GetUserStamps(c *gin.Context)
}

type achievementsController struct {
	achievementsService services.AchievementsServiceInterface
}

func NewAchievementsController(achievementsService services.AchievementsServiceInterface) AchievementsController {
	return &achievementsController{achievementsService: achievementsService}
}

// GetCatalog godoc
// @Summary      Catálogo de logros
// @Description  Devuelve las filas del catálogo de logros (catalogo versionado en constants). Compartido: cualquier usuario autenticado puede consultarlo, sin guard de recurso.
// @Tags         gamification
// @Produce      json
// @Success      200  {object}  gamification.AchievementCatalogResponse
// @Failure      401  {object}  apierror.APIError
// @Router       /api/v1/achievements/catalog [get]
func (ac *achievementsController) GetCatalog(c *gin.Context) {
	if _, ok := utils.GetAuthUserID(c); !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}
	c.JSON(http.StatusOK, gamification.AchievementCatalogResponse{
		Data: ac.achievementsService.GetCatalog(c),
	})
}

// GetUserAchievements godoc
// @Summary      Vitrina de logros del usuario
// @Description  Devuelve el merge catálogo + progreso del propio usuario autenticado (21 filas; unlocked_at null en las que no desbloqueó, current_level y count en null en v1), con has_unseen de la sección achievements embebido. Self-only.
// @Tags         gamification
// @Produce      json
// @Param        id  path  int  true  "ID del usuario (debe ser el autenticado)"
// @Success      200  {object}  gamification.UserAchievementsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users/{id}/achievements [get]
func (ac *achievementsController) GetUserAchievements(c *gin.Context) {
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

	resp, err := ac.achievementsService.GetUserAchievements(c, targetID)
	if err != nil {
		// Error interno de DAO: 500 genérico sin filtrar detalles del driver.
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    "error interno",
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetUserStamps godoc
// @Summary      Vitrina de estampas del usuario
// @Description  Devuelve las estampas del propio usuario autenticado (self-only). En E2 la vitrina está vacía: la tabla solo recibe emisiones de competencia desde E4.
// @Tags         gamification
// @Produce      json
// @Param        id  path  int  true  "ID del usuario (debe ser el autenticado)"
// @Success      200  {object}  gamification.StampsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users/{id}/stamps [get]
func (ac *achievementsController) GetUserStamps(c *gin.Context) {
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

	resp, err := ac.achievementsService.GetUserStamps(c, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    "error interno",
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}
