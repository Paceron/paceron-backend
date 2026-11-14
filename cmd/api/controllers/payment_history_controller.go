package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// artZone es la zona fija de Argentina, la misma que usa el resumen para cortar
// los meses (no hay horario de verano desde 2009).
var artZone = time.FixedZone("ART", -3*3600)

// PaymentHistoryController expone el historial de pagos y cobros del usuario
// autenticado (change historial-pagos-cobros-entrenador). La identidad sale
// siempre del token: no hay :id en el path.
type PaymentHistoryController interface {
	ListReceived(c *gin.Context)
	GetReceivedSummary(c *gin.Context)
	ListHistory(c *gin.Context)
	GetUpcoming(c *gin.Context)
}

// Los tipos de respuesta se nombran acá para que swag los resuelva: los
// handlers devuelven lo que arma el service y no los referencian en el código.
var (
	_ *payment.ReceivedPaymentsResponse
	_ *payment.ReceivedSummaryResponse
	_ *payment.HistoryPaymentsResponse
	_ *payment.UpcomingPaymentsResponse
)

type paymentHistoryController struct {
	service services.PaymentHistoryServiceInterface
}

func NewPaymentHistoryController(service services.PaymentHistoryServiceInterface) PaymentHistoryController {
	return &paymentHistoryController{service: service}
}

// ListReceived godoc
// @Summary      Listar cobros recibidos
// @Description  Cobros de membresía de equipo recibidos por el usuario autenticado, del más reciente al más antiguo. Paginado de a 20. `net_amount` es el bruto menos la comisión de Paceron (`fee_amount`, `fee_percent`) y es null si el cobro no está aprobado.
// @Tags         payments
// @Produce      json
// @Param        page     query     int     false  "Página (default 1)"
// @Param        team_id  query     int     false  "Filtrar por equipo"
// @Param        payer_id query     int     false  "Filtrar por corredor (payer)"
// @Param        from     query     string  false  "Desde (YYYY-MM-DD, hora argentina, inclusivo)"
// @Param        to       query     string  false  "Hasta (YYYY-MM-DD, hora argentina, inclusivo)"
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
	teamID, ok := parseOptionalIntQuery(c, "team_id")
	if !ok {
		return
	}
	payerID, ok := parseOptionalIntQuery(c, "payer_id")
	if !ok {
		return
	}
	from, ok := parseOptionalDateQuery(c, "from")
	if !ok {
		return
	}
	to, ok := parseOptionalDateQuery(c, "to")
	if !ok {
		return
	}
	if from != nil && to != nil && to.Before(*from) {
		respondInvalidQuery(c, "el rango de fechas está invertido")
		return
	}
	// `to` es inclusivo en la API y exclusivo en la consulta: se suma un día.
	var toExclusive *time.Time
	if to != nil {
		next := to.AddDate(0, 0, 1)
		toExclusive = &next
	}

	filter := payment.ReceivedFilter{
		TeamID:      teamID,
		PayerID:     payerID,
		StatusGroup: c.Query("status"),
		From:        from,
		To:          toExclusive,
	}
	resp, err := pc.service.ListReceived(c, userID, filter, page)
	if err != nil {
		respondPaymentHistoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// parseOptionalIntQuery parsea un entero opcional. Devuelve (nil, true) cuando
// el parámetro no vino.
func parseOptionalIntQuery(c *gin.Context, key string) (*int64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		respondInvalidQuery(c, key+" debe ser un número válido")
		return nil, false
	}
	return &parsed, true
}

// parseOptionalDateQuery parsea una fecha YYYY-MM-DD en hora argentina. El
// listado de cobros usa el mismo día calendario que ve el entrenador.
func parseOptionalDateQuery(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, artZone)
	if err != nil {
		respondInvalidQuery(c, key+" debe tener formato YYYY-MM-DD")
		return nil, false
	}
	return &parsed, true
}

// GetReceivedSummary godoc
// @Summary      Resumen de cobros recibidos
// @Description  Totales mensuales (hora argentina; el último es `until` o, sin él, el mes actual), totales por equipo y cantidad de cuotas pendientes o rechazadas según su último intento, dentro de la ventana. `earliest_month` es el primer mes con cobros del vendedor. Cada mes suma además las cuotas esperadas sin pagar que vencen en él (`pending_*`), y `due_this_month` trae lo que vence desde hoy hasta fin de mes y `overdue`, todo lo que ya venció sin pagar, de este mes o de meses anteriores. `pending_periods` (obligaciones sin pagar) y `rejected_periods` (intentos rechazados) traen monto bruto, neto y cantidad de cuotas para los períodos `all` (vencidas + mes en curso), `current`, `last` y `quarter`; `payers` lista los corredores con cobros para el filtro del listado.
// @Tags         payments
// @Produce      json
// @Param        months  query     int     false  "Meses de la ventana, entre 2 y 12 (default 6)"
// @Param        until   query     string  false  "Último mes de la ventana (YYYY-MM), no posterior al actual (default: el mes actual)"
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

	resp, err := pc.service.GetReceivedSummary(c, userID, months, c.Query("until"))
	if err != nil {
		respondPaymentHistoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListHistory godoc
// @Summary      Historial de pagos del usuario
// @Description  Pagos que hizo el usuario autenticado, del más reciente al más antiguo: suscripciones de tier (`type=subscription`) y pagos a entrenadores por la membresía de un equipo (`type=trainer_payment`). Paginado de a 20.
// @Tags         payments
// @Produce      json
// @Param        page    query     int     false  "Página (default 1)"
// @Param        type    query     string  false  "Filtrar por tipo"  Enums(subscription, trainer_payment)
// @Param        status  query     string  false  "Filtrar por grupo de estado"  Enums(approved, pending, rejected, refunded)
// @Success      200  {object}  payment.HistoryPaymentsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/history [get]
func (pc *paymentHistoryController) ListHistory(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	page, ok := parseIntQuery(c, "page", 1)
	if !ok {
		return
	}

	resp, err := pc.service.ListHistory(c, userID, c.Query("type"), c.Query("status"), page)
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
	// El detalle queda en el log del service: el error envuelve el de la DB, y
	// devolverlo filtraría SQL al cliente.
	c.JSON(http.StatusInternalServerError, apierror.APIError{StatusCode: http.StatusInternalServerError, Code: "Internal Server Error", Message: "error interno"})
}

// GetUpcoming godoc
// @Summary      Próximos vencimientos del usuario
// @Description  Próxima cuota sin pagar de cada membresía de equipo con cuota (`type=trainer_payment`) y de cada suscripción de tier (`type=subscription`) del usuario autenticado, ordenadas por vencimiento. `overdue_count` cuenta las cuotas sin pagar que ya vencieron.
// @Tags         payments
// @Produce      json
// @Success      200  {object}  payment.UpcomingPaymentsResponse
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/upcoming [get]
func (pc *paymentHistoryController) GetUpcoming(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	resp, err := pc.service.GetUpcoming(c, userID)
	if err != nil {
		respondPaymentHistoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
