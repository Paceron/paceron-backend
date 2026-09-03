package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type PaymentController interface {
	CreatePreference(c *gin.Context)
	ProcessPayment(c *gin.Context)
	GetPaymentStatus(c *gin.Context)
	GetPaymentStatusFromMP(c *gin.Context)
	HandleWebhook(c *gin.Context)
	GenerateTestCardToken(c *gin.Context)
}

type paymentController struct {
	paymentService services.PaymentServiceInterface
}

func NewPaymentController(paymentService services.PaymentServiceInterface) PaymentController {
	return &paymentController{
		paymentService: paymentService,
	}
}

// CreatePreference godoc
// @Summary      Create Mercado Pago preference
// @Description  Creates a preference for Checkout Brick (Status Screen / Card Form)
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        body body      payment.CreatePreferenceRequest true "Preference data"
// @Success      201  {object}  payment.CreatePreferenceResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/preference [post]
func (pc *paymentController) CreatePreference(c *gin.Context) {
	var req payment.CreatePreferenceRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid preference request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if req.Items == nil || len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Items son requeridos",
		})
		return
	}

	for i, item := range req.Items {
		if item.Title == "" {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "Título del item " + strconv.Itoa(i+1) + " es requerido",
			})
			return
		}
		if item.Quantity <= 0 {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "Cantidad del item " + strconv.Itoa(i+1) + " debe ser mayor a 0",
			})
			return
		}
		if item.UnitPrice <= 0 {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "Precio del item " + strconv.Itoa(i+1) + " debe ser mayor a 0",
			})
			return
		}
	}

	if req.Description == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Descripción es requerida",
		})
		return
	}

	if req.Concept == "" {
		req.Concept = "order"
	}

	resp, err := pc.paymentService.CreatePreference(c, req)
	if err != nil {
		customlogger.Error(c, "error creating preference", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal server error",
			Message:    "Error al crear la preferencia",
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ProcessPayment godoc
// @Summary      Process a card payment
// @Description  Processes a payment with card token from Card Form Brick
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        body body      payment.ProcessPaymentRequest true "Payment data"
// @Success      200  {object}  payment.PaymentResponse
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments [post]
func (pc *paymentController) ProcessPayment(c *gin.Context) {
	var req payment.ProcessPaymentRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid payment request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if req.Token == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Token de pago es requerido",
		})
		return
	}

	if req.TransactionAmount <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Monto del pago debe ser mayor a 0",
		})
		return
	}

	if req.PaymentMethodID == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Método de pago es requerido",
		})
		return
	}

	if req.Installments <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuotas deben ser mayores a 0",
		})
		return
	}

	if req.PayerEmail == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Email del pagador es requerido",
		})
		return
	}

	resp, err := pc.paymentService.ProcessPayment(c, req)
	if err != nil {
		statusCode, code := mapPaymentError(err)
		if statusCode == http.StatusInternalServerError {
			customlogger.Error(c, "error processing payment", err)
		}
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    "Error al procesar el pago",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPaymentStatus godoc
// @Summary      Get payment status
// @Description  Get the current status of a payment by its ID
// @Tags         payments
// @Produce      json
// @Param        id   path      int true "Payment ID"
// @Success      200  {object}  payment.PaymentResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/{id} [get]
func (pc *paymentController) GetPaymentStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de pago inválido",
		})
		return
	}

	resp, err := pc.paymentService.GetPaymentStatus(c, id)
	if err != nil {
		if err.Error() == "payment not found" {
			c.JSON(http.StatusNotFound, apierror.APIError{
				StatusCode: http.StatusNotFound,
				Code:       "Not found",
				Message:    "Pago no encontrado",
			})
			return
		}
		customlogger.Error(c, "error getting payment status", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal server error",
			Message:    "Error al obtener estado del pago",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPaymentStatusFromMP godoc
// @Summary      Get payment status from Mercado Pago
// @Description  Fetches the payment status directly from Mercado Pago API
// @Tags         payments
// @Produce      json
// @Param        id   path      string true "Mercado Pago Payment ID"
// @Success      200  {object}  payment.MPPaymentStatusResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/mp/{id} [get]
func (pc *paymentController) GetPaymentStatusFromMP(c *gin.Context) {
	mpPaymentID := c.Param("id")
	if mpPaymentID == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de pago de MP requerido",
		})
		return
	}

	resp, err := pc.paymentService.GetPaymentStatusFromMP(c, mpPaymentID)
	if err != nil {
		customlogger.Error(c, "error getting payment status from MP", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal server error",
			Message:    "Error al obtener estado del pago de MP",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// HandleWebhook godoc
// @Summary      Handle Mercado Pago webhook
// @Description  Receives and processes payment notifications from Mercado Pago
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        body body      payment.WebhookNotification true "Webhook notification"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  apierror.APIError
// @Router       /api/v1/payments/webhook [post]
func (pc *paymentController) HandleWebhook(c *gin.Context) {
	var notification payment.WebhookNotification
	if err := c.BindJSON(&notification); err != nil {
		customlogger.Warn(c, "invalid webhook body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de webhook inválido",
		})
		return
	}

	xSignature := c.GetHeader("X-Signature")
	xRequestID := c.GetHeader("X-Request-Id")

	if xSignature == "" || xRequestID == "" {
		customlogger.Warn(c, "webhook missing signature headers",
			customlogger.Tag("x-signature", xSignature),
			customlogger.Tag("x-request-id", xRequestID))
		c.JSON(http.StatusOK, gin.H{"message": "missing signature headers"})
		return
	}

	if notification.Type != "payment" {
		c.JSON(http.StatusOK, gin.H{"message": "ignored"})
		return
	}

	if err := pc.paymentService.HandleWebhook(c, notification); err != nil {
		customlogger.Error(c, "error handling webhook", err)
		c.JSON(http.StatusOK, gin.H{"message": "error processing"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GenerateTestCardToken godoc
// @Summary      Generate test card token
// @Description  Generates a card token for testing (sandbox only)
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        body body      payment.TestCardTokenRequest true "Card details"
// @Success      200  {object}  payment.TestCardTokenResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/payments/test-card-token [post]
func (pc *paymentController) GenerateTestCardToken(c *gin.Context) {
	var req payment.TestCardTokenRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid test card token request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	resp, err := pc.paymentService.GenerateTestCardToken(c, req)
	if err != nil {
		customlogger.Error(c, "error generating test card token", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal server error",
			Message:    "Error al generar token de prueba",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// mapPaymentError traduce los errores de dominio del pago al status y custom
// code del DTO apierror.APIError. Los códigos tipificados están en
// domains/constants/error_code.go.
func mapPaymentError(err error) (statusCode int, code string) {
	switch err.Error() {
	case "cuota de suscripcion no encontrada":
		return http.StatusNotFound, constants.ErrorCodePaymentInstallmentNotFound
	case "la cuota no pertenece al usuario autenticado":
		return http.StatusForbidden, constants.ErrorCodePaymentInstallmentForbidden
	case "usuario autenticado no encontrado en el contexto":
		return http.StatusUnauthorized, "Unauthorized"
	default:
		return http.StatusInternalServerError, "Internal server error"
	}
}
