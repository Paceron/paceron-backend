package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/permission"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type PermissionController interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	GetByID(c *gin.Context)
	GetByName(c *gin.Context)
	GetAll(c *gin.Context)
}

type permissionController struct {
	permissionService services.PermissionServiceInterface
}

func NewPermissionController(permissionService services.PermissionServiceInterface) PermissionController {
	return &permissionController{
		permissionService: permissionService,
	}
}

// Create godoc
// @Summary      Create a new permission
// @Description  Creates a new permission with name and optional description
// @Tags         permissions
// @Accept       json
// @Produce      json
// @Param        body  body      permission.CreatePermissionRequest  true  "Permission data"
// @Success      201   {object}  permission.PermissionResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/permissions [post]
func (pc *permissionController) Create(c *gin.Context) {
	var req permission.CreatePermissionRequest
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

	response, err := pc.permissionService.Create(c, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el nombre es requerido" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		} else if errMsg == "el nombre del permiso ya existe" {
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
// @Summary      Update a permission
// @Description  Updates a permission by ID with optional fields
// @Tags         permissions
// @Accept       json
// @Produce      json
// @Param        id    path      int                              true  "Permission ID"
// @Param        body  body      permission.UpdatePermissionRequest  true  "Fields to update"
// @Success      200   {object}  permission.PermissionResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/permissions/{id} [put]
func (pc *permissionController) Update(c *gin.Context) {
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

	var req permission.UpdatePermissionRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := pc.permissionService.Update(c, id, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "permiso no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el nombre no puede estar vacío" || errMsg == "el nombre del permiso ya existe" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
			if errMsg == "el nombre del permiso ya existe" {
				statusCode = http.StatusConflict
				code = "Conflict"
			}
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
// @Summary      Get permission by ID
// @Description  Get a permission by its ID
// @Tags         permissions
// @Produce      json
// @Param        id   path      int  true  "Permission ID"
// @Success      200  {object}  permission.PermissionResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/permissions/{id} [get]
func (pc *permissionController) GetByID(c *gin.Context) {
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

	response, err := pc.permissionService.GetByID(c, id)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "permiso no encontrado" {
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
// @Summary      Get permission by name
// @Description  Get a permission by its unique name
// @Tags         permissions
// @Produce      json
// @Param        name  query     string  true  "Permission name"
// @Success      200   {object}  permission.PermissionResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Router       /api/v1/permissions/by-name [get]
func (pc *permissionController) GetByName(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "el parámetro name es requerido",
		})
		return
	}

	response, err := pc.permissionService.GetByName(c, name)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "permiso no encontrado" {
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
// @Summary      Get all permissions
// @Description  Get all active permissions
// @Tags         permissions
// @Produce      json
// @Success      200  {array}   permission.PermissionResponse
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/permissions [get]
func (pc *permissionController) GetAll(c *gin.Context) {
	response, err := pc.permissionService.GetAll(c)
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
// @Summary      Delete a permission
// @Description  Soft deletes a permission by ID
// @Tags         permissions
// @Produce      json
// @Param        id    path      int  true  "Permission ID"
// @Success      200   {object}  permission.DeletePermissionResponse
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/permissions/{id} [delete]
func (pc *permissionController) Delete(c *gin.Context) {
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

	response, err := pc.permissionService.Delete(c, id)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "permiso no encontrado" {
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
