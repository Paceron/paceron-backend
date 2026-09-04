package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/platformsettings"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// PlatformSettingControllerInterface define los endpoints de configuración global.
type PlatformSettingControllerInterface interface {
	GetMarketplaceFee(c *gin.Context)
	UpdateMarketplaceFee(c *gin.Context)
}

type platformSettingController struct {
	service services.PlatformSettingServiceInterface
}

func NewPlatformSettingController(svc services.PlatformSettingServiceInterface) PlatformSettingControllerInterface {
	return &platformSettingController{service: svc}
}

// GetMarketplaceFee godoc
// @Summary      Get marketplace fee
// @Description  Returns the current marketplace fee percentage.
// @Tags         platform-settings
// @Accept       json
// @Produce      json
// @Success      200      {object}  platformsettings.MarketplaceFeeResponse
// @Failure      500      {object}  apierror.APIError
// @Router       /api/v1/platform-settings/marketplace-fee [get]
func (c *platformSettingController) GetMarketplaceFee(ctx *gin.Context) {
	resp, err := c.service.GetMarketplaceFee(ctx)
	if err != nil {
		statusCode, code := mapPlatformSettingError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// UpdateMarketplaceFee godoc
// @Summary      Update marketplace fee
// @Description  Updates the marketplace fee percentage (admin/owner only).
// @Tags         platform-settings
// @Accept       json
// @Produce      json
// @Param        body    body      platformsettings.UpdateMarketplaceFeeRequest  true  "New fee percentage"
// @Success      200     {object}  platformsettings.MarketplaceFeeResponse
// @Failure      400     {object}  apierror.APIError
// @Failure      401     {object}  apierror.APIError
// @Failure      403     {object}  apierror.APIError
// @Failure      500     {object}  apierror.APIError
// @Router       /api/v1/platform-settings/marketplace-fee [put]
func (c *platformSettingController) UpdateMarketplaceFee(ctx *gin.Context) {
	userID, ok := utils.GetAuthUserID(ctx)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "Unauthorized",
			Message:    "no autenticado",
		})
		return
	}

	var req platformsettings.UpdateMarketplaceFeeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "cuerpo inválido",
		})
		return
	}

	resp, err := c.service.UpdateMarketplaceFee(ctx, userID, &req)
	if err != nil {
		statusCode, code := mapPlatformSettingError(err)
		ctx.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// mapPlatformSettingError traduce los errores del servicio a status/code de apierror.
func mapPlatformSettingError(err error) (int, string) {
	errMsg := err.Error()

	switch {
	case errMsg == "comisión debe estar entre 0 y 100":
		return http.StatusBadRequest, "Bad request"
	case errMsg == "no tenés permisos para actualizar la configuración":
		return http.StatusForbidden, constants.ErrorCodeNotAppOwner
	case errMsg == "error consultando comisión":
		return http.StatusInternalServerError, "Internal Server Error"
	case errMsg == "error actualizando comisión":
		return http.StatusInternalServerError, "Internal Server Error"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}