package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/tiersubscription"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type TierSubscriptionController interface {
	ChangeTier(c *gin.Context)
	GetCurrentSubscription(c *gin.Context)
}

type tierSubscriptionController struct {
	tierSubscriptionService services.TierSubscriptionServiceInterface
}

func NewTierSubscriptionController(tierSubscriptionService services.TierSubscriptionServiceInterface) TierSubscriptionController {
	return &tierSubscriptionController{
		tierSubscriptionService: tierSubscriptionService,
	}
}

// forbiddenNotSelfSubscription centraliza el 403 self-only, idéntico al criterio
// de user_role_controller: nadie cambia el tier ni consulta la suscripción de otro.
func forbiddenNotSelfSubscription(c *gin.Context) {
	c.JSON(http.StatusForbidden, apierror.APIError{
		StatusCode: http.StatusForbidden,
		Code:       "Forbidden",
		Message:    "solo podés gestionar tus propios roles",
	})
}

// ChangeTier godoc
// @Summary      Change tier of a role subscription
// @Description  Changes the tier of an assigned role, blocking if there is debt or a pending first payment.
// @Tags         user-roles
// @Accept       json
// @Produce      json
// @Param        id      path      int                            true  "User ID"
// @Param        role_id path      int                            true  "Role ID"
// @Param        body    body      tiersubscription.ChangeTierRequest  true  "Target tier"
// @Success      200     {object}  tiersubscription.ChangeTierResponse
// @Failure      400     {object}  apierror.APIError
// @Failure      403     {object}  apierror.APIError
// @Failure      404     {object}  apierror.APIError
// @Failure      409     {object}  apierror.APIError
// @Failure      500     {object}  apierror.APIError
// @Router       /api/v1/users/{id}/roles/{role_id}/tier [put]
func (tsc *tierSubscriptionController) ChangeTier(c *gin.Context) {
	userID, ok := parseUserIDFromPath(c)
	if !ok {
		return
	}
	roleID, ok := parseRoleIDFromPath(c)
	if !ok {
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelfSubscription(c)
		return
	}

	var req tiersubscription.ChangeTierRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := tsc.tierSubscriptionService.ChangeTier(c, userID, roleID, &req)
	if err != nil {
		statusCode, code := mapTierSubscriptionError(err)
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCurrentSubscription godoc
// @Summary      Get current subscription of a role
// @Description  Returns the current subscription and next installment to pay for the role (Bricks checkout data). Free roles return tier/role only.
// @Tags         user-roles
// @Produce      json
// @Param        id      path      int  true  "User ID"
// @Param        role_id query     int  true  "Role ID"
// @Success      200     {object}  tiersubscription.CurrentSubscriptionResponse
// @Failure      400     {object}  apierror.APIError
// @Failure      403     {object}  apierror.APIError
// @Failure      404     {object}  apierror.APIError
// @Failure      500     {object}  apierror.APIError
// @Router       /api/v1/users/{id}/subscriptions/current [get]
func (tsc *tierSubscriptionController) GetCurrentSubscription(c *gin.Context) {
	userID, ok := parseUserIDFromPath(c)
	if !ok {
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		forbiddenNotSelfSubscription(c)
		return
	}

	roleIDStr := c.Query("role_id")
	if roleIDStr == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "el parámetro role_id es requerido",
		})
		return
	}
	roleID, err := strconv.ParseInt(roleIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "role_id debe ser un número válido",
		})
		return
	}

	response, err := tsc.tierSubscriptionService.GetCurrentSubscription(c, userID, roleID)
	if err != nil {
		statusCode, code := mapTierSubscriptionError(err)
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func parseUserIDFromPath(c *gin.Context) (int64, bool) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "user id debe ser un número válido",
		})
		return 0, false
	}
	return userID, true
}

func parseRoleIDFromPath(c *gin.Context) (int64, bool) {
	roleIDStr := c.Param("role_id")
	roleID, err := strconv.ParseInt(roleIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "role_id debe ser un número válido",
		})
		return 0, false
	}
	return roleID, true
}

// mapTierSubscriptionError traduce los errores de dominio de la suscripción al
// status y custom code del DTO apierror.APIError (D11): los códigos tipificados
// están en domains/constants/error_code.go.
func mapTierSubscriptionError(err error) (statusCode int, code string) {
	errMsg := err.Error()

	switch {
	case errMsg == "el usuario no tiene asignado este rol":
		return http.StatusNotFound, "Not Found"
	case errMsg == "tier no encontrado":
		return http.StatusNotFound, constants.ErrorCodeTierNotFound
	case errMsg == "el tier no pertenece al rol especificado":
		return http.StatusBadRequest, constants.ErrorCodeTierRoleMismatch
	case errMsg == "no podés cambiar de tier con deuda pendiente":
		return http.StatusConflict, constants.ErrorCodeDebtBlocksOperation
	case errMsg == "no podés cambiar de tier con el primer pago pendiente":
		return http.StatusConflict, constants.ErrorCodeSubscriptionPendingFirstPayment
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}
