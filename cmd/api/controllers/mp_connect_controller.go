package controllers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/mpconnect"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
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
	// URLs del frontend a las que vuelve el navegador tras el callback. Son dos
	// porque los destinos son de naturaleza distinta: un origen HTTPS para la web
	// y un deep link (paceron://) para la app nativa.
	webReturnURL string
	appReturnURL string
}

func NewMPConnectController(svc services.MPConnectServiceInterface, webReturnURL, appReturnURL string) MPConnectControllerInterface {
	return &mpConnectController{
		service:      svc,
		webReturnURL: webReturnURL,
		appReturnURL: appReturnURL,
	}
}

// GetAuthURL godoc
// @Summary      Get Mercado Pago OAuth authorization URL
// @Description  Returns the authorization URL to connect a Mercado Pago account for split payments.
// @Tags         mercadopago-connect
// @Accept       json
// @Produce      json
// @Param        platform query     string  false  "Destino de retorno tras el callback"  Enums(web, app)  default(web)
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

	// El destino de retorno se decide acá y viaja adentro del state — el
	// redirect_uri registrado en Mercado Pago es fijo y no puede variar por
	// request. Un platform desconocido colapsa a web dentro de BuildState.
	resp, err := c.service.GetAuthURL(ctx, userID, ctx.DefaultQuery("platform", mpconnect.TargetWeb))
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
// @Description  Processes the OAuth callback from Mercado Pago, exchanges code for tokens, stores the connection and redirects the browser back to the frontend with the result.
// @Tags         mercadopago-connect
// @Accept       json
// @Produce      json
// @Param        code   query     string  true  "Authorization code"
// @Param        state  query     string  true  "CSRF state"
// @Success      302    {string}  string  "Redirect al frontend: ?status=success, o ?status=error&reason=<slug> si falló"
// @Header       302    {string}  Location  "URL de retorno (origen web o deep link de la app)"
// @Success      200    {object}  mpconnect.CallbackResponse  "Solo si no hay URLs de retorno configuradas"
// @Router       /api/v1/mercadopago/connect/callback [get]
func (c *mpConnectController) HandleCallback(ctx *gin.Context) {
	var req mpconnect.CallbackRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		customlogger.Warn(ctx, "MP OAuth callback bad request",
			customlogger.Tag("error", err.Error()),
			customlogger.TagMethod("HandleCallback"))
		if !c.redirectResult(ctx, ctx.Query("state"), "error", "bad_request") {
			ctx.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "parámetros inválidos",
			})
		}
		return
	}

	customlogger.Info(ctx, "[DEBUG] HandleCallback controller",
		customlogger.Tag("code", utils.MaskSecret(req.Code)),
		customlogger.Tag("state", req.State),
		customlogger.Tag("error", req.Error),
		customlogger.Tag("error_description", req.ErrorDescription),
		customlogger.TagMethod("HandleCallback"))

	resp, err := c.service.HandleCallback(ctx, &req)
	if err != nil {
		reason := mapCallbackReason(err)
		customlogger.Error(ctx, "MP OAuth callback process failed", err,
			customlogger.Tag("reason", reason),
			customlogger.Tag("state", req.State),
			customlogger.TagMethod("HandleCallback"))
		if !c.redirectResult(ctx, req.State, "error", reason) {
			statusCode, code := mapMPConnectError(err)
			ctx.JSON(statusCode, apierror.APIError{
				StatusCode: statusCode,
				Code:       code,
				Message:    err.Error(),
			})
		}
		return
	}

	customlogger.Info(ctx, "[DEBUG] HandleCallback controller OK",
		customlogger.Tag("success", fmt.Sprintf("%t", resp.Success)),
		customlogger.Tag("message", resp.Message),
		customlogger.Tag("state", req.State),
		customlogger.TagMethod("HandleCallback"))
	if !c.redirectResult(ctx, req.State, "success", "") {
		ctx.JSON(http.StatusOK, resp)
	}
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

// redirectResult manda el navegador de vuelta al frontend con el resultado del
// callback. Devuelve false si no hay URL de retorno configurada para ese target:
// en ese caso no escribe nada y el caller responde JSON como antes, para que un
// entorno mal configurado degrade a lo anterior en vez de redirigir a "".
func (c *mpConnectController) redirectResult(ctx *gin.Context, state, status, reason string) bool {
	base := c.returnURLFor(mpconnect.TargetFromState(state))
	if base == "" {
		customlogger.Warn(ctx, "MP OAuth return URL no configurada, respondiendo JSON",
			customlogger.Tag("state", state),
			customlogger.TagMethod("redirectResult"))
		return false
	}

	ctx.Redirect(http.StatusFound, buildReturnURL(base, status, reason))
	return true
}

// returnURLFor resuelve a qué URL del frontend volver. El target sale del state
// y ya viene normalizado al enum — por eso esto no puede terminar en un open
// redirect: el destino siempre es uno de los dos configurados en el servidor.
func (c *mpConnectController) returnURLFor(target string) string {
	if target == mpconnect.TargetApp {
		return c.appReturnURL
	}
	return c.webReturnURL
}

// buildReturnURL le agrega status (y reason, si hay) a la URL de retorno,
// preservando la query que ya tuviera. Sirve igual para una URL web
// (https://host/path) que para un deep link (paceron://mp-connect/callback).
func buildReturnURL(base, status, reason string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}

	query := parsed.Query()
	query.Set("status", status)
	if reason != "" {
		query.Set("reason", reason)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

// mapCallbackReason traduce el error del service a un slug ASCII estable para la
// query del redirect.
//
// A diferencia de mapMPConnectError (que sigue sirviendo a los endpoints que
// responden JSON), acá no se filtra el texto del error: esa URL termina en el
// historial del navegador y en los logs de acceso del hosting del frontend, y el
// mensaje crudo puede arrastrar la respuesta de Mercado Pago. Además le da al
// frontend un contrato estable para mapear a un mensaje en español.
func mapCallbackReason(err error) string {
	errMsg := err.Error()

	switch {
	case errMsg == "parámetros code y state requeridos":
		return "missing_params"
	case errMsg == "state inválido", errMsg == "formato de state inválido":
		return "invalid_state"
	case errMsg == "state expirado":
		return "expired_state"
	case strings.HasPrefix(errMsg, "error en autorización:"):
		return "authorization_denied"
	case errMsg == "configuración de Mercado Pago incompleta":
		return "config_error"
	case strings.HasPrefix(errMsg, "error al obtener tokens"),
		strings.HasPrefix(errMsg, "error al refrescar tokens"),
		strings.HasPrefix(errMsg, "error al obtener info de usuario"):
		return "exchange_failed"
	case strings.HasPrefix(errMsg, "error cifrando"),
		strings.HasPrefix(errMsg, "error guardando"):
		return "save_failed"
	default:
		return "unknown_error"
	}
}
