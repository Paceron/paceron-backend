package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	_ "simple-arq-golang/cmd/api/domains/joinrequest"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// JoinRequestController define las operaciones HTTP para solicitudes de ingreso.
type JoinRequestController interface {
	Create(c *gin.Context)
	Cancel(c *gin.Context)
	Accept(c *gin.Context)
	Reject(c *gin.Context)
	ListMine(c *gin.Context)
	ListByTeam(c *gin.Context)
	PendingCount(c *gin.Context)
}

type joinRequestController struct {
	joinRequestService services.JoinRequestServiceInterface
}

// NewJoinRequestController crea una nueva instancia de JoinRequestController.
func NewJoinRequestController(joinRequestService services.JoinRequestServiceInterface) JoinRequestController {
	return &joinRequestController{joinRequestService: joinRequestService}
}

// mapJoinRequestError traduce los sentinels de services a (status, code) HTTP.
func mapJoinRequestError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrTeamNotFound):
		return http.StatusNotFound, "TEAM_NOT_FOUND"
	case errors.Is(err, services.ErrTeamNotPublic):
		return http.StatusForbidden, "TEAM_NOT_PUBLIC"
	case errors.Is(err, services.ErrTeamFull):
		return http.StatusConflict, "TEAM_FULL"
	case errors.Is(err, services.ErrAlreadyMember):
		return http.StatusConflict, "ALREADY_MEMBER"
	case errors.Is(err, services.ErrJoinRequestAlreadyPending):
		return http.StatusConflict, "JOIN_REQUEST_ALREADY_PENDING"
	case errors.Is(err, services.ErrJoinRequestNotFound):
		return http.StatusNotFound, "JOIN_REQUEST_NOT_FOUND"
	case errors.Is(err, services.ErrJoinRequestForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, services.ErrJoinRequestNotPending):
		return http.StatusConflict, "JOIN_REQUEST_NOT_PENDING"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}

func respondJoinRequestError(c *gin.Context, err error) {
	statusCode, code := mapJoinRequestError(err)
	c.JSON(statusCode, apierror.APIError{StatusCode: statusCode, Code: code, Message: err.Error()})
}

// Create godoc
// @Summary      Solicitar ingreso a un equipo
// @Tags         join-requests
// @Produce      json
// @Param        id  path      int  true  "Team ID"
// @Success      201  {object}  joinrequest.JoinRequestResponse
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/join-requests [post]
func (jc *joinRequestController) Create(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "team id debe ser un número válido"})
		return
	}

	runnerID, _ := utils.GetAuthUserID(c)
	response, err := jc.joinRequestService.Create(c, teamID, runnerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// Cancel godoc
// @Summary      Cancelar solicitud propia
// @Tags         join-requests
// @Param        id  path  int  true  "Join request ID"
// @Success      204
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/join-requests/{id} [delete]
func (jc *joinRequestController) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := jc.joinRequestService.Cancel(c, id, callerID); err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Accept godoc
// @Summary      Aceptar solicitud de ingreso
// @Tags         join-requests
// @Produce      json
// @Param        id  path  int  true  "Join request ID"
// @Success      200
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/join-requests/{id}/accept [post]
func (jc *joinRequestController) Accept(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := jc.joinRequestService.Accept(c, id, callerID); err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solicitud aceptada"})
}

// Reject godoc
// @Summary      Rechazar solicitud de ingreso
// @Tags         join-requests
// @Produce      json
// @Param        id  path  int  true  "Join request ID"
// @Success      200
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/join-requests/{id}/reject [post]
func (jc *joinRequestController) Reject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := jc.joinRequestService.Reject(c, id, callerID); err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solicitud rechazada"})
}

// ListMine godoc
// @Summary      Mis solicitudes de ingreso
// @Tags         join-requests
// @Produce      json
// @Success      200  {array}  joinrequest.JoinRequestResponse
// @Router       /api/v1/join-requests/mine [get]
func (jc *joinRequestController) ListMine(c *gin.Context) {
	runnerID, _ := utils.GetAuthUserID(c)
	response, err := jc.joinRequestService.ListMine(c, runnerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListByTeam godoc
// @Summary      Solicitudes pendientes de un equipo
// @Tags         join-requests
// @Produce      json
// @Param        id  path  int  true  "Team ID"
// @Success      200  {array}  joinrequest.JoinRequestResponse
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/join-requests [get]
func (jc *joinRequestController) ListByTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "Bad request", Message: "team id debe ser un número válido"})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := jc.joinRequestService.ListByTeam(c, teamID, callerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// PendingCount godoc
// @Summary      Conteo agregado de solicitudes pendientes
// @Tags         join-requests
// @Produce      json
// @Success      200  {object}  joinrequest.PendingCountResponse
// @Router       /api/v1/join-requests/pending-count [get]
func (jc *joinRequestController) PendingCount(c *gin.Context) {
	ownerID, _ := utils.GetAuthUserID(c)
	count, err := jc.joinRequestService.PendingCount(c, ownerID)
	if err != nil {
		respondJoinRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}
