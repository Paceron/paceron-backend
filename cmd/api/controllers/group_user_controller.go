package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/groupuser"
	"simple-arq-golang/cmd/api/services"
)

// GroupUserController define las operaciones HTTP para la asociación usuario-grupo.
type GroupUserController interface {
	AddUser(c *gin.Context)
	RemoveUser(c *gin.Context)
	GetUsersByGroup(c *gin.Context)
}

type groupUserController struct {
	groupUserService services.GroupUserServiceInterface
}

// NewGroupUserController crea una nueva instancia de GroupUserController.
func NewGroupUserController(groupUserService services.GroupUserServiceInterface) GroupUserController {
	return &groupUserController{
		groupUserService: groupUserService,
	}
}

// AddUser godoc
// @Summary      Agregar usuario a grupo
// @Description  Agrega un usuario a un grupo dentro de un equipo
// @Tags         group-users
// @Accept       json
// @Produce      json
// @Param        id        path      int                           true  "Team ID"
// @Param        group_id  path      int                           true  "Group ID"
// @Param        body      body      groupuser.AddGroupUserRequest  true  "Usuario a agregar"
// @Success      201   {object}  groupuser.GroupUserResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/groups/{group_id}/users [post]
func (guc *groupUserController) AddUser(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group id debe ser un número válido",
		})
		return
	}

	var req groupuser.AddGroupUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := guc.groupUserService.AddUser(c, teamID, groupID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "grupo no encontrado en este equipo" || errMsg == "usuario no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el usuario ya pertenece a este grupo" {
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

	c.JSON(http.StatusCreated, response)
}

// RemoveUser godoc
// @Summary      Quitar usuario de grupo
// @Description  Quita un usuario de un grupo (soft-delete de la asociación)
// @Tags         group-users
// @Produce      json
// @Param        id       path  int  true  "Group ID"
// @Param        user_id  path  int  true  "User ID"
// @Success      200  {object}  groupuser.RemoveGroupUserResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/groups/{id}/users/{user_id} [delete]
func (guc *groupUserController) RemoveUser(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group id debe ser un número válido",
		})
		return
	}

	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return
	}

	if err := guc.groupUserService.RemoveUser(c, groupID, userID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "grupo no encontrado" || errMsg == "el usuario no pertenece a este grupo" {
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

	c.JSON(http.StatusOK, groupuser.RemoveGroupUserResponse{Message: "Usuario removido del grupo correctamente"})
}

// GetUsersByGroup godoc
// @Summary      Listar usuarios de un grupo
// @Description  Devuelve todos los miembros activos de un grupo
// @Tags         group-users
// @Produce      json
// @Param        id    path  int  true  "Group ID"
// @Success      200   {array}  groupuser.GroupUserResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/groups/{id}/users [get]
func (guc *groupUserController) GetUsersByGroup(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group id debe ser un número válido",
		})
		return
	}

	response, err := guc.groupUserService.GetUsersByGroup(c, groupID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "grupo no encontrado" {
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
