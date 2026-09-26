package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
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
}

// NewRunnerSessionController crea una nueva instancia de RunnerSessionController.
func NewRunnerSessionController(runnerSessionService services.RunnerSessionServiceInterface) RunnerSessionController {
	return &runnerSessionController{
		runnerSessionService: runnerSessionService,
	}
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

	rs, created, err := rc.runnerSessionService.Create(c, authUserID, sessionInstanceID, req)
	if err != nil {
		respondRunnerSessionError(c, err)
		return
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
// @Summary      Marcar la sesión del corredor como completada
// @Description  Pasa el estado a finished con end_date seteada por el servidor (solo desde wip). Idempotente: ya finished responde 200 sin cambios.
// @Tags         runner-session
// @Accept       json
// @Produce      json
// @Param        id    path  int                                true  "ID de la sesión asignada"
// @Param        body  body  runnersession.RunnerStatusRequest   true  "Status (finished)"
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

	response := toRunnerSessionResponse(rs)
	c.JSON(http.StatusOK, runnersession.MutationResponse{
		Message: runnersession.MsgRunnerSessionFinished,
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