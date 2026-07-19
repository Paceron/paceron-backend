package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/role"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type RoleController interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	GetByID(c *gin.Context)
	GetByName(c *gin.Context)
	GetAll(c *gin.Context)
}

type roleController struct {
	roleService services.RoleServiceInterface
}

func NewRoleController(roleService services.RoleServiceInterface) RoleController {
	return &roleController{
		roleService: roleService,
	}
}

// Create godoc
// @Summary      Create a new role
// @Description  Creates a new role with name and optional description
// @Tags         roles
// @Accept       json
// @Produce      json
// @Param        body  body      role.CreateRoleRequest  true  "Role data"
// @Success      201   {object}  role.RoleResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/roles [post]
func (rc *roleController) Create(c *gin.Context) {
	var req role.CreateRoleRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := rc.roleService.Create(c, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el nombre es requerido" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		} else if errMsg == "el nombre del rol ya existe" {
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

// Update godoc
// @Summary      Update a role
// @Description  Updates a role by ID with optional fields
// @Tags         roles
// @Accept       json
// @Produce      json
// @Param        id    path      int                    true  "Role ID"
// @Param        body  body      role.UpdateRoleRequest  true  "Fields to update"
// @Success      200   {object}  role.RoleResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/roles/{id} [put]
func (rc *roleController) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "id debe ser un número válido",
		})
		return
	}

	var req role.UpdateRoleRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := rc.roleService.Update(c, id, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "rol no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el nombre no puede estar vacío" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		} else if errMsg == "el nombre del rol ya existe" {
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

// GetByID godoc
// @Summary      Get role by ID
// @Description  Get a role by its ID
// @Tags         roles
// @Produce      json
// @Param        id   path      int  true  "Role ID"
// @Success      200  {object}  role.RoleResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/roles/{id} [get]
func (rc *roleController) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "id debe ser un número válido",
		})
		return
	}

	response, err := rc.roleService.GetByID(c, id)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "rol no encontrado" {
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

// GetByName godoc
// @Summary      Get role by name
// @Description  Get a role by its unique name
// @Tags         roles
// @Produce      json
// @Param        name  query     string  true  "Role name"
// @Success      200   {object}  role.RoleResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Router       /api/v1/roles/by-name [get]
func (rc *roleController) GetByName(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "el parámetro name es requerido",
		})
		return
	}

	response, err := rc.roleService.GetByName(c, name)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "rol no encontrado" {
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

// GetAll godoc
// @Summary      Get all roles
// @Description  Get all active roles
// @Tags         roles
// @Produce      json
// @Success      200  {array}   role.RoleResponse
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/roles [get]
func (rc *roleController) GetAll(c *gin.Context) {
	response, err := rc.roleService.GetAll(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Delete godoc
// @Summary      Delete a role
// @Description  Soft deletes a role by ID
// @Tags         roles
// @Produce      json
// @Param        id    path      int  true  "Role ID"
// @Success      200   {object}  role.DeleteRoleResponse
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/roles/{id} [delete]
func (rc *roleController) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "id debe ser un número válido",
		})
		return
	}

	response, err := rc.roleService.Delete(c, id)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "rol no encontrado" {
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
