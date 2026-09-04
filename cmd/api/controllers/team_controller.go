package controllers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/delegates"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// TeamController define las operaciones HTTP para equipos.
type TeamController interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	GetByID(c *gin.Context)
	GetAll(c *gin.Context)
	UpdateAddress(c *gin.Context)
	UploadIcon(c *gin.Context)
	DeleteIcon(c *gin.Context)
}

type teamController struct {
	teamService  services.TeamServiceInterface
	teamDelegate delegates.TeamDelegate
}

// NewTeamController crea una nueva instancia de TeamController.
func NewTeamController(teamService services.TeamServiceInterface, teamDelegate delegates.TeamDelegate) TeamController {
	return &teamController{
		teamService:  teamService,
		teamDelegate: teamDelegate,
	}
}

// Create godoc
// @Summary      Crear equipo
// @Description  Crea un nuevo equipo. El owner debe tener el rol "entrenador"
// @Tags         teams
// @Accept       json
// @Produce      json
// @Param        body  body      team.CreateTeamRequest  true  "Datos del equipo"
// @Success      201   {object}  team.TeamResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams [post]
func (tc *teamController) Create(c *gin.Context) {
	var req team.CreateTeamRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	ownerID, _ := utils.GetAuthUserID(c)
	response, err := tc.teamDelegate.CreateTeam(c, ownerID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el usuario owner no existe" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el owner debe tener el rol 'entrenador'" {
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

// Update godoc
// @Summary      Actualizar equipo
// @Description  Actualiza los campos de un equipo existente. Solo el entrenador del equipo puede hacerlo
// @Tags         teams
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "Team ID"
// @Param        body  body      team.UpdateTeamRequest   true  "Campos a actualizar"
// @Success      200   {object}  team.TeamResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id} [put]
func (tc *teamController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	var req team.UpdateTeamRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := tc.teamService.Update(c, id, callerID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador puede actualizar el equipo" {
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
// @Summary      Eliminar equipo
// @Description  Elimina lógicamente un equipo. Solo el entrenador puede hacerlo y no debe tener miembros
// @Tags         teams
// @Produce      json
// @Param        id  path  int  true  "Team ID"
// @Success      200   {object}  team.DeleteTeamResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id} [delete]
func (tc *teamController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	userID, _ := utils.GetAuthUserID(c)

	if err := tc.teamService.Delete(c, id, userID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" || errMsg == "el usuario no pertenece a este equipo" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador puede eliminar el equipo" {
			statusCode = http.StatusForbidden
			code = "Forbidden"
		} else if errMsg == "no se puede eliminar un equipo con miembros activos" {
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

	c.JSON(http.StatusOK, team.DeleteTeamResponse{Message: "Equipo eliminado correctamente"})
}

// GetByID godoc
// @Summary      Obtener equipo por ID
// @Description  Devuelve un equipo por su ID
// @Tags         teams
// @Produce      json
// @Param        id    path      int  true  "Team ID"
// @Success      200   {object}  team.TeamResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id} [get]
func (tc *teamController) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	response, err := tc.teamService.GetByID(c, id)
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

// GetAll godoc
// @Summary      Listar equipos
// @Description  Devuelve equipos activos. Sin filtros, todos. owner_id filtra por equipos administrados, member_id por equipos donde el usuario es miembro
// @Tags         teams
// @Produce      json
// @Param        owner_id   query     int  false  "Filtrar por owner"
// @Param        member_id  query     int  false  "Filtrar por miembro"
// @Success      200  {array}   team.TeamResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/teams [get]
func (tc *teamController) GetAll(c *gin.Context) {
	var ownerID *int64
	var memberID *int64

	if oid := c.Query("owner_id"); oid != "" {
		parsed, err := strconv.ParseInt(oid, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "owner_id debe ser un número válido",
			})
			return
		}
		ownerID = &parsed
	}

	if mid := c.Query("member_id"); mid != "" {
		parsed, err := strconv.ParseInt(mid, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "member_id debe ser un número válido",
			})
			return
		}
		memberID = &parsed
	}

	response, err := tc.teamService.GetAll(c, ownerID, memberID)
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

// UpdateAddress godoc
// @Summary      Actualizar dirección del equipo
// @Description  Actualiza la dirección de un equipo mediante un endpoint dedicado. Solo el entrenador del equipo puede hacerlo
// @Tags         teams
// @Accept       json
// @Produce      json
// @Param        id    path      int                             true  "Team ID"
// @Param        body  body      team.UpdateTeamAddressRequest   true  "Dirección del equipo"
// @Success      200   {object}  team.TeamResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/address [put]
func (tc *teamController) UpdateAddress(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	var req team.UpdateTeamAddressRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	response, err := tc.teamService.UpdateAddress(c, id, callerID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "equipo no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "solo el entrenador puede actualizar el equipo" {
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

// UploadIcon godoc
// @Summary      Subir ícono del equipo
// @Description  Sube o reemplaza el ícono del equipo. Solo el entrenador dueño puede hacerlo. Max 5MB, JPEG/PNG/WEBP
// @Tags         teams
// @Accept       multipart/form-data
// @Produce      json
// @Param        id     path      int   true  "Team ID"
// @Param        photo  formData  file  true  "Icon file"
// @Success      200    {object}  map[string]string
// @Failure      400    {object}  apierror.APIError
// @Failure      403    {object}  apierror.APIError
// @Failure      500    {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/icon [put]
func (tc *teamController) UploadIcon(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxPhotoSizeBytes+1024)
	fileHeader, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "PHOTO_TOO_LARGE",
			Message:    "Archivo inválido o demasiado grande (máximo 5MB)",
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "No se pudo leer el archivo",
		})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "No se pudo leer el archivo",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	iconURL, err := tc.teamService.UploadIcon(c, id, callerID, content)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errors.Is(err, services.ErrPhotoTooLarge) {
			statusCode = http.StatusBadRequest
			code = "PHOTO_TOO_LARGE"
		} else if errors.Is(err, services.ErrPhotoInvalidType) {
			statusCode = http.StatusBadRequest
			code = "PHOTO_INVALID_TYPE"
		} else if errors.Is(err, services.ErrStorageUnavailable) {
			code = "STORAGE_UNAVAILABLE"
		} else if errMsg == "solo el entrenador dueño del equipo puede cambiar el ícono" {
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

	c.JSON(http.StatusOK, gin.H{"icon_url": iconURL})
}

// DeleteIcon godoc
// @Summary      Borrar ícono del equipo
// @Description  Borra el ícono del equipo. Solo el entrenador dueño puede hacerlo. Idempotente
// @Tags         teams
// @Param        id  path  int  true  "Team ID"
// @Success      204
// @Failure      400  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/icon [delete]
func (tc *teamController) DeleteIcon(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team id debe ser un número válido",
		})
		return
	}

	callerID, _ := utils.GetAuthUserID(c)
	if err := tc.teamService.DeleteIcon(c, id, callerID); err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "solo el entrenador dueño del equipo puede cambiar el ícono" {
			statusCode = http.StatusForbidden
			code = "Forbidden"
		} else if errors.Is(err, services.ErrStorageUnavailable) {
			code = "STORAGE_UNAVAILABLE"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.Status(http.StatusNoContent)
}
