package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/pushtoken"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type PushTokenController interface {
	RegisterToken(c *gin.Context)
}

type pushTokenController struct {
	pushTokenService services.PushTokenServiceInterface
}

func NewPushTokenController(pushTokenService services.PushTokenServiceInterface) PushTokenController {
	return &pushTokenController{
		pushTokenService: pushTokenService,
	}
}

// RegisterToken godoc
// @Summary      Register push token
// @Description  Registra o actualiza el token de push de un dispositivo para el usuario autenticado. Upsert por token: si el dispositivo cambia de cuenta, el siguiente registro reescribe el dueño.
// @Tags         push-tokens
// @Accept       json
// @Produce      json
// @Param        body  body      pushtoken.RegisterPushTokenRequest  true  "Token de push y plataforma"
// @Success      200   {object}  pushtoken.RegisterPushTokenResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/push-tokens [post]
func (pc *pushTokenController) RegisterToken(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	var req pushtoken.RegisterPushTokenRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if err := pc.pushTokenService.RegisterToken(c, authUserID, &req); err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		if strings.Contains(err.Error(), "platform inválida") {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, pushtoken.RegisterPushTokenResponse{Message: "Token de push registrado correctamente"})
}
