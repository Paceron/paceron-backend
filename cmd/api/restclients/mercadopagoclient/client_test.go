package mercadopagoclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appconfig "simple-arq-golang/cmd/api/config"
)

// captureRequester implementa requester.Requester (misma firma que
// http.Client.Do) para interceptar lo que CreatePreference/CreatePayment le
// mandarían de verdad a Mercado Pago — esos dos métodos usan el SDK oficial
// (preference.NewClient/payment.NewClient), que arma la URL real
// (api.mercadopago.com) hardcodeada adentro del SDK, así que apiBaseURL de
// mpClient no les aplica (a diferencia de ExchangeCodeForToken/GetUserInfo,
// que usan net/http directo). config.WithHTTPClient(...) es el único punto de
// inyección real: reemplaza el transporte, nunca sale a la red.
type captureRequester struct {
	lastBody []byte
	status   int
	respBody string
}

func (r *captureRequester) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		r.lastBody, _ = io.ReadAll(req.Body)
	}
	status := r.status
	if status == 0 {
		status = http.StatusCreated
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader([]byte(r.respBody))),
		Header:     make(http.Header),
	}, nil
}

func TestNew_ImplementsInterface(t *testing.T) {
	client := New()
	var iface MercadoPagoClientInterface = client
	_ = iface
}

func TestGetAuthURL_Success(t *testing.T) {
	original := appconfig.MyMP.OAuthClientID
	appconfig.MyMP.OAuthClientID = "CLIENT_ID_TEST"
	t.Cleanup(func() { appconfig.MyMP.OAuthClientID = original })

	client := newMP("http://api.local", "http://auth.local")
	url := client.GetAuthURL("https://paceron/api/v1/mercadopago/connect/callback", "42-123456")

	assert.Contains(t, url, "http://auth.local/authorization")
	assert.Contains(t, url, "client_id=CLIENT_ID_TEST")
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "redirect_uri=https%3A%2F%2Fpaceron%2Fapi%2Fv1%2Fmercadopago%2Fconnect%2Fcallback")
	assert.Contains(t, url, "state=42-123456")
}

func TestGetAuthURL_NotConfigured(t *testing.T) {
	original := appconfig.MyMP.OAuthClientID
	appconfig.MyMP.OAuthClientID = ""
	t.Cleanup(func() { appconfig.MyMP.OAuthClientID = original })

	client := newMP("http://api.local", "http://auth.local")
	url := client.GetAuthURL("https://paceron/callback", "state-1")
	assert.Equal(t, "", url)
}

func TestExchangeCodeForToken_Success(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		r.ParseForm()
		received = r.Form.Get("code")
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "the-code", r.Form.Get("code"))
		assert.Equal(t, "true", r.Form.Get("test_token"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new-access","token_type":"bearer","expires_in":21600,"refresh_token":"new-refresh","scope":"offline_access","user_id":456}`))
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	resp, err := client.ExchangeCodeForToken(context.Background(), "cid", "csec", "http://cb", "the-code")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "new-access", resp.AccessToken)
	assert.Equal(t, "new-refresh", resp.RefreshToken)
	assert.Equal(t, 21600, resp.ExpiresIn)
	assert.Equal(t, int64(456), resp.UserID)
	assert.Equal(t, "the-code", received)
}

func TestExchangeCodeForToken_HTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	_, err := client.ExchangeCodeForToken(context.Background(), "cid", "csec", "http://cb", "bad-code")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "error exchanging code"))
}

func TestExchangeCodeForToken_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	_, err := client.ExchangeCodeForToken(context.Background(), "cid", "csec", "http://cb", "the-code")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "error parsing token response"))
}

func TestRefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth/token", r.URL.Path)
		r.ParseForm()
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "refresh-tok", r.Form.Get("refresh_token"))
		assert.Equal(t, "true", r.Form.Get("test_token"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"rotated-access","token_type":"bearer","expires_in":21600,"refresh_token":"rotated-refresh","scope":"offline_access","user_id":456}`))
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	resp, err := client.RefreshAccessToken(context.Background(), "cid", "csec", "refresh-tok")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "rotated-access", resp.AccessToken)
	assert.Equal(t, "rotated-refresh", resp.RefreshToken)
}

func TestRefreshAccessToken_HTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	_, err := client.RefreshAccessToken(context.Background(), "cid", "csec", "expired-refresh")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "error refreshing token"))
}

func TestGetUserInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/me", r.URL.Path)
		assert.Equal(t, "access-tok", r.URL.Query().Get("access_token"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":456,"nickname":"TEST-SELLER"}`))
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	user, err := client.GetUserInfo(context.Background(), "access-tok")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, int64(456), user.ID)
	assert.Equal(t, "TEST-SELLER", user.Nick)
}

func TestGetUserInfo_HTTPErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newMP(server.URL, "http://auth.local")
	_, err := client.GetUserInfo(context.Background(), "invalid-token")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "error getting user info"))
}

// Los 4 tests de acá abajo cubren el fix del split de Mercado Pago
// (plan_fix_pagos.md quedaba con el diagnóstico completo pero el fix de la
// PK del vendedor sin implementar en ese momento — verificado empíricamente
// el 2026-09-27 que ese fix ya está en develop; el que faltaba, y sigue
// faltando hasta este commit, era este: marketplaceFee/ApplicationFee nunca
// llegaban al request real de MP, así que la comisión de Paceron se calculaba
// y persistía local pero el 100% del dinero quedaba en la cuenta del
// vendedor). Usan captureRequester en vez de httptest.NewServer porque
// CreatePreference/CreatePayment pasan por el SDK oficial de MP, que arma la
// URL real internamente — apiBaseURL no les aplica, config.WithHTTPClient es
// el único punto de inyección real (ver mpClient.httpRequester).

func TestCreatePreference_SendsMarketplaceFee(t *testing.T) {
	capture := &captureRequester{respBody: `{"id":"pref-123"}`}
	c := newMP("http://api.local", "http://auth.local").(*mpClient)
	c.httpRequester = capture

	id, err := c.CreatePreference(context.Background(), "TEST-seller-token",
		[]PreferenceItem{{Title: "Mensualidad", Quantity: 1, UnitPrice: 1500}},
		"ext-1", "https://webhook.local", "75.00", "ARS")

	require.NoError(t, err)
	assert.Equal(t, "pref-123", id)
	assert.Contains(t, string(capture.lastBody), `"marketplace_fee":75`)
}

func TestCreatePreference_ZeroFeeOmitsField(t *testing.T) {
	capture := &captureRequester{respBody: `{"id":"pref-456"}`}
	c := newMP("http://api.local", "http://auth.local").(*mpClient)
	c.httpRequester = capture

	_, err := c.CreatePreference(context.Background(), "TEST-token",
		[]PreferenceItem{{Title: "Item", Quantity: 1, UnitPrice: 100}},
		"ext-2", "https://webhook.local", "0.00", "ARS")

	require.NoError(t, err)
	assert.NotContains(t, string(capture.lastBody), "marketplace_fee")
}

func TestCreatePayment_SendsApplicationFee(t *testing.T) {
	capture := &captureRequester{respBody: `{"id":789,"status":"approved","status_detail":"accredited"}`}
	c := newMP("http://api.local", "http://auth.local").(*mpClient)
	c.httpRequester = capture

	result, err := c.CreatePayment(context.Background(), "TEST-seller-token", CreatePaymentRequest{
		Token:             "card-tok",
		TransactionAmount: 1500,
		PaymentMethodID:   "master",
		Installments:      1,
		PayerEmail:        "payer@example.com",
		ApplicationFee:    75,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 789, result.ID)
	assert.Contains(t, string(capture.lastBody), `"application_fee":75`)
}

func TestCreatePayment_ZeroFeeOmitsField(t *testing.T) {
	capture := &captureRequester{respBody: `{"id":790,"status":"approved","status_detail":"accredited"}`}
	c := newMP("http://api.local", "http://auth.local").(*mpClient)
	c.httpRequester = capture

	_, err := c.CreatePayment(context.Background(), "TEST-token", CreatePaymentRequest{
		Token:             "card-tok",
		TransactionAmount: 1000,
		PaymentMethodID:   "visa",
		Installments:      1,
		PayerEmail:        "payer@example.com",
	})

	require.NoError(t, err)
	assert.NotContains(t, string(capture.lastBody), "application_fee")
}
