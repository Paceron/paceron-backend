package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/userrole"
	"simple-arq-golang/cmd/api/services"
)

type UserRoleController interface {
	AssignRole(c *gin.Context)
}

type userRoleController struct {
	userRoleService services.UserRoleServiceInterface
}

func NewUserRoleController(userRoleService services.UserRoleServiceInterface) UserRoleController {
	return &userRoleController{
		userRoleService: userRoleService,
	}
}

// AssignRole godoc
// @Summary      Assign role to user
// @Description  Assigns a role to a user with optional tier (default: "base")
// @Tags         user-roles
// @Accept       json
// @Produce      json
// @Param        id    path      int                        true  "User ID"
// @Param        body  body      userrole.AssignRoleRequest  true  "Role to assign"
// @Success      201   {object}  userrole.UserRoleResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/users/{id}/roles [post]
func (urc *userRoleController) AssignRole(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return
	}

	var req userrole.AssignRoleRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := urc.userRoleService.AssignRole(c, userID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "usuario no encontrado" || errMsg == "rol no encontrado" || errMsg == "tier no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el usuario ya tiene asignado este rol" {
			statusCode = http.StatusConflict
			code = "Conflict"
		} else if errMsg == "el tier no pertenece al rol especificado" || errMsg == "el tier por defecto 'base' no existe para este rol" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
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
