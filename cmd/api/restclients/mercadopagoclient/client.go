package mercadopagoclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"
	"github.com/mercadopago/sdk-go/pkg/requestoptions"
	"github.com/mercadopago/sdk-go/pkg/webhook"

	appconfig "simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type MercadoPagoClientInterface interface {
	CreatePreference(ctx context.Context, accessToken string, items []PreferenceItem, externalRef, notificationURL, marketplaceFee string, currencyID string) (string, error)
	CreatePayment(ctx context.Context, accessToken string, req CreatePaymentRequest) (*PaymentResult, error)
	GetPayment(ctx context.Context, accessToken string, paymentID int) (*payment.Response, error)
	ValidateWebhookSignature(xSignature, xRequestID, dataID, secret string) error
	GenerateCardToken(ctx context.Context, accessToken string, cardNumber, expirationMonth, expirationYear, cvv, cardholderName, identificationType, identificationNumber, siteID string) (string, error)
	GetAuthURL(redirectURI string, state string) string
	ExchangeCodeForToken(ctx context.Context, clientID, clientSecret, redirectURI, code string) (*OAuthTokenResponse, error)
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error)
}

type PreferenceItem struct {
	Title     string
	Quantity  int
	UnitPrice float64
}

type CreatePaymentRequest struct {
	Token             string
	TransactionAmount float64
	PaymentMethodID   string
	Installments      int
	PayerEmail        string
	Description       string
	ExternalReference string
	NotificationURL   string
	ThreeDSecureMode  string
}

type PaymentResult struct {
	ID            int
	Status        string
	StatusDetail  string
	FeeDetailsRaw json.RawMessage
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	UserID       int64  `json:"user_id"`
}

type UserInfoResponse struct {
	ID   int64  `json:"id"`
	Nick string `json:"nickname"`
}

type mpClient struct{}

func New() MercadoPagoClientInterface {
	return &mpClient{}
}

func (c *mpClient) CreatePreference(ctx context.Context, accessToken string, items []PreferenceItem, externalRef, notificationURL, marketplaceFee string, currencyID string) (string, error) {
	cfg, err := config.New(accessToken)
	if err != nil {
		return "", fmt.Errorf("error creating MP config: %w", err)
	}

	client := preference.NewClient(cfg)

	var sdkItems []preference.ItemRequest
	for _, item := range items {
		sdkItems = append(sdkItems, preference.ItemRequest{
			Title:      item.Title,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			CurrencyID: currencyID,
		})
	}

	req := preference.Request{
		Items:             sdkItems,
		ExternalReference: externalRef,
		NotificationURL:   notificationURL,
	}

	resp, err := client.Create(ctx, req)
	if err != nil {
		customlogger.Error(nil, "error creating MP preference", err)
		return "", fmt.Errorf("error creating MP preference: %w", err)
	}

	return resp.ID, nil
}

func (c *mpClient) CreatePayment(ctx context.Context, accessToken string, req CreatePaymentRequest) (*PaymentResult, error) {
	cfg, err := config.New(accessToken)
	if err != nil {
		return nil, fmt.Errorf("error creating MP config: %w", err)
	}

	client := payment.NewClient(cfg)

	idempotencyKey := fmt.Sprintf("paceron-%d", time.Now().UnixNano())
	ctx = requestoptions.WithIdempotencyKey(ctx, idempotencyKey)

	payerEmail := req.PayerEmail
	mpReq := payment.Request{
		Token:             req.Token,
		TransactionAmount: req.TransactionAmount,
		PaymentMethodID:   req.PaymentMethodID,
		Installments:      req.Installments,
		Description:       req.Description,
		ExternalReference: req.ExternalReference,
		NotificationURL:   req.NotificationURL,
		ThreeDSecureMode:  req.ThreeDSecureMode,
		Payer: &payment.PayerRequest{
			Email: payerEmail,
		},
	}

	resp, err := client.Create(ctx, mpReq)
	if err != nil {
		customlogger.Error(nil, "error creating MP payment", err)
		return nil, fmt.Errorf("error creating MP payment: %w", err)
	}

	feeDetails, _ := json.Marshal(resp.FeeDetails)

	return &PaymentResult{
		ID:            resp.ID,
		Status:        resp.Status,
		StatusDetail:  resp.StatusDetail,
		FeeDetailsRaw: feeDetails,
	}, nil
}

func (c *mpClient) GetPayment(ctx context.Context, accessToken string, paymentID int) (*payment.Response, error) {
	cfg, err := config.New(accessToken)
	if err != nil {
		return nil, fmt.Errorf("error creating MP config: %w", err)
	}

	client := payment.NewClient(cfg)

	resp, err := client.Get(ctx, paymentID)
	if err != nil {
		customlogger.Error(nil, "error getting MP payment", err)
		return nil, fmt.Errorf("error getting MP payment: %w", err)
	}

	return resp, nil
}

func (c *mpClient) ValidateWebhookSignature(xSignature, xRequestID, dataID, secret string) error {
	return webhook.ValidateSignature(xSignature, xRequestID, dataID, secret)
}

func (c *mpClient) GenerateCardToken(ctx context.Context, accessToken string, cardNumber, expirationMonth, expirationYear, cvv, cardholderName, identificationType, identificationNumber, siteID string) (string, error) {
	publicKey := appconfig.MyMP.PublicKey

	type cardTokenRequest struct {
		CardNumber      string `json:"card_number"`
		ExpirationMonth string `json:"expiration_month"`
		ExpirationYear  string `json:"expiration_year"`
		SecurityCode    string `json:"security_code"`
		SiteID          string `json:"site_id,omitempty"`
		Cardholder      *struct {
			Name           string `json:"name"`
			Identification *struct {
				Type   string `json:"type"`
				Number string `json:"number"`
			} `json:"identification"`
		} `json:"cardholder"`
	}

	type cardTokenResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	body := cardTokenRequest{
		CardNumber:      cardNumber,
		ExpirationMonth: expirationMonth,
		ExpirationYear:  expirationYear,
		SecurityCode:    cvv,
		SiteID:          siteID,
		Cardholder: &struct {
			Name           string `json:"name"`
			Identification *struct {
				Type   string `json:"type"`
				Number string `json:"number"`
			} `json:"identification"`
		}{
			Name: cardholderName,
			Identification: &struct {
				Type   string `json:"type"`
				Number string `json:"number"`
			}{
				Type:   identificationType,
				Number: identificationNumber,
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("error marshaling card token request: %w", err)
	}

	url := fmt.Sprintf("https://api.mercadopago.com/v1/card_tokens?public_key=%s", publicKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("error creating card token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending card token request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error generating card token: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp cardTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("error parsing card token response: %w", err)
	}

	return tokenResp.ID, nil
}

func (c *mpClient) GetAuthURL(redirectURI string, state string) string {
	clientID := appconfig.MyMP.OAuthClientID
	if clientID == "" {
		customlogger.Error(nil, "MP OAuth client ID not configured", fmt.Errorf("missing client ID"))
		return ""
	}
	return fmt.Sprintf("https://auth.mercadopago.com/authorization?client_id=%s&response_type=code&redirect_uri=%s&state=%s", clientID, redirectURI, state)
}

func (c *mpClient) ExchangeCodeForToken(ctx context.Context, clientID, clientSecret, redirectURI, code string) (*OAuthTokenResponse, error) {
	url := "https://api.mercadopago.com/oauth/token"
	body := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=authorization_code&redirect_uri=%s&code=%s", clientID, clientSecret, redirectURI, code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending token request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error exchanging code: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("error parsing token response: %w", err)
	}

	return &tokenResp, nil
}

func (c *mpClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error) {
	url := fmt.Sprintf("https://api.mercadopago.com/users/me?access_token=%s", accessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating user info request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending user info request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting user info: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var userResp UserInfoResponse
	if err := json.Unmarshal(respBody, &userResp); err != nil {
		return nil, fmt.Errorf("error parsing user info response: %w", err)
	}

	return &userResp, nil
}