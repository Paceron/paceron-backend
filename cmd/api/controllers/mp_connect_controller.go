package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/mpconnect"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// MPConnectControllerInterface define los endpoints de conexión OAuth con MP.
type MPConnectControllerInterface interface {
	GetAuthURL(c *gin.Context)
	HandleCallback(c *gin.Context)
	GetStatus(c *gin.Context)
	HandleDeauthWebhook(c *gin.Context)
}

type mpConnectController struct {
	service services.MPConnectServiceInterface
}

func NewMPConnectController(svc services.MPConnectServiceInterface) MPConnectControllerInterface {
	return &mpConnectController{service: svc}
}

// GetAuthURL godoc
// @Summary      Get Mercado Pago OAuth authorization URL
// @Description  Returns the authorization URL to connect a Mercado Pago account for split payments.
// @Tags         mercadopago-connect
// @Accept       json
// @Produce      json
// @Success      200      {object}  mpconnect.AuthURLResponse
// @Failure      401      {object}  apierror.APIError
// @Failure      500      {object}  apierror.APIError
// @Router       /api/v1/mercadopago/connect [get]
func (c *mpConnectController) GetAuthURL(ctx *gin.Context) {
	userID, ok := utils.GetAuthUserID(ctx)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "no autenticado",
		})
		return
	}

	resp, err := c.service.GetAuthURL(ctx, userID)
	if err != nil {
		statusCode, code := mapMPConnectError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// HandleCallback godoc
// @Summary      Handle Mercado Pago OAuth callback
// @Description  Processes the OAuth callback from Mercado Pago, exchanges code for tokens, and stores the connection.
// @Tags         mercadopago-connect
// @Accept       json
// @Produce      json
// @Param        code   query     string  true  "Authorization code"
// @Param        state  query     string  true  "CSRF state"
// @Success      200    {object}  mpconnect.CallbackResponse
// @Failure      400    {object}  apierror.APIError
// @Failure      500    {object}  apierror.APIError
// @Router       /api/v1/mercadopago/connect/callback [get]
func (c *mpConnectController) HandleCallback(ctx *gin.Context) {
	var req mpconnect.CallbackRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "parámetros inválidos",
		})
		return
	}

	resp, err := c.service.HandleCallback(ctx, &req)
	if err != nil {
		statusCode, code := mapMPConnectError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetStatus godoc
// @Summary      Get Mercado Pago connection status
// @Description  Returns whether the authenticated user has a connected Mercado Pago account.
// @Tags         mercadopago-connect
// @Accept       json
// @Produce      json
// @Success      200    {object}  mpconnect.StatusResponse
// @Failure      401    {object}  apierror.APIError
// @Failure      500    {object}  apierror.APIError
// @Router       /api/v1/mercadopago/connect/status [get]
func (c *mpConnectController) GetStatus(ctx *gin.Context) {
	userID, ok := utils.GetAuthUserID(ctx)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "no autenticado",
		})
		return
	}

	resp, err := c.service.GetStatus(ctx, userID)
	if err != nil {
		statusCode, code := mapMPConnectError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// HandleDeauthWebhook godoc
// @Summary      Handle Mercado Pago deauthorization webhook
// @Description  Marks the seller connection as deauthorized when MP notifies the seller disconnected the app. Idempotent (MP re-sends).
// @Tags         mercadopago-connect
// @Accept       json
// @Produce      json
// @Param        body  body      mpconnect.DeauthWebhookRequest  true  "Deauthorization notification"
// @Success      200   {object}  map[string]string
// @Failure      400   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/mercadopago/webhook/connect [post]
func (c *mpConnectController) HandleDeauthWebhook(ctx *gin.Context) {
	var req mpconnect.DeauthWebhookRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user_id (de MP) requerido",
		})
		return
	}

	if err := c.service.HandleDeauthorization(ctx, req.UserID); err != nil {
		statusCode, code := mapMPConnectError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// mapMPConnectError traduce los errores del servicio a status/code de apierror.
func mapMPConnectError(err error) (int, string) {
	errMsg := err.Error()

	switch {
	case errMsg == "configuración de Mercado Pago incompleta":
		return http.StatusInternalServerError, constants.ErrorCodeSellerNotConnected
	case errMsg == "el entrenador debe conectar su cuenta de Mercado Pago":
		return http.StatusConflict, constants.ErrorCodeSellerNotConnected
	case errMsg == "state inválido":
		return http.StatusBadRequest, "Invalid State"
	case errMsg == "state expirado":
		return http.StatusBadRequest, "State Expired"
	case errMsg == "parámetros code y state requeridos":
		return http.StatusBadRequest, "Bad request"
	case errMsg == "formato de state inválido":
		return http.StatusBadRequest, "Invalid State"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}