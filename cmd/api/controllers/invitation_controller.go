package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/invitation"
	"simple-arq-golang/cmd/api/services"
)

// InvitationController define las operaciones HTTP para invitaciones.
type InvitationController interface {
	InviteRunner(c *gin.Context)
	ListPendingInvitations(c *gin.Context)
	AcceptInvitation(c *gin.Context)
	RejectInvitation(c *gin.Context)
}

type invitationController struct {
	invitationService services.InvitationServiceInterface
}

// NewInvitationController crea una nueva instancia de InvitationController.
func NewInvitationController(invitationService services.InvitationServiceInterface) InvitationController {
	return &invitationController{
		invitationService: invitationService,
	}
}

// InviteRunner godoc
// @Summary      Invitar corredor por email
// @Description  Envía una invitación por email a un usuario existente para unirlo a un equipo
// @Tags         invitations
// @Accept       json
// @Produce      json
// @Param        id    path      int                           true  "Team ID"
// @Param        body  body      invitation.InviteRunnerRequest true  "Email del usuario a invitar"
// @Success      200   {object}  invitation.InviteRunnerResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/invite [post]
func (ic *invitationController) InviteRunner(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	var req invitation.InviteRunnerRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := ic.invitationService.InviteRunner(c, teamID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		switch errMsg {
		case "equipo no encontrado", "no se encontró un usuario con el email proporcionado":
			statusCode = http.StatusNotFound
			code = "Not Found"
		case "el usuario ya pertenece a este equipo", "ya existe una invitación pendiente para este usuario en este equipo":
			statusCode = http.StatusConflict
			code = "Conflict"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListPendingInvitations godoc
// @Summary      Listar invitaciones pendientes de un equipo
// @Description  Devuelve las invitaciones pendientes (no vencidas) de un equipo
// @Tags         invitations
// @Produce      json
// @Param        id  path      int  true  "Team ID"
// @Success      200 {array}   invitation.InvitationResponse
// @Failure      400 {object}  apierror.APIError
// @Failure      404 {object}  apierror.APIError
// @Failure      500 {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/invitations [get]
func (ic *invitationController) ListPendingInvitations(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	response, err := ic.invitationService.ListPendingInvitations(c, teamID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// AcceptInvitation godoc
// @Summary      Aceptar invitación
// @Description  El usuario invitado acepta la invitación y queda como corredor del equipo
// @Tags         invitations
// @Accept       json
// @Produce      json
// @Param        id    path      int                                true  "Invitation ID"
// @Param        body  body      invitation.RespondInvitationRequest true  "ID del usuario que responde"
// @Success      200   {object}  invitation.RespondInvitationResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/invitations/{id}/accept [post]
func (ic *invitationController) AcceptInvitation(c *gin.Context) {
	ic.respondInvitation(c, ic.invitationService.AcceptInvitation)
}

// RejectInvitation godoc
// @Summary      Rechazar invitación
// @Description  El usuario invitado rechaza la invitación
// @Tags         invitations
// @Accept       json
// @Produce      json
// @Param        id    path      int                                true  "Invitation ID"
// @Param        body  body      invitation.RespondInvitationRequest true  "ID del usuario que responde"
// @Success      200   {object}  invitation.RespondInvitationResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/invitations/{id}/reject [post]
func (ic *invitationController) RejectInvitation(c *gin.Context) {
	ic.respondInvitation(c, ic.invitationService.RejectInvitation)
}

// respondInvitation centraliza el parseo/validación/mapeo de errores compartido
// entre accept y reject, que solo difieren en qué método del service invocan.
func (ic *invitationController) respondInvitation(
	c *gin.Context,
	respond func(ctx *gin.Context, invitationID, userID int64) (*invitation.RespondInvitationResponse, error),
) {
	invitationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "invitation id debe ser un número válido",
		})
		return
	}

	var req invitation.RespondInvitationRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := respond(c, invitationID, req.UserID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		switch errMsg {
		case "invitación no encontrada":
			statusCode = http.StatusNotFound
			code = "Not Found"
		case "la invitación no pertenece a este usuario":
			statusCode = http.StatusForbidden
			code = "Forbidden"
		case "la invitación ya fue respondida", "la invitación ha expirado":
			statusCode = http.StatusConflict
			code = "Conflict"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
