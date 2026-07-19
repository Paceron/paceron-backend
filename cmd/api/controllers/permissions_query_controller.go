package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/services"
)

type PermissionsQueryController interface {
	GetUserPermissions(c *gin.Context)
}

type permissionsQueryController struct {
	permissionsQueryService services.PermissionsQueryServiceInterface
}

func NewPermissionsQueryController(permissionsQueryService services.PermissionsQueryServiceInterface) PermissionsQueryController {
	return &permissionsQueryController{
		permissionsQueryService: permissionsQueryService,
	}
}

// GetUserPermissions godoc
// @Summary      Get user permissions
// @Description  Returns all permissions for a user grouped by role and tier
// @Tags         permissions
// @Produce      json
// @Param        user_id  query     int  true  "User ID"
// @Success      200      {object}  services.PermissionsQueryResponse
// @Failure      400      {object}  apierror.APIError
// @Failure      404      {object}  apierror.APIError
// @Failure      500      {object}  apierror.APIError
// @Router       /api/v1/auth/permissions [get]
func (pqc *permissionsQueryController) GetUserPermissions(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "El parámetro user_id es requerido",
		})
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user_id debe ser un número válido",
		})
		return
	}

	response, err := pqc.permissionsQueryService.GetUserPermissions(c, userID)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "usuario no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if strings.HasPrefix(errMsg, "datos faltantes") {
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
