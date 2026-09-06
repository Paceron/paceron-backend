package mercadopagoclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
	"github.com/mercadopago/sdk-go/pkg/preference"
	"github.com/mercadopago/sdk-go/pkg/requestoptions"
	"github.com/mercadopago/sdk-go/pkg/webhook"

	appconfig "simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/utils"
)

type MercadoPagoClientInterface interface {
	CreatePreference(ctx context.Context, accessToken string, items []PreferenceItem, externalRef, notificationURL, marketplaceFee string, currencyID string) (string, error)
	CreatePayment(ctx context.Context, accessToken string, req CreatePaymentRequest) (*PaymentResult, error)
	GetPayment(ctx context.Context, accessToken string, paymentID int) (*payment.Response, error)
	ValidateWebhookSignature(xSignature, xRequestID, dataID, secret string) error
	GenerateCardToken(ctx context.Context, accessToken string, cardNumber, expirationMonth, expirationYear, cvv, cardholderName, identificationType, identificationNumber, siteID string) (string, error)
	GetAuthURL(redirectURI string, state string) string
	ExchangeCodeForToken(ctx context.Context, clientID, clientSecret, redirectURI, code string) (*OAuthTokenResponse, error)
	RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*OAuthTokenResponse, error)
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

type mpClient struct {
	apiBaseURL  string
	authBaseURL string
}

// New crea el cliente de MercadoPago apuntando a las URLs productivas.
func New() MercadoPagoClientInterface {
	return newMP("https://api.mercadopago.com", "https://auth.mercadopago.com")
}

// newMP crea el cliente con base URLs inyectables (tests usan httptest).
func newMP(apiBaseURL, authBaseURL string) MercadoPagoClientInterface {
	return &mpClient{apiBaseURL: apiBaseURL, authBaseURL: authBaseURL}
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

	url := fmt.Sprintf("%s/v1/card_tokens?public_key=%s", c.apiBaseURL, publicKey)
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
	url := fmt.Sprintf("%s/authorization?client_id=%s&response_type=code&redirect_uri=%s&state=%s", c.authBaseURL, clientID, url.QueryEscape(redirectURI), url.QueryEscape(state))
	maskedAuthURL := strings.Replace(url, clientID, utils.MaskSecret(clientID), -1)
	customlogger.Info(nil, "[DEBUG] GetAuthURL",
		customlogger.Tag("client_id", utils.MaskSecret(clientID)),
		customlogger.Tag("redirect_uri", redirectURI),
		customlogger.Tag("state", state),
		customlogger.Tag("auth_url", maskedAuthURL),
		customlogger.TagMethod("GetAuthURL"))
	return url
}

func (c *mpClient) ExchangeCodeForToken(ctx context.Context, clientID, clientSecret, redirectURI, code string) (*OAuthTokenResponse, error) {
	url := fmt.Sprintf("%s/oauth/token", c.apiBaseURL)
	body := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=authorization_code&redirect_uri=%s&code=%s", clientID, clientSecret, redirectURI, code)
	if appconfig.MyMP.OAuthTestToken {
		body += "&test_token=true"
	}

	customlogger.Info(nil, "[DEBUG] ExchangeCodeForToken request",
		customlogger.Tag("client_id", utils.MaskSecret(clientID)),
		customlogger.Tag("client_secret", utils.MaskSecret(clientSecret)),
		customlogger.Tag("redirect_uri", redirectURI),
		customlogger.Tag("code", utils.MaskSecret(code)),
		customlogger.TagMethod("ExchangeCodeForToken"))

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

	customlogger.Info(nil, "[DEBUG] ExchangeCodeForToken response",
		customlogger.Tag("access_token", utils.MaskSecret(tokenResp.AccessToken)),
		customlogger.Tag("token_type", tokenResp.TokenType),
		customlogger.Tag("expires_in", fmt.Sprintf("%d", tokenResp.ExpiresIn)),
		customlogger.Tag("refresh_token", utils.MaskSecret(tokenResp.RefreshToken)),
		customlogger.Tag("scope", tokenResp.Scope),
		customlogger.Tag("user_id", fmt.Sprintf("%d", tokenResp.UserID)),
		customlogger.TagMethod("ExchangeCodeForToken"))

	return &tokenResp, nil
}

func (c *mpClient) RefreshAccessToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*OAuthTokenResponse, error) {
	url := fmt.Sprintf("%s/oauth/token", c.apiBaseURL)
	body := fmt.Sprintf("grant_type=refresh_token&client_id=%s&client_secret=%s&refresh_token=%s", clientID, clientSecret, refreshToken)
	if appconfig.MyMP.OAuthTestToken {
		body += "&test_token=true"
	}

	customlogger.Info(nil, "[DEBUG] RefreshAccessToken request",
		customlogger.Tag("client_id", utils.MaskSecret(clientID)),
		customlogger.Tag("client_secret", utils.MaskSecret(clientSecret)),
		customlogger.Tag("refresh_token", utils.MaskSecret(refreshToken)),
		customlogger.TagMethod("RefreshAccessToken"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending refresh request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error refreshing token: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("error parsing refresh response: %w", err)
	}

	customlogger.Info(nil, "[DEBUG] RefreshAccessToken response",
		customlogger.Tag("access_token", utils.MaskSecret(tokenResp.AccessToken)),
		customlogger.Tag("token_type", tokenResp.TokenType),
		customlogger.Tag("expires_in", fmt.Sprintf("%d", tokenResp.ExpiresIn)),
		customlogger.Tag("refresh_token", utils.MaskSecret(tokenResp.RefreshToken)),
		customlogger.Tag("scope", tokenResp.Scope),
		customlogger.Tag("user_id", fmt.Sprintf("%d", tokenResp.UserID)),
		customlogger.TagMethod("RefreshAccessToken"))

	return &tokenResp, nil
}

func (c *mpClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error) {
	url := fmt.Sprintf("%s/users/me?access_token=%s", c.apiBaseURL, accessToken)
	maskedURL := strings.Replace(url, accessToken, utils.MaskSecret(accessToken), -1)

	customlogger.Info(nil, "[DEBUG] GetUserInfo request",
		customlogger.Tag("access_token", utils.MaskSecret(accessToken)),
		customlogger.Tag("url", maskedURL),
		customlogger.TagMethod("GetUserInfo"))

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

	customlogger.Info(nil, "[DEBUG] GetUserInfo response",
		customlogger.Tag("mp_user_id", fmt.Sprintf("%d", userResp.ID)),
		customlogger.Tag("nickname", userResp.Nick),
		customlogger.TagMethod("GetUserInfo"))

	return &userResp, nil
}
