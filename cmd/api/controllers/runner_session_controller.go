package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// RunnerSessionController define los handlers HTTP del estado de sesión del
// corredor (runner_session).
type RunnerSessionController interface {
	Create(c *gin.Context)
	Finish(c *gin.Context)
	Get(c *gin.Context)
}

type runnerSessionController struct {
	runnerSessionService services.RunnerSessionServiceInterface
	// presencial es opcional (nil en tests sin calendario): hooks D7 de
	// apertura (Create) y cierre (PATCH finished) del día presencial.
	presencial services.PresencialSessionServiceInterface
	// notifier es opcional (nil): emite update:session_state a session:{id}
	// al abrir/cerrar (D10, best-effort, patrón workout-feedback).
	notifier realtime.Notifier
}

// NewRunnerSessionController crea una nueva instancia de RunnerSessionController.
func NewRunnerSessionController(runnerSessionService services.RunnerSessionServiceInterface, presencial services.PresencialSessionServiceInterface, notifier realtime.Notifier) RunnerSessionController {
	return &runnerSessionController{
		runnerSessionService: runnerSessionService,
		presencial:           presencial,
		notifier:             notifier,
	}
}

// sessionStateEvent es el objeto data del frame update:session_state (D10,
// Gap 26). Sin omitempty: los nulls explícitos de opened_at/closed_at son
// parte del contrato del evento.
type sessionStateEvent struct {
	PresencialOpen bool       `json:"presencial_open"`
	OpenedAt       *time.Time `json:"opened_at"`
	ClosedAt       *time.Time `json:"closed_at"`
}

// toSessionStateData proyecta el día post-write al estado del evento (D10).
func toSessionStateData(day *dbs.GroupCalendarDay) sessionStateEvent {
	return sessionStateEvent{
		PresencialOpen: day.PresencialOpenedAt != nil && day.PresencialClosedAt == nil,
		OpenedAt:       day.PresencialOpenedAt,
		ClosedAt:       day.PresencialClosedAt,
	}
}

// emitSessionState emite el frame update:session_state (D10) al canal
// canónico session:{id} con el estado post-write. nil-safe.
func (rc *runnerSessionController) emitSessionState(sessionInstanceID int64, day *dbs.GroupCalendarDay) {
	if rc.notifier == nil || day == nil {
		return
	}
	channel := sessionChannel(sessionInstanceID)
	rc.notifier.Emit(channel, realtime.MarshalUpdateSessionState(channel, toSessionStateData(day)))
}

// respondRunnerSessionError mapea los errores de negocio del service a status HTTP.
func respondRunnerSessionError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := "Internal Server Error"
	switch {
	case errors.Is(err, services.ErrRunnerSessionInvalid):
		statusCode = http.StatusBadRequest
		code = "Bad request"
	case errors.Is(err, services.ErrRunnerSessionForbidden):
		statusCode = http.StatusForbidden
		code = "Forbidden"
	case errors.Is(err, daos.ErrRunnerSessionNotFound), errors.Is(err, services.ErrSessionInstanceNotFound):
		statusCode = http.StatusNotFound
		code = "Not Found"
	case errors.Is(err, services.ErrRunnerSessionClosed):
		statusCode = http.StatusConflict
		code = "session_closed"
	case errors.Is(err, services.ErrRunnerSessionNotOpen):
		statusCode = http.StatusConflict
		code = "session_not_opened"
	}
	c.JSON(statusCode, apierror.APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    err.Error(),
	})
}

// toRunnerSessionResponse mapea el modelo al shape plano de la API.
func toRunnerSessionResponse(rs *dbs.RunnerSession) runnersession.RunnerSessionResponse {
	return runnersession.RunnerSessionResponse{
		ID:                rs.ID,
		SessionInstanceID: rs.SessionInstanceID,
		AthleteUserID:     rs.AthleteUserID,
		Status:            rs.Status,
		StartDate:         rs.StartDate,
		EndDate:           rs.EndDate,
	}
}

// Create godoc
// @Summary      Registrar estado de sesión del corredor (wip)
// @Description  Crea el estado de la sesión en wip para el atleta (self por default o el indicado si el auth es entrenador del equipo). Idempotente: si ya existía, responde 200 con el estado actual sin pisar start_date ni bajar de finished a wip.
// @Tags         runner-session
// @Accept       json
// @Produce      json
// @Param        id    path  int                                            true  "ID de la sesión asignada"
// @Param        body  body  runnersession.CreateRunnerSessionRequest        true  "Datos del estado"
// @Success      201  {object}  runnersession.MutationResponse
// @Success      200  {object}  runnersession.MutationResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError  "code: session_closed | session_not_opened (sesión presencial)"
// @Router       /api/v1/session-instances/{id}/runner [post]
func (rc *runnerSessionController) Create(c *gin.Context) {
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

	var req runnersession.CreateRunnerSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	rs, gateDay, created, err := rc.runnerSessionService.Create(c, authUserID, sessionInstanceID, req)
	if err != nil {
		respondRunnerSessionError(c, err)
		return
	}

	// Hook de apertura D7: después del éxito del write del estado (si el
	// update del día falla, el error sube con el estado del corredor ya
	// persistido — aceptado por diseño). El hook decide si aplica
	// (día presencial + owner); idempotente y solo muta si estaba NULL.
	// Reutiliza el día cargado por el gate del service (sin re-consulta).
	if rc.presencial != nil {
		day, mutated, hookErr := rc.presencial.OnRunnerCreated(c, sessionInstanceID, authUserID, gateDay)
		if hookErr != nil {
			respondRunnerSessionError(c, hookErr)
			return
		}
		if mutated {
			rc.emitSessionState(sessionInstanceID, day)
		}
	}

	message := runnersession.MsgRunnerSessionCreated
	statusCode := http.StatusCreated
	if !created {
		message = runnersession.MsgRunnerSessionExisted
		statusCode = http.StatusOK
	}
	response := toRunnerSessionResponse(rs)
	c.JSON(statusCode, runnersession.MutationResponse{
		Message: message,
		Data:    &response,
	})
}

// Finish godoc
// @Summary      Marcar la sesión del corredor como completada o interrumpida
// @Description  Aplica la transición de status sobre el estado de la sesión, con end_date seteada por el servidor. Status admite "finished" (completar) o "interrupted" (terminar temprano). Transiciones válidas: wip→finished, wip→interrupted e interrupted→finished (re-setea end_date). Idempotente: ya finished con {"status":"finished"} o ya interrupted con {"status":"interrupted"} responde 200 sin cambios. finished→interrupted responde 400.
// @Tags         runner-session
// @Accept       json
// @Produce      json
// @Param        id    path  int                                true  "ID de la sesión asignada"
// @Param        body  body  runnersession.RunnerStatusRequest   true  "Status (finished|interrupted)"
// @Success      200  {object}  runnersession.MutationResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/session-instances/{id}/runner [patch]
func (rc *runnerSessionController) Finish(c *gin.Context) {
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

	var req runnersession.RunnerStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	rs, err := rc.runnerSessionService.Finish(c, authUserID, sessionInstanceID, req)
	if err != nil {
		respondRunnerSessionError(c, err)
		return
	}

	// Hook de cierre D7: solo finished del owner cierra la sesión presencial;
	// interrupted NO cierra. Idempotente vía guard SQL.
	if rs.Status == runnersession.RunnerSessionStatusFinished && rc.presencial != nil {
		day, mutated, hookErr := rc.presencial.OnRunnerFinished(c, sessionInstanceID, authUserID)
		if hookErr != nil {
			respondRunnerSessionError(c, hookErr)
			return
		}
		if mutated {
			rc.emitSessionState(sessionInstanceID, day)
		}
	}

	response := toRunnerSessionResponse(rs)
	message := runnersession.MsgRunnerSessionFinished
	if rs.Status == runnersession.RunnerSessionStatusInterrupted {
		message = runnersession.MsgRunnerSessionInterrupted
	}
	c.JSON(http.StatusOK, runnersession.MutationResponse{
		Message: message,
		Data:    &response,
	})
}

// Get godoc
// @Summary      Consultar estado de la sesión del corredor
// @Description  Devuelve el estado actual de la sesión para el atleta (self por default o el indicado si el auth es entrenador del equipo).
// @Tags         runner-session
// @Accept       json
// @Produce      json
// @Param        id              path  int  false  "ID de la sesión asignada"
// @Param        athlete_user_id query int  false  "ID del atleta (default: usuario autenticado)"
// @Success      200  {object}  runnersession.RunnerSessionListResponse
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/session-instances/{id}/runner [get]
func (rc *runnerSessionController) Get(c *gin.Context) {
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

	athleteUserID, err := parseOptionalPositiveQueryParam(c, "athlete_user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	rs, err := rc.runnerSessionService.Get(c, authUserID, sessionInstanceID, athleteUserID)
	if err != nil {
		respondRunnerSessionError(c, err)
		return
	}

	c.JSON(http.StatusOK, runnersession.RunnerSessionListResponse{
		Data: toRunnerSessionResponse(rs),
	})
}
