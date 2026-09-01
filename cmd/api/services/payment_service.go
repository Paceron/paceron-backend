package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/mercadopagoclient"
)

type PaymentServiceInterface interface {
	CreatePreference(ctx *gin.Context, req payment.CreatePreferenceRequest) (*payment.CreatePreferenceResponse, error)
	ProcessPayment(ctx *gin.Context, req payment.ProcessPaymentRequest) (*payment.PaymentResponse, error)
	GetPaymentStatus(ctx *gin.Context, paymentID int64) (*payment.PaymentResponse, error)
	GetPaymentStatusFromMP(ctx *gin.Context, mpPaymentID string) (*payment.MPPaymentStatusResponse, error)
	HandleWebhook(ctx *gin.Context, notification payment.WebhookNotification) error
	GenerateTestCardToken(ctx *gin.Context, req payment.TestCardTokenRequest) (*payment.TestCardTokenResponse, error)
}

type paymentService struct {
	paymentDao    daos.PaymentDaoInterface
	mpClient      mercadopagoclient.MercadoPagoClientInterface
	accessToken   string
	publicKey     string
	webhookSecret string
	currencyID    string
}

func NewPaymentService(
	paymentDao daos.PaymentDaoInterface,
	mpClient mercadopagoclient.MercadoPagoClientInterface,
) PaymentServiceInterface {
	return &paymentService{
		paymentDao:    paymentDao,
		mpClient:      mpClient,
		accessToken:   config.MyMP.AccessToken,
		publicKey:     config.MyMP.PublicKey,
		webhookSecret: config.MyMP.WebhookSecret,
		currencyID:    config.MyMP.CurrencyID,
	}
}

func (s *paymentService) CreatePreference(ctx *gin.Context, req payment.CreatePreferenceRequest) (*payment.CreatePreferenceResponse, error) {
	customlogger.Info(ctx, "creating MP preference", customlogger.TagMethod("CreatePreference"))

	amount := 0.0
	for _, item := range req.Items {
		amount += item.UnitPrice * float64(item.Quantity)
	}

	paymentRecord := &dbs.Payment{
		UserID:      req.SellerID,
		Concept:     req.Concept,
		Description: req.Description,
		Amount:      amount,
		CurrencyID:  s.currencyID,
		Status:      "pending",
		PayerEmail:  "",
	}

	if err := s.paymentDao.Create(ctx, paymentRecord); err != nil {
		customlogger.Error(ctx, "error creating payment record", err)
		return nil, fmt.Errorf("error creating payment record: %w", err)
	}

	externalRef := fmt.Sprintf("%d", paymentRecord.ID)
	if err := s.paymentDao.UpdateExternalRef(ctx, paymentRecord.ID, externalRef); err != nil {
		customlogger.Warn(ctx, "error updating external ref", customlogger.Tag("payment_id", fmt.Sprintf("%d", paymentRecord.ID)))
	}

	var items []mercadopagoclient.PreferenceItem
	for _, item := range req.Items {
		items = append(items, mercadopagoclient.PreferenceItem{
			Title:     item.Title,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		})
	}

	notificationURL := config.MyMP.WebhookURL
	preferenceID, err := s.mpClient.CreatePreference(ctx, s.accessToken, items, externalRef, notificationURL, "", s.currencyID)
	if err != nil {
		customlogger.Error(ctx, "error creating MP preference", err)
		return nil, fmt.Errorf("error creating MP preference: %w", err)
	}

	if err := s.paymentDao.UpdateRawResponse(ctx, paymentRecord.ID, fmt.Sprintf(`{"preference_id":"%s"}`, preferenceID)); err != nil {
		customlogger.Warn(ctx, "error updating raw response", customlogger.Tag("payment_id", fmt.Sprintf("%d", paymentRecord.ID)))
	}

	return &payment.CreatePreferenceResponse{
		PreferenceID: preferenceID,
		PublicKey:    s.publicKey,
	}, nil
}

func (s *paymentService) ProcessPayment(ctx *gin.Context, req payment.ProcessPaymentRequest) (*payment.PaymentResponse, error) {
	customlogger.Info(ctx, "processing MP payment", customlogger.TagMethod("ProcessPayment"))

	if req.PreferenceID != "" {
		existing, err := s.paymentDao.FindByExternalReference(ctx, req.PreferenceID)
		if err != nil {
			customlogger.Error(ctx, "error finding payment by preference ID", err)
			return nil, fmt.Errorf("error finding payment: %w", err)
		}
		if existing != nil {
			existing.Status = "in_process"
			if err := s.paymentDao.UpdateStatus(ctx, existing.ID, "in_process", ""); err != nil {
				customlogger.Error(ctx, "error updating payment status", err)
			}
			return s.mapPaymentResponse(existing), nil
		}
	}

	paymentRecord := &dbs.Payment{
		Concept:         "order",
		Description:     "Pago con tarjeta",
		Amount:          req.TransactionAmount,
		CurrencyID:      s.currencyID,
		Status:          "in_process",
		PayerEmail:      req.PayerEmail,
		PreferenceID:    req.PreferenceID,
		PaymentMethodID: req.PaymentMethodID,
		Installments:    req.Installments,
	}

	if err := s.paymentDao.Create(ctx, paymentRecord); err != nil {
		customlogger.Error(ctx, "error creating payment record", err)
		return nil, fmt.Errorf("error creating payment record: %w", err)
	}

	externalRef := fmt.Sprintf("%d", paymentRecord.ID)

	mpReq := mercadopagoclient.CreatePaymentRequest{
		Token:             req.Token,
		TransactionAmount: req.TransactionAmount,
		PaymentMethodID:   req.PaymentMethodID,
		Installments:      req.Installments,
		PayerEmail:        req.PayerEmail,
		Description:       paymentRecord.Description,
		ExternalReference: externalRef,
		NotificationURL:   config.MyMP.WebhookURL,
		ThreeDSecureMode:  "optional",
	}

	result, err := s.mpClient.CreatePayment(ctx, s.accessToken, mpReq)
	if err != nil {
		customlogger.Error(ctx, "error creating MP payment", err)
		return nil, fmt.Errorf("error creating MP payment: %w", err)
	}

	mpPaymentID := fmt.Sprintf("%d", result.ID)
	if err := s.paymentDao.UpdateStatus(ctx, paymentRecord.ID, result.Status, result.StatusDetail); err != nil {
		customlogger.Error(ctx, "error updating payment status", err)
	}

	rawResp, _ := json.Marshal(result)
	if err := s.paymentDao.UpdateRawResponse(ctx, paymentRecord.ID, string(rawResp)); err != nil {
		customlogger.Warn(ctx, "error updating raw response")
	}

	paymentRecord.PaymentID = mpPaymentID
	paymentRecord.Status = result.Status
	paymentRecord.StatusDetail = result.StatusDetail

	return s.mapPaymentResponse(paymentRecord), nil
}

func (s *paymentService) GetPaymentStatus(ctx *gin.Context, paymentID int64) (*payment.PaymentResponse, error) {
	customlogger.Info(ctx, "getting payment status", customlogger.TagMethod("GetPaymentStatus"))

	paymentRecord, err := s.paymentDao.FindByID(ctx, paymentID)
	if err != nil {
		customlogger.Error(ctx, "error finding payment", err)
		return nil, fmt.Errorf("error finding payment: %w", err)
	}
	if paymentRecord == nil {
		return nil, fmt.Errorf("payment not found")
	}

	if paymentRecord.PaymentID != "" {
		mpID, _ := strconv.Atoi(paymentRecord.PaymentID)
		result, err := s.mpClient.GetPayment(ctx, s.accessToken, mpID)
		if err != nil {
			customlogger.Warn(ctx, "error fetching payment from MP", customlogger.Tag("mp_payment_id", paymentRecord.PaymentID))
		} else {
			if result.Status != paymentRecord.Status {
				if err := s.paymentDao.UpdateStatus(ctx, paymentRecord.ID, result.Status, result.StatusDetail); err != nil {
					customlogger.Error(ctx, "error updating payment status", err)
				}
				paymentRecord.Status = result.Status
				paymentRecord.StatusDetail = result.StatusDetail
			}
		}
	}

	return s.mapPaymentResponse(paymentRecord), nil
}

func (s *paymentService) GetPaymentStatusFromMP(ctx *gin.Context, mpPaymentID string) (*payment.MPPaymentStatusResponse, error) {
	customlogger.Info(ctx, "getting payment status from MP", customlogger.TagMethod("GetPaymentStatusFromMP"), customlogger.Tag("mp_payment_id", mpPaymentID))

	mpID, err := strconv.Atoi(mpPaymentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment ID: %w", err)
	}

	result, err := s.mpClient.GetPayment(ctx, s.accessToken, mpID)
	if err != nil {
		return nil, fmt.Errorf("error fetching payment from MP: %w", err)
	}

	card := payment.MPPaymentStatusCard{}
	if result.Card.ID != "" {
		card = payment.MPPaymentStatusCard{
			ID:             result.Card.ID,
			LastFourDigits: result.Card.LastFourDigits,
			FirstSixDigits: result.Card.FirstSixDigits,
			CardholderName: result.Card.Cardholder.Name,
		}
		if v := result.Card.ExpirationMonth.Value; v != nil {
			card.ExpirationMonth = strconv.Itoa(*v)
		}
		if v := result.Card.ExpirationYear.Value; v != nil {
			card.ExpirationYear = strconv.Itoa(*v)
		}
	}

	feeDetails := make([]payment.MPPaymentStatusFeeDetail, 0, len(result.FeeDetails))
	for _, f := range result.FeeDetails {
		feeDetails = append(feeDetails, payment.MPPaymentStatusFeeDetail{
			Type:     f.Type,
			FeePayer: f.FeePayer,
			Amount:   f.Amount,
		})
	}

	return &payment.MPPaymentStatusResponse{
		ID:                        result.ID,
		Status:                    result.Status,
		StatusDetail:              result.StatusDetail,
		OperationType:             result.OperationType,
		Description:               result.Description,
		ExternalReference:         result.ExternalReference,
		TransactionAmount:         result.TransactionAmount,
		TransactionAmountRefunded: result.TransactionAmountRefunded,
		NetAmount:                 result.NetAmount,
		CouponAmount:              result.CouponAmount,
		CurrencyID:                result.CurrencyID,
		PaymentMethodID:           result.PaymentMethodID,
		PaymentTypeID:             result.PaymentTypeID,
		Installments:              result.Installments,
		IssuerID:                  result.IssuerID,
		LiveMode:                  result.LiveMode,
		Captured:                  result.Captured,
		DateCreated:               result.DateCreated.Format(time.RFC3339),
		DateApproved:              result.DateApproved.Format(time.RFC3339),
		DateLastUpdated:           result.DateLastUpdated.Format(time.RFC3339),
		Payer: payment.MPPaymentStatusPayer{
			ID:        result.Payer.ID,
			Email:     result.Payer.Email,
			FirstName: result.Payer.FirstName,
			LastName:  result.Payer.LastName,
			Type:      result.Payer.Type,
			Identification: payment.MPPaymentStatusIdentif{
				Type:   result.Payer.Identification.Type,
				Number: result.Payer.Identification.Number,
			},
			Phone: payment.MPPaymentStatusPhone{
				AreaCode: result.Payer.Phone.AreaCode,
				Number:   result.Payer.Phone.Number,
			},
		},
		Card:       card,
		FeeDetails: feeDetails,
		TransactionDetails: payment.MPPaymentStatusTransaction{
			NetReceivedAmount: result.TransactionDetails.NetReceivedAmount,
			TotalPaidAmount:   result.TransactionDetails.TotalPaidAmount,
			InstallmentAmount: result.TransactionDetails.InstallmentAmount,
			OverpaidAmount:    result.TransactionDetails.OverpaidAmount,
		},
	}, nil
}

func (s *paymentService) HandleWebhook(ctx *gin.Context, notification payment.WebhookNotification) error {
	customlogger.Info(ctx, "handling MP webhook", customlogger.TagMethod("HandleWebhook"), customlogger.Tag("type", notification.Type))

	if notification.Type != "payment" {
		customlogger.Info(ctx, "ignoring non-payment webhook", customlogger.Tag("type", notification.Type))
		return nil
	}

	mpPaymentID := notification.Data.ID
	if mpPaymentID == "" {
		customlogger.Warn(ctx, "webhook missing data.id")
		return fmt.Errorf("webhook missing data.id")
	}

	mpID, err := strconv.Atoi(mpPaymentID)
	if err != nil {
		customlogger.Warn(ctx, "webhook invalid data.id", customlogger.Tag("data_id", mpPaymentID))
		return fmt.Errorf("webhook invalid data.id: %w", err)
	}

	result, err := s.mpClient.GetPayment(ctx, s.accessToken, mpID)
	if err != nil {
		customlogger.Error(ctx, "error fetching payment from MP webhook", err)
		return fmt.Errorf("error fetching payment from MP: %w", err)
	}

	paymentRecord, err := s.paymentDao.FindByPaymentID(ctx, mpPaymentID)
	if err != nil {
		customlogger.Error(ctx, "error finding local payment", err)
		return fmt.Errorf("error finding local payment: %w", err)
	}

	if paymentRecord == nil {
		paymentRecord, err = s.paymentDao.FindByExternalReference(ctx, mpPaymentID)
		if err != nil {
			customlogger.Error(ctx, "error finding local payment by external ref", err)
			return fmt.Errorf("error finding local payment: %w", err)
		}
	}

	if paymentRecord == nil {
		customlogger.Warn(ctx, "payment not found locally", customlogger.Tag("mp_payment_id", mpPaymentID))
		return fmt.Errorf("payment not found locally")
	}

	if err := s.paymentDao.UpdateStatus(ctx, paymentRecord.ID, result.Status, result.StatusDetail); err != nil {
		customlogger.Error(ctx, "error updating payment status from webhook", err)
		return fmt.Errorf("error updating payment status: %w", err)
	}

	rawResp, _ := json.Marshal(result)
	if err := s.paymentDao.UpdateRawResponse(ctx, paymentRecord.ID, string(rawResp)); err != nil {
		customlogger.Warn(ctx, "error updating raw response from webhook")
	}

	customlogger.Info(ctx, "webhook processed successfully",
		customlogger.Tag("payment_id", fmt.Sprintf("%d", paymentRecord.ID)),
		customlogger.Tag("status", result.Status),
	)

	return nil
}

func (s *paymentService) mapPaymentResponse(p *dbs.Payment) *payment.PaymentResponse {
	var externalRef string
	if p.ExternalRef != nil {
		externalRef = *p.ExternalRef
	}
	return &payment.PaymentResponse{
		ID:              p.ID,
		PreferenceID:    p.PreferenceID,
		PaymentID:       p.PaymentID,
		ExternalRef:     externalRef,
		Concept:         p.Concept,
		Description:     p.Description,
		Amount:          p.Amount,
		CurrencyID:      p.CurrencyID,
		Status:          p.Status,
		StatusDetail:    p.StatusDetail,
		PaymentMethodID: p.PaymentMethodID,
		Installments:    p.Installments,
		PayerEmail:      p.PayerEmail,
		CreatedAt:       p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *paymentService) GenerateTestCardToken(ctx *gin.Context, req payment.TestCardTokenRequest) (*payment.TestCardTokenResponse, error) {
	customlogger.Info(ctx, "generating test card token", customlogger.TagMethod("GenerateTestCardToken"))

	token, err := s.mpClient.GenerateCardToken(ctx, s.accessToken,
		req.CardNumber,
		req.ExpirationMonth,
		req.ExpirationYear,
		req.SecurityCode,
		req.CardholderName,
		req.IdentificationType,
		req.IdentificationNumber,
		"MLA",
	)
	if err != nil {
		customlogger.Error(ctx, "error generating card token", err)
		return nil, fmt.Errorf("error generating card token: %w", err)
	}

	return &payment.TestCardTokenResponse{
		Token: token,
	}, nil
}
