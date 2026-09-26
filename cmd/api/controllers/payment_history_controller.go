package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// PaymentHistoryController expone el historial de pagos y cobros del usuario
// autenticado (change historial-pagos-cobros-entrenador). La identidad sale
// siempre del token: no hay :id en el path.
type PaymentHistoryController interface {
	ListReceived(c *gin.Context)
	GetReceivedSummary(c *gin.Context)
	ListMine(c *gin.Context)
}

// Los tipos de respuesta se nombran acá para que swag los resuelva: los
// handlers devuelven lo que arma el service y no los referencian en el código.
var (
	_ *payment.ReceivedPaymentsResponse
	_ *payment.ReceivedSummaryResponse
	_ *payment.TierPaymentsResponse
)

type paymentHistoryController struct {
	service services.PaymentHistoryServiceInterface
}

func NewPaymentHistoryController(service services.PaymentHistoryServiceInterface) PaymentHistoryController {
	return &paymentHistoryController{service: service}
}

// ListReceived godoc
// @Summary      Listar cobros recibidos
// @Description  Cobros de membresía de equipo recibidos por el usuario autenticado, del más reciente al más antiguo. Paginado de a 20. `net_amount` es null salvo que Mercado Pago haya informado el neto real.
// @Tags         payments
// @Produce      json
// @Param        page     query     int     false  "Página (default 1)"
// @Param        team_id  query     int     false  "Filtrar por equipo"
// @Param        status   query     string  false  "Filtrar por grupo de estado"  Enums(approved, pending, rejected, refunded)
// @Success      200  {object}  payment.ReceivedPaymentsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/received [get]
func (pc *paymentHistoryController) ListReceived(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	page, ok := parseIntQuery(c, "page", 1)
	if !ok {
		return
	}
	var teamID *int64
	if raw := c.Query("team_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			respondInvalidQuery(c, "team_id debe ser un número válido")
			return
		}
		teamID = &parsed
	}

	resp, err := pc.service.ListReceived(c, userID, teamID, c.Query("status"), page)
	if err != nil {
		respondPaymentHistoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetReceivedSummary godoc
// @Summary      Resumen de cobros recibidos
// @Description  Totales mensuales (hora argentina, el último es el mes actual), totales por equipo y cantidad de cuotas pendientes o rechazadas según su último intento.
// @Tags         payments
// @Produce      json
// @Param        months  query     int  false  "Meses de la ventana, entre 2 y 12 (default 6)"
// @Success      200  {object}  payment.ReceivedSummaryResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/received/summary [get]
func (pc *paymentHistoryController) GetReceivedSummary(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	months, ok := parseIntQuery(c, "months", services.PaymentSummaryDefaultMonths)
	if !ok {
		return
	}

	resp, err := pc.service.GetReceivedSummary(c, userID, months)
	if err != nil {
		respondPaymentHistoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListMine godoc
// @Summary      Listar mis pagos de suscripción
// @Description  Pagos de suscripción de tier del usuario autenticado, del más reciente al más antiguo. Paginado de a 20.
// @Tags         payments
// @Produce      json
// @Param        page  query     int     false  "Página (default 1)"
// @Param        role  query     string  false  "Filtrar por rol del tier (ej. entrenador)"
// @Success      200  {object}  payment.TierPaymentsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/mine [get]
func (pc *paymentHistoryController) ListMine(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	page, ok := parseIntQuery(c, "page", 1)
	if !ok {
		return
	}

	resp, err := pc.service.ListMyTierPayments(c, userID, c.Query("role"), page)
	if err != nil {
		respondPaymentHistoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// parseIntQuery lee un parámetro entero opcional. Si no parsea, responde 400 y
// devuelve false; los rangos los valida el service.
func parseIntQuery(c *gin.Context, name string, def int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return def, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		respondInvalidQuery(c, name+" debe ser un número válido")
		return 0, false
	}
	return v, true
}

func respondInvalidQuery(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, apierror.APIError{StatusCode: http.StatusBadRequest, Code: "INVALID_QUERY", Message: message})
}

func respondUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, apierror.APIError{StatusCode: http.StatusUnauthorized, Code: "Unauthorized", Message: "no autenticado"})
}

func respondPaymentHistoryError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrInvalidPaymentHistoryQuery) {
		respondInvalidQuery(c, err.Error())
		return
	}
	c.JSON(http.StatusInternalServerError, apierror.APIError{StatusCode: http.StatusInternalServerError, Code: "Internal Server Error", Message: err.Error()})
}
