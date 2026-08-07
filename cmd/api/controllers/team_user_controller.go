package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/teamuser"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// TeamUserController define las operaciones HTTP para la asociación usuario-equipo.
type TeamUserController interface {
	AddUser(c *gin.Context)
	RemoveUser(c *gin.Context)
	GetUsersByTeam(c *gin.Context)
}

type teamUserController struct {
	teamUserService services.TeamUserServiceInterface
}

// NewTeamUserController crea una nueva instancia de TeamUserController.
func NewTeamUserController(teamUserService services.TeamUserServiceInterface) TeamUserController {
	return &teamUserController{
		teamUserService: teamUserService,
	}
}

// AddUser godoc
// @Summary      Agregar usuario a equipo
// @Description  Agrega un usuario a un equipo con un rol específico. Solo el entrenador del equipo puede hacerlo
// @Tags         team-users
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "Team ID"
// @Param        body  body      teamuser.AddTeamUserRequest   true  "Usuario a agregar"
// @Success      201   {object}  teamuser.TeamUserResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/users [post]
func (tuc *teamUserController) AddUser(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	var req teamuser.AddTeamUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := tuc.teamUserService.AddUser(c, teamID, callerID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" || errMsg == "usuario no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el usuario ya pertenece a este equipo" {
			statusCode = http.StatusConflict
			code = "Conflict"
		} else if errMsg == "solo el entrenador puede agregar usuarios al equipo" {
			statusCode = http.StatusForbidden
			code = "Forbidden"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// RemoveUser godoc
// @Summary      Quitar usuario de equipo
// @Description  Quita un usuario de un equipo (soft-delete de la asociación). El propio usuario puede salirse, o el entrenador del equipo puede quitar a otro
// @Tags         team-users
// @Produce      json
// @Param        id       path  int  true  "Team ID"
// @Param        user_id  path  int  true  "User ID"
// @Success      200  {object}  teamuser.RemoveTeamUserResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/users/{user_id} [delete]
func (tuc *teamUserController) RemoveUser(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	targetUserID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := tuc.teamUserService.RemoveUser(c, teamID, callerID, targetUserID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" || errMsg == "el usuario no pertenece a este equipo" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador puede quitar a otro usuario del equipo" {
			statusCode = http.StatusForbidden
			code = "Forbidden"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, teamuser.RemoveTeamUserResponse{Message: "Usuario removido del equipo correctamente"})
}

// GetUsersByTeam godoc
// @Summary      Listar usuarios de un equipo
// @Description  Devuelve todos los miembros activos de un equipo. Solo otro miembro del equipo puede consultarlo
// @Tags         team-users
// @Produce      json
// @Param        id    path  int  true  "Team ID"
// @Success      200   {array}  teamuser.TeamUserResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/users [get]
func (tuc *teamUserController) GetUsersByTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := tuc.teamUserService.GetUsersByTeam(c, teamID, callerID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el usuario no pertenece a este equipo" {
			statusCode = http.StatusForbidden
			code = "Forbidden"
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
