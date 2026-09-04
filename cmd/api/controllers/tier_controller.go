package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/tier"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type TierController interface {
	Create(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	GetByID(c *gin.Context)
	GetByName(c *gin.Context)
	GetAll(c *gin.Context)
}

type tierController struct {
	tierService services.TierServiceInterface
}

func NewTierController(tierService services.TierServiceInterface) TierController {
	return &tierController{
		tierService: tierService,
	}
}

// Create godoc
// @Summary      Create a new tier
// @Description  Creates a new tier associated with a role
// @Tags         tiers
// @Accept       json
// @Produce      json
// @Param        body  body      tier.CreateTierRequest  true  "Tier data"
// @Success      201   {object}  tier.TierResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/tiers [post]
func (tc *tierController) Create(c *gin.Context) {
	var req tier.CreateTierRequest
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

	response, err := tc.tierService.Create(c, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el nombre es requerido" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		} else if errMsg == "rol no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "ya existe un tier con ese nombre para este rol" {
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
// @Summary      Update a tier
// @Description  Updates a tier by ID with optional fields
// @Tags         tiers
// @Accept       json
// @Produce      json
// @Param        id    path      int                    true  "Tier ID"
// @Param        body  body      tier.UpdateTierRequest  true  "Fields to update"
// @Success      200   {object}  tier.TierResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/tiers/{id} [put]
func (tc *tierController) Update(c *gin.Context) {
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

	var req tier.UpdateTierRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := tc.tierService.Update(c, id, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "tier no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el nombre no puede estar vacío" {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		} else if errMsg == "ya existe un tier con ese nombre para este rol" {
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
// @Summary      Get tier by ID
// @Description  Get a tier by its ID
// @Tags         tiers
// @Produce      json
// @Param        id   path      int  true  "Tier ID"
// @Success      200  {object}  tier.TierResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/tiers/{id} [get]
func (tc *tierController) GetByID(c *gin.Context) {
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

	response, err := tc.tierService.GetByID(c, id)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "tier no encontrado" {
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
// @Summary      Get tier by name
// @Description  Get a tier by its name
// @Tags         tiers
// @Produce      json
// @Param        name  query     string  true  "Tier name"
// @Success      200   {object}  tier.TierResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Router       /api/v1/tiers/by-name [get]
func (tc *tierController) GetByName(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "el parámetro name es requerido",
		})
		return
	}

	response, err := tc.tierService.GetByName(c, name)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "tier no encontrado" {
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
// @Summary      Get all tiers
// @Description  Get all active tiers. Si se pasa role_id, devuelve solo los tiers de ese rol
// @Tags         tiers
// @Produce      json
// @Param        role_id  query     int  false  "Filtrar por rol (opcional)"
// @Success      200  {array}   tier.TierResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/tiers [get]
func (tc *tierController) GetAll(c *gin.Context) {
	var roleID *int64
	if roleIDStr := c.Query("role_id"); roleIDStr != "" {
		parsed, err := strconv.ParseInt(roleIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "role_id debe ser un número válido",
			})
			return
		}
		roleID = &parsed
	}

	response, err := tc.tierService.GetAll(c, roleID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "rol no encontrado" {
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

	c.JSON(http.StatusOK, response)
}

// Delete godoc
// @Summary      Delete a tier
// @Description  Soft deletes a tier by ID
// @Tags         tiers
// @Produce      json
// @Param        id    path      int  true  "Tier ID"
// @Success      200   {object}  tier.DeleteTierResponse
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/tiers/{id} [delete]
func (tc *tierController) Delete(c *gin.Context) {
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

	response, err := tc.tierService.Delete(c, id)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "tier no encontrado" {
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
