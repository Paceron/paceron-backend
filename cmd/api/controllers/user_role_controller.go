package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/userrole"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type UserRoleController interface {
	AssignRole(c *gin.Context)
	RemoveRole(c *gin.Context)
	ActivateEntrenador(c *gin.Context)
	DeactivateEntrenador(c *gin.Context)
}

type userRoleController struct {
	userRoleService services.UserRoleServiceInterface
}

func NewUserRoleController(userRoleService services.UserRoleServiceInterface) UserRoleController {
	return &userRoleController{
		userRoleService: userRoleService,
	}
}

// forbiddenNotSelf centraliza la respuesta 403 compartida por los cuatro endpoints de
// este controller — todos son self-only, nadie gestiona roles de otro usuario.
func forbiddenNotSelf(c *gin.Context) {
	c.JSON(http.StatusForbidden, apierror.APIError{
		StatusCode: http.StatusForbidden,
		Code:       "Forbidden",
		Message:    "solo podés gestionar tus propios roles",
	})
}

// AssignRole godoc
// @Summary      Assign role to user
// @Description  Assigns a role to yourself, with optional tier (default: "base"). Self only.
// @Tags         user-roles
// @Accept       json
// @Produce      json
// @Param        id    path      int                        true  "User ID"
// @Param        body  body      userrole.AssignRoleRequest  true  "Role to assign"
// @Success      201   {object}  userrole.UserRoleResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
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

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelf(c)
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

// RemoveRole godoc
// @Summary      Remove a role from a user
// @Description  Soft-deletes your own active role assignment, identified by role_id. Self only.
// @Tags         user-roles
// @Accept       json
// @Produce      json
// @Param        id       path  int  true  "User ID"
// @Param        role_id  path  int  true  "Role ID"
// @Success      200 {object}  userrole.RemoveRoleResponse
// @Failure      400 {object}  apierror.APIError
// @Failure      403 {object}  apierror.APIError
// @Failure      404 {object}  apierror.APIError
// @Failure      500 {object}  apierror.APIError
// @Router       /api/v1/users/{id}/roles/{role_id} [delete]
func (urc *userRoleController) RemoveRole(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelf(c)
		return
	}

	roleID, err := strconv.ParseInt(c.Param("role_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "role id debe ser un número válido",
		})
		return
	}

	if err := urc.userRoleService.RemoveRole(c, userID, roleID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el usuario no tiene asignado este rol" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if strings.Contains(errMsg, "no se puede eliminar") {
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

	c.JSON(http.StatusOK, userrole.RemoveRoleResponse{Message: "Rol eliminado correctamente"})
}

// ActivateEntrenador godoc
// @Summary      Activar rol entrenador
// @Description  Activa el rol entrenador del usuario autenticado sobre sí mismo. Exige confirmar la contraseña actual y un alias bancario válido (propio o recién provisto)
// @Tags         user-roles
// @Accept       json
// @Produce      json
// @Param        id    path      int                              true  "User ID"
// @Param        body  body      userrole.ActivateEntrenadorRequest true  "Contraseña actual y alias bancario"
// @Success      201   {object}  userrole.UserRoleResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      401   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/users/{id}/entrenador-role [post]
func (urc *userRoleController) ActivateEntrenador(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelf(c)
		return
	}

	var req userrole.ActivateEntrenadorRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := urc.userRoleService.ActivateEntrenador(c, userID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		switch {
		case errMsg == "usuario no encontrado":
			statusCode = http.StatusNotFound
			code = "Not Found"
		case errMsg == "contraseña actual incorrecta":
			statusCode = http.StatusUnauthorized
			code = "Unauthorized"
		case errMsg == "el usuario ya tiene asignado este rol":
			statusCode = http.StatusConflict
			code = "Conflict"
		case errMsg == "se requiere un alias bancario para activar el rol entrenador",
			strings.Contains(errMsg, "bank_alias"):
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

// DeactivateEntrenador godoc
// @Summary      Desactivar rol entrenador
// @Description  Desactiva el rol entrenador del usuario autenticado sobre sí mismo. Bloqueado mientras lidere equipos activos
// @Tags         user-roles
// @Produce      json
// @Param        id  path  int  true  "User ID"
// @Success      200 {object}  userrole.RemoveRoleResponse
// @Failure      400 {object}  apierror.APIError
// @Failure      403 {object}  apierror.APIError
// @Failure      404 {object}  apierror.APIError
// @Failure      409 {object}  apierror.APIError
// @Failure      500 {object}  apierror.APIError
// @Router       /api/v1/users/{id}/entrenador-role [delete]
func (urc *userRoleController) DeactivateEntrenador(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelf(c)
		return
	}

	if err := urc.userRoleService.DeactivateEntrenador(c, userID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		switch errMsg {
		case "el usuario no tiene asignado este rol":
			statusCode = http.StatusNotFound
			code = "Not Found"
		case "no podés desactivar el rol entrenador mientras lideres equipos activos":
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

	c.JSON(http.StatusOK, userrole.RemoveRoleResponse{Message: "Rol entrenador desactivado correctamente"})
}
