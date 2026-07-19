package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/tierpermission"
	"simple-arq-golang/cmd/api/services"
)

type TierPermissionController interface {
	Assign(c *gin.Context)
	Unassign(c *gin.Context)
}

type tierPermissionController struct {
	tierPermissionService services.TierPermissionServiceInterface
}

func NewTierPermissionController(tierPermissionService services.TierPermissionServiceInterface) TierPermissionController {
	return &tierPermissionController{
		tierPermissionService: tierPermissionService,
	}
}

// Assign godoc
// @Summary      Assign permission to tier
// @Description  Assigns a permission to a tier
// @Tags         tier-permissions
// @Accept       json
// @Produce      json
// @Param        id    path      int                                true  "Tier ID"
// @Param        body  body      tierpermission.AssignPermissionRequest  true  "Permission to assign"
// @Success      201   {object}  tierpermission.TierPermissionResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/tiers/{id}/permissions [post]
func (tpc *tierPermissionController) Assign(c *gin.Context) {
	tierIDStr := c.Param("id")
	tierID, err := strconv.ParseInt(tierIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "tier id debe ser un número válido",
		})
		return
	}

	var req tierpermission.AssignPermissionRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := tpc.tierPermissionService.Assign(c, tierID, &req)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "tier no encontrado" || errMsg == "permiso no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "el permiso ya está asignado a este tier" {
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

// Unassign godoc
// @Summary      Unassign permission from tier
// @Description  Soft deletes a permission assignment from a tier
// @Tags         tier-permissions
// @Produce      json
// @Param        id             path      int  true  "Tier ID"
// @Param        permission_id  path      int  true  "Permission ID"
// @Success      200            {object}  tierpermission.DeleteTierPermissionResponse
// @Failure      404            {object}  apierror.APIError
// @Failure      500            {object}  apierror.APIError
// @Router       /api/v1/tiers/{id}/permissions/{permission_id} [delete]
func (tpc *tierPermissionController) Unassign(c *gin.Context) {
	tierIDStr := c.Param("id")
	tierID, err := strconv.ParseInt(tierIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "tier id debe ser un número válido",
		})
		return
	}

	permIDStr := c.Param("permission_id")
	permID, err := strconv.ParseInt(permIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "permission_id debe ser un número válido",
		})
		return
	}

	response, err := tpc.tierPermissionService.Unassign(c, tierID, permID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "asignación no encontrada" {
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
