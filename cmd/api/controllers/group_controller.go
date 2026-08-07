package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/group"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// GroupController define las operaciones HTTP para grupos.
type GroupController interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	GetByID(c *gin.Context)
	GetAll(c *gin.Context)
}

type groupController struct {
	groupService services.GroupServiceInterface
}

// NewGroupController crea una nueva instancia de GroupController.
func NewGroupController(groupService services.GroupServiceInterface) GroupController {
	return &groupController{
		groupService: groupService,
	}
}

// Create godoc
// @Summary      Crear grupo
// @Description  Crea un nuevo grupo dentro de un equipo. Solo el entrenador del equipo puede hacerlo
// @Tags         groups
// @Accept       json
// @Produce      json
// @Param        body  body      group.CreateGroupRequest  true  "Datos del grupo"
// @Success      201   {object}  group.GroupResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/groups [post]
func (gc *groupController) Create(c *gin.Context) {
	var req group.CreateGroupRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := gc.groupService.Create(c, callerID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el equipo no existe" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador del equipo puede crear grupos" {
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

// Update godoc
// @Summary      Actualizar grupo
// @Description  Actualiza los campos de un grupo existente. Solo el entrenador del equipo puede hacerlo
// @Tags         groups
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "Group ID"
// @Param        body  body      group.UpdateGroupRequest  true  "Campos a actualizar"
// @Success      200   {object}  group.GroupResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/groups/{id} [put]
func (gc *groupController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group id debe ser un número válido",
		})
		return
	}

	var req group.UpdateGroupRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := gc.groupService.Update(c, id, callerID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "grupo no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador puede actualizar el grupo" {
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

// Delete godoc
// @Summary      Eliminar grupo
// @Description  Elimina lógicamente un grupo. Solo el entrenador del equipo puede hacerlo
// @Tags         groups
// @Produce      json
// @Param        id  path  int  true  "Group ID"
// @Success      200   {object}  group.DeleteGroupResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/groups/{id} [delete]
func (gc *groupController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group id debe ser un número válido",
		})
		return
	}

	userID, _ := utils.GetAuthUserID(c)

	if err := gc.groupService.Delete(c, id, userID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "grupo no encontrado" || errMsg == "el usuario no pertenece al equipo de este grupo" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador puede eliminar el grupo" {
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

	c.JSON(http.StatusOK, group.DeleteGroupResponse{Message: "Grupo eliminado correctamente"})
}

// GetByID godoc
// @Summary      Obtener grupo por ID
// @Description  Devuelve un grupo por su ID
// @Tags         groups
// @Produce      json
// @Param        id    path      int  true  "Group ID"
// @Success      200   {object}  group.GroupResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/groups/{id} [get]
func (gc *groupController) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group id debe ser un número válido",
		})
		return
	}

	response, err := gc.groupService.GetByID(c, id)
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

// GetAll godoc
// @Summary      Listar grupos de un equipo
// @Description  Devuelve los grupos de un equipo. Si se filtra por team_id, valida que el usuario autenticado sea miembro
// @Tags         groups
// @Produce      json
// @Param        team_id  query     int  false  "ID del equipo"
// @Success      200  {array}   group.GroupResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/groups [get]
func (gc *groupController) GetAll(c *gin.Context) {
	var teamID *int64
	var userID *int64

	if tid := c.Query("team_id"); tid != "" {
		parsed, err := strconv.ParseInt(tid, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "team_id debe ser un número válido",
			})
			return
		}
		teamID = &parsed

		authUserID, _ := utils.GetAuthUserID(c)
		userID = &authUserID
	}

	response, err := gc.groupService.GetAll(c, teamID, userID)
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
