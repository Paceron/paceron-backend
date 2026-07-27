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

		if errMsg == "equipo no encontrado" || errMsg == "no se encontró un usuario con el email proporcionado" {
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
