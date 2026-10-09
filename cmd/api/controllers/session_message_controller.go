package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/sessionmessage"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// SessionMessageController define los handlers HTTP del chat de sesión.
type SessionMessageController interface {
	Create(c *gin.Context)
	List(c *gin.Context)
}

type sessionMessageController struct {
	sessionMessageService services.SessionMessageServiceInterface
	// notifier es opcional (nil en tests = sin eventos): emite
	// control:message_created a session:{id} al crear un mensaje (D9, best-effort).
	notifier realtime.Notifier
}

// NewSessionMessageController crea una nueva instancia de SessionMessageController.
func NewSessionMessageController(sessionMessageService services.SessionMessageServiceInterface, notifier realtime.Notifier) SessionMessageController {
	return &sessionMessageController{
		sessionMessageService: sessionMessageService,
		notifier:              notifier,
	}
}

// respondSessionMessageError mapea los errores de negocio a status HTTP, con
// los mismos mensajes que el resto de los endpoints de session-instances.
func respondSessionMessageError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := "Internal Server Error"
	message := err.Error()
	switch {
	case errors.Is(err, services.ErrSessionMessageInvalid):
		statusCode = http.StatusBadRequest
		code = "Bad request"
	case errors.Is(err, services.ErrSessionMessageForbidden):
		statusCode = http.StatusForbidden
		code = "Forbidden"
		message = "no autorizado"
	case errors.Is(err, services.ErrCalendarInstanceNotFound):
		statusCode = http.StatusNotFound
		code = "Not Found"
		message = "sesión instancia no encontrada"
	}
	c.JSON(statusCode, apierror.APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	})
}

// Create godoc
// @Summary      Enviar un mensaje al chat de la sesión
// @Description  Crea un mensaje de sesión (type info/aviso/alerta) dirigido a todos los participantes (recipient_mode all) o a uno (direct) o varios (multiple) destinatarios concretos. El emisor debe tener acceso a la instancia; sender_role se deriva del owner del equipo. Tras crear, emite el aviso WS control:message_created en session:{id}.
// @Tags         session-messages
// @Accept       json
// @Produce      json
// @Param        id    path  int  true  "ID de la sesión instanciada"
// @Param        body  body  sessionmessage.SendMessageRequest  true  "Datos del mensaje"
// @Success      201  {object}  sessionmessage.SessionMessageResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/session-instances/{id}/messages [post]
func (sc *sessionMessageController) Create(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	sessionInstanceID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	var req sessionmessage.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	response, err := sc.sessionMessageService.Create(c, authUserID, sessionInstanceID, req)
	if err != nil {
		respondSessionMessageError(c, err)
		return
	}

	if sc.notifier != nil {
		channel := sessionChannel(sessionInstanceID)
		sc.notifier.Emit(channel, realtime.MarshalControlMessageCreated(channel, response.ID))
	}
	c.JSON(http.StatusCreated, response)
}

// List godoc
// @Summary      Historial de mensajes de la sesión
// @Description  Devuelve los mensajes de la sesión que el consultante ve (emisor, recipient_mode all o destinatario), ordenados cronológicamente. El query param since cursa el historial: mensajes con id mayor al valor (ausente/0 = todo).
// @Tags         session-messages
// @Produce      json
// @Param        id     path  int  true   "ID de la sesión instanciada"
// @Param        since  query int  false  "Cursor: mensajes con id mayor a este (default: todos)"
// @Success      200  {object}  sessionmessage.MessagesListResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/session-instances/{id}/messages [get]
func (sc *sessionMessageController) List(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	sessionInstanceID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	sinceID, err := parseSinceQueryParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	response, err := sc.sessionMessageService.List(c, authUserID, sessionInstanceID, sinceID)
	if err != nil {
		respondSessionMessageError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// parseSinceQueryParam parsea el cursor opcional del GET: ausente/0 = todo el
// historial; negativo o no numérico es error (400).
func parseSinceQueryParam(c *gin.Context) (int64, error) {
	raw := c.Query("since")
	if raw == "" {
		return 0, nil
	}
	since, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, errors.New("since debe ser un número entero")
	}
	if since < 0 {
		return 0, errors.New("since debe ser mayor o igual a 0")
	}
	return since, nil
}
