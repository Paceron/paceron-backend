package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/infrastructure/crypto"
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
	paymentDao       daos.PaymentDaoInterface
	mpClient         mercadopagoclient.MercadoPagoClientInterface
	accessToken      string
	publicKey        string
	webhookSecret    string
	currencyID       string
	db               *gorm.DB
	// split team subscriptions
	sellerConnDao    daos.SellerConnectionDaoInterface
	teamDao          daos.TeamDaoInterface
	teamUserDao      daos.TeamUserDaoInterface
	settingDao       daos.PlatformSettingDaoInterface
	installDao       daos.InstallmentDaoInterface
	encryptor        crypto.EncryptorInterface
}

func NewPaymentService(
	paymentDao daos.PaymentDaoInterface,
	mpClient mercadopagoclient.MercadoPagoClientInterface,
	db *gorm.DB,
	sellerConnDao daos.SellerConnectionDaoInterface,
	teamDao daos.TeamDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	settingDao daos.PlatformSettingDaoInterface,
	installDao daos.InstallmentDaoInterface,
	encryptor crypto.EncryptorInterface,
) PaymentServiceInterface {
	return &paymentService{
		paymentDao:    paymentDao,
		mpClient:      mpClient,
		accessToken:   config.MyMP.AccessToken,
		publicKey:     config.MyMP.PublicKey,
		webhookSecret: config.MyMP.WebhookSecret,
		currencyID:    config.MyMP.CurrencyID,
		db:            db,
		sellerConnDao: sellerConnDao,
		teamDao:       teamDao,
		teamUserDao:   teamUserDao,
		settingDao:    settingDao,
		installDao:    installDao,
		encryptor:     encryptor,
	}
}

func (s *paymentService) CreatePreference(ctx *gin.Context, req payment.CreatePreferenceRequest) (*payment.CreatePreferenceResponse, error) {
	customlogger.Info(ctx, "creating MP preference", customlogger.TagMethod("CreatePreference"))

	amount := 0.0
	for _, item := range req.Items {
		amount += item.UnitPrice * float64(item.Quantity)
	}

	var mpAccessToken string
	var marketplaceFee float64
	var sellerUserID int64

	// Si es suscripción de equipo (team_subscription), usar token del dueño y aplicar split
	if req.Concept == string(constants.PaymentConceptTeamSubscription) {
		var err error
		mpAccessToken, marketplaceFee, sellerUserID, err = s.resolveTeamSplitConfig(ctx, req.InstallmentID)
		if err != nil {
			return nil, err
		}
	} else {
		mpAccessToken = s.accessToken
		marketplaceFee = 0
		sellerUserID = 0
	}

	var sellerUserIDPtr *int64
	var marketplaceFeePtr *float64
	if sellerUserID != 0 {
		sellerUserIDPtr = &sellerUserID
	}
	if marketplaceFee > 0 {
		marketplaceFeePtr = &marketplaceFee
	}

	paymentRecord := &dbs.Payment{
		UserID:         req.SellerID,
		Concept:        req.Concept,
		Description:    req.Description,
		Amount:         amount,
		CurrencyID:     s.currencyID,
		Status:         "pending",
		PayerEmail:     "",
		InstallmentID:  req.InstallmentID,
		SellerUserID:   sellerUserIDPtr,
		MarketplaceFee: marketplaceFeePtr,
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
	preferenceID, err := s.mpClient.CreatePreference(ctx, mpAccessToken, items, externalRef, notificationURL, fmt.Sprintf("%.2f", marketplaceFee), s.currencyID)
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

	var mpAccessToken string
	var marketplaceFee float64
	var sellerUserID int64

	// Resolver configuración de split si es team_subscription
	if req.Concept == string(constants.PaymentConceptTeamSubscription) && req.InstallmentID != nil {
		var err error
		mpAccessToken, marketplaceFee, sellerUserID, err = s.resolveTeamSplitConfig(ctx, req.InstallmentID)
		if err != nil {
			return nil, err
		}
	} else {
		mpAccessToken = s.accessToken
		marketplaceFee = 0
		sellerUserID = 0
	}

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

	concept := req.Concept
	if concept == "" {
		concept = "order"
	}

	var sellerUserIDPtr *int64
	var marketplaceFeePtr *float64
	if sellerUserID != 0 {
		sellerUserIDPtr = &sellerUserID
	}
	if marketplaceFee > 0 {
		marketplaceFeePtr = &marketplaceFee
	}

	paymentRecord := &dbs.Payment{
		Concept:         concept,
		Description:     req.Description,
		Amount:          req.TransactionAmount,
		CurrencyID:      s.currencyID,
		Status:          "in_process",
		PayerEmail:      req.PayerEmail,
		PreferenceID:    req.PreferenceID,
		PaymentMethodID: req.PaymentMethodID,
		Installments:    req.Installments,
		InstallmentID:   req.InstallmentID,
		SellerUserID:    sellerUserIDPtr,
		MarketplaceFee:  marketplaceFeePtr,
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

	result, err := s.mpClient.CreatePayment(ctx, mpAccessToken, mpReq)
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

	// Pago de cuota aprobado: confirmar la cuota y avanzar el ciclo mensual (D6/D7).
	if result.Status == "approved" && paymentRecord.InstallmentID != nil {
		// Determinar si es cuota de tier o de equipo
		installment, err := s.installDao.FindByID(ctx, *paymentRecord.InstallmentID)
		if err != nil {
			customlogger.Error(ctx, "error finding installment for webhook", err)
		} else if installment != nil && installment.TeamID != nil {
			// Cuota de equipo (change suscripcion-teams-split D10)
			if err := s.applyApprovedTeamInstallment(ctx, paymentRecord, mpPaymentID); err != nil {
				return fmt.Errorf("error confirming team installment: %w", err)
			}
		} else {
			// Cuota de tier (D6/D7)
			if err := s.applyApprovedInstallment(ctx, paymentRecord, mpPaymentID); err != nil {
				return fmt.Errorf("error confirming installment: %w", err)
			}
		}
	}

	customlogger.Info(ctx, "webhook processed successfully",
		customlogger.Tag("payment_id", fmt.Sprintf("%d", paymentRecord.ID)),
		customlogger.Tag("status", result.Status),
	)

	return nil
}

// applyApprovedInstallment confirma una cuota pagada y genera la siguiente (D6/D7),
// todo en una transacción GORM. El marcado es condicional (`WHERE status='pending'`,
// MarkPaidConditional): la doble notificación del webhook no tiene efectos (idempotencia).
// Solo se aplica a cuotas de suscripción de tier (`subscription_id`); las de equipo
// (change suscripcion-teams-split) quedan marcadas como paid y el resto del flujo
// lo completa ese change.
func (s *paymentService) applyApprovedInstallment(ctx *gin.Context, paymentRecord *dbs.Payment, mpPaymentID string) error {
	if s.db == nil {
		return fmt.Errorf("payment service sin db configurada")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		insDao := daos.NewInstallmentDao(tx)
		subDao := daos.NewTierSubscriptionDao(tx)
		urDao := daos.NewUserRoleDao(tx)

		marked, err := insDao.MarkPaidConditional(ctx, *paymentRecord.InstallmentID, &paymentRecord.ID, &mpPaymentID)
		if err != nil {
			return err
		}
		if !marked {
			customlogger.Warn(ctx, "installment already paid, webhook is a duplicate",
				customlogger.Tag("installment_id", fmt.Sprintf("%d", *paymentRecord.InstallmentID)),
				customlogger.TagMethod("applyApprovedInstallment"))
			return nil
		}

		installment, err := insDao.FindByID(ctx, *paymentRecord.InstallmentID)
		if err != nil {
			return err
		}
		if installment == nil {
			return fmt.Errorf("installment %d not found after marking paid", *paymentRecord.InstallmentID)
		}
		if installment.SubscriptionID == nil {
			return nil // cuota de equipo: el flujo de split lo completa (change 2)
		}

		sub, err := subDao.FindByID(ctx, *installment.SubscriptionID)
		if err != nil {
			return err
		}
		if sub == nil {
			return fmt.Errorf("subscription %d not found", *installment.SubscriptionID)
		}

		if err := subDao.IncrementPaidInstallments(ctx, sub.ID); err != nil {
			return err
		}

		if installment.InstallmentNumber == 1 {
			if err := subDao.Activate(ctx, sub.ID); err != nil {
				return err
			}
			if err := urDao.UpdateTier(ctx, sub.UserID, sub.RoleID, sub.TierID); err != nil {
				return err
			}
		}

		return insDao.Create(ctx, NextTierInstallment(sub, installment))
	})
}

// applyApprovedTeamInstallment confirma una cuota de equipo pagada (D10):
// - Marca la cuota paid idempotentemente
// - Incrementa paid_installments del team_user
// - Si era la cuota #1, activa la membresía (subscription_status = active)
// - Genera la siguiente cuota con NextTeamInstallment
func (s *paymentService) applyApprovedTeamInstallment(ctx *gin.Context, paymentRecord *dbs.Payment, mpPaymentID string) error {
	if s.db == nil {
		return fmt.Errorf("payment service sin db configurada")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		insDao := daos.NewInstallmentDao(tx)
		tuDao := daos.NewTeamUserDao(tx)

		marked, err := insDao.MarkPaidConditional(ctx, *paymentRecord.InstallmentID, &paymentRecord.ID, &mpPaymentID)
		if err != nil {
			return err
		}
		if !marked {
			customlogger.Warn(ctx, "team installment already paid, webhook is a duplicate",
				customlogger.Tag("installment_id", fmt.Sprintf("%d", *paymentRecord.InstallmentID)),
				customlogger.TagMethod("applyApprovedTeamInstallment"))
			return nil
		}

		installment, err := insDao.FindByID(ctx, *paymentRecord.InstallmentID)
		if err != nil {
			return err
		}
		if installment == nil {
			return fmt.Errorf("installment %d not found after marking paid", *paymentRecord.InstallmentID)
		}
		if installment.TeamID == nil || installment.UserID == 0 {
			return fmt.Errorf("installment %d missing team_id or user_id", *paymentRecord.InstallmentID)
		}

		// Buscar el team_user para obtener su ID
		teamUser, err := tuDao.FindByTeamAndUser(ctx, *installment.TeamID, installment.UserID)
		if err != nil {
			return err
		}
		if teamUser == nil {
			return fmt.Errorf("team_user not found for team_id=%d user_id=%d", *installment.TeamID, installment.UserID)
		}

		// Incrementar paid_installments del team_user
		if err := tuDao.IncrementPaidInstallments(ctx, teamUser.ID); err != nil {
			return err
		}

		// Si era la cuota #1, activar membresía
		if installment.InstallmentNumber == 1 {
			if err := tuDao.ActivateSubscription(ctx, teamUser.ID); err != nil {
				return err
			}
		}

		// Generar siguiente cuota de equipo
		nextIns := NextTeamInstallment(teamUser, installment)
		return insDao.Create(ctx, nextIns)
	})
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

// resolveTeamSplitConfig obtiene el token del dueño del equipo y calcula la comisión
// marketplace_fee. Debe llamarse con un installment_id válido de tipo team_subscription.
func (s *paymentService) resolveTeamSplitConfig(ctx *gin.Context, installmentID *int64) (accessToken string, marketplaceFee float64, sellerUserID int64, err error) {
	if installmentID == nil {
		return "", 0, 0, fmt.Errorf("installment_id requerido para team_subscription")
	}

	installment, err := s.installDao.FindByID(ctx, *installmentID)
	if err != nil {
		customlogger.Error(ctx, "error finding installment for split", err,
			customlogger.Tag("installment_id", fmt.Sprintf("%d", *installmentID)),
			customlogger.TagMethod("resolveTeamSplitConfig"))
		return "", 0, 0, fmt.Errorf("error consultando cuota")
	}
	if installment == nil {
		return "", 0, 0, fmt.Errorf("cuota no encontrada")
	}
	if installment.TeamID == nil {
		return "", 0, 0, fmt.Errorf("la cuota no pertenece a un equipo")
	}

	team, err := s.teamDao.FindByID(ctx, *installment.TeamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for split", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", *installment.TeamID)),
			customlogger.TagMethod("resolveTeamSplitConfig"))
		return "", 0, 0, fmt.Errorf("error consultando equipo")
	}
	if team == nil {
		return "", 0, 0, fmt.Errorf("equipo no encontrado")
	}

	ownerID := team.OwnerID
	conn, err := s.sellerConnDao.FindByUser(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error finding seller connection", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)),
			customlogger.TagMethod("resolveTeamSplitConfig"))
		return "", 0, 0, fmt.Errorf("error consultando conexión del entrenador")
	}
	if conn == nil || conn.Status != string(constants.SellerConnectionStatusAuthorized) {
		customlogger.Warn(ctx, "team owner not connected to MP",
			customlogger.Tag("team_id", fmt.Sprintf("%d", team.ID)),
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)),
			customlogger.TagMethod("resolveTeamSplitConfig"))
		return "", 0, 0, fmt.Errorf("el entrenador debe conectar su cuenta de Mercado Pago")
	}

	// Descifrar access_token
	decrypted, err := s.encryptor.Decrypt(conn.AccessToken)
	if err != nil {
		customlogger.Error(ctx, "error decrypting access token", err,
			customlogger.Tag("owner_id", fmt.Sprintf("%d", ownerID)),
			customlogger.TagMethod("resolveTeamSplitConfig"))
		return "", 0, 0, fmt.Errorf("error descifrando token del entrenador")
	}

	// Obtener marketplace_fee_percent (default 5%)
	feePercent, setting, err := s.settingDao.Get(ctx, "marketplace_fee_percent", 5.0)
	if err != nil {
		customlogger.Error(ctx, "error getting marketplace fee", err,
			customlogger.TagMethod("resolveTeamSplitConfig"))
		feePercent = 5.0
	}
	if setting == nil {
		feePercent = 5.0
	}

	// Calcular fee redondeado (moneda local, 2 decimales)
	marketplaceFee = float64(int(installment.Amount*feePercent/100*100+0.5)) / 100

	return decrypted, marketplaceFee, ownerID, nil
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
