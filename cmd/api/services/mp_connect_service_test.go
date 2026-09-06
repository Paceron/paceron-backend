package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/mpconnect"
	"simple-arq-golang/cmd/api/restclients/mercadopagoclient"
)

type mockSellerConnectionDao struct {
	upsertFn            func(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error)
	findByUserFn        func(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error)
	setStatusFn         func(ctx *gin.Context, userID int64, status string) error
	setStatusByMPUserFn func(ctx *gin.Context, mpUserID int64, status string) error
	findAuthorizedFn    func(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error)
}

func (m *mockSellerConnectionDao) Upsert(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, conn)
	}
	return conn, nil
}

func (m *mockSellerConnectionDao) FindByUser(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
	if m.findByUserFn != nil {
		return m.findByUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockSellerConnectionDao) SetStatus(ctx *gin.Context, userID int64, status string) error {
	if m.setStatusFn != nil {
		return m.setStatusFn(ctx, userID, status)
	}
	return nil
}

func (m *mockSellerConnectionDao) SetStatusByMPUser(ctx *gin.Context, mpUserID int64, status string) error {
	if m.setStatusByMPUserFn != nil {
		return m.setStatusByMPUserFn(ctx, mpUserID, status)
	}
	return nil
}

func (m *mockSellerConnectionDao) FindAuthorizedByUser(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
	if m.findAuthorizedFn != nil {
		return m.findAuthorizedFn(ctx, userID)
	}
	return nil, nil
}

type mockEncryptor struct {
	encryptFn func(plaintext string) (string, error)
	decryptFn func(ciphertext string) (string, error)
}

func (m *mockEncryptor) Encrypt(plaintext string) (string, error) {
	if m.encryptFn != nil {
		return m.encryptFn(plaintext)
	}
	return "enc(" + plaintext + ")", nil
}

func (m *mockEncryptor) Decrypt(ciphertext string) (string, error) {
	if m.decryptFn != nil {
		return m.decryptFn(ciphertext)
	}
	return strings.TrimSuffix(strings.TrimPrefix(ciphertext, "enc("), ")"), nil
}

func newTestMPConnectService(connDao *mockSellerConnectionDao, client *mockMercadoPagoClient, enc *mockEncryptor) MPConnectServiceInterface {
	return NewMPConnectService(connDao, client, enc, "client-id", "client-secret", "https://redirect")
}

func TestGetAuthURL_Success(t *testing.T) {
	client := new(mockMercadoPagoClient)
	client.On("GetAuthURL", mock.Anything, mock.Anything).Return("https://mp/auth?code=xyz").Once()

	svc := newTestMPConnectService(&mockSellerConnectionDao{}, client, &mockEncryptor{})
	resp, err := svc.GetAuthURL(&gin.Context{}, 42)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "https://mp/auth?code=xyz", resp.AuthURL)
	assert.True(t, strings.HasPrefix(resp.State, "42-"))
	client.AssertExpectations(t)
}

func TestGetAuthURL_NotConfigured(t *testing.T) {
	client := new(mockMercadoPagoClient)
	client.On("GetAuthURL", mock.Anything, mock.Anything).Return("").Once()

	svc := newTestMPConnectService(&mockSellerConnectionDao{}, client, &mockEncryptor{})
	resp, err := svc.GetAuthURL(&gin.Context{}, 42)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "configuración de Mercado Pago incompleta")
	client.AssertExpectations(t)
}

func TestHandleCallback_ErrorFromMP(t *testing.T) {
	svc := newTestMPConnectService(&mockSellerConnectionDao{}, new(mockMercadoPagoClient), &mockEncryptor{})
	resp, err := svc.HandleCallback(&gin.Context{}, &mpconnect.CallbackRequest{Error: "access_denied", ErrorDescription: "usuario canceló"})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error en autorización: usuario canceló")
}

func TestHandleCallback_MissingCodeOrState(t *testing.T) {
	svc := newTestMPConnectService(&mockSellerConnectionDao{}, new(mockMercadoPagoClient), &mockEncryptor{})

	resp, err := svc.HandleCallback(&gin.Context{}, &mpconnect.CallbackRequest{Code: "", State: ""})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "parámetros code y state requeridos")

	resp, err = svc.HandleCallback(&gin.Context{}, &mpconnect.CallbackRequest{Code: "abc", State: ""})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "parámetros code y state requeridos")
}

func TestHandleCallback_InvalidState(t *testing.T) {
	svc := newTestMPConnectService(&mockSellerConnectionDao{}, new(mockMercadoPagoClient), &mockEncryptor{})
	resp, err := svc.HandleCallback(&gin.Context{}, &mpconnect.CallbackRequest{Code: "abc", State: "not-a-state"})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "state inválido")
}

func TestHandleCallback_ExpiredState(t *testing.T) {
	svc := newTestMPConnectService(&mockSellerConnectionDao{}, new(mockMercadoPagoClient), &mockEncryptor{})
	expired := fmt.Sprintf("%d-%d", 1, time.Now().UnixNano()-11*time.Minute.Nanoseconds())
	resp, err := svc.HandleCallback(&gin.Context{}, &mpconnect.CallbackRequest{Code: "abc", State: expired})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "state inválido")
}

func TestHandleCallback_Success(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, "client-id", "client-secret", "https://redirect", "abc").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk", RefreshToken: "rt", ExpiresIn: 3600}, nil).Once()
	client.On("RefreshAccessToken", mock.Anything, "client-id", "client-secret", "rt").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk2", RefreshToken: "rt2", ExpiresIn: 5400}, nil).Once()
	client.On("GetUserInfo", mock.Anything, "tk2").
		Return(&mercadopagoclient.UserInfoResponse{ID: 123, Nick: "vendedor"}, nil).Once()

	var saved *dbs.SellerConnection
	connDao := &mockSellerConnectionDao{
		upsertFn: func(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error) {
			saved = conn
			return conn, nil
		},
	}
	enc := &mockEncryptor{
		encryptFn: func(p string) (string, error) { return "enc(" + p + ")", nil },
	}

	svc := NewMPConnectService(connDao, client, enc, "client-id", "client-secret", "https://redirect")
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.NotNil(t, saved)
	assert.Equal(t, int64(7), saved.UserID)
	assert.Equal(t, "123", saved.MPUserID)
	assert.Equal(t, "enc(tk2)", saved.AccessToken)
	assert.Equal(t, "enc(rt2)", saved.RefreshToken)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), saved.Status)
	require.NotNil(t, saved.TokenExpiresAt)
	client.AssertExpectations(t)
}

func TestHandleCallback_RefreshKeepsOriginalRefreshToken(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "abc").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk", RefreshToken: "rt", ExpiresIn: 3600}, nil).Once()
	client.On("RefreshAccessToken", mock.Anything, mock.Anything, mock.Anything, "rt").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk2", RefreshToken: "", ExpiresIn: 0}, nil).Once()
	client.On("GetUserInfo", mock.Anything, "tk2").
		Return(&mercadopagoclient.UserInfoResponse{ID: 123}, nil).Once()

	var saved *dbs.SellerConnection
	connDao := &mockSellerConnectionDao{
		upsertFn: func(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error) {
			saved = conn
			return conn, nil
		},
	}

	svc := NewMPConnectService(connDao, client, &mockEncryptor{}, "client-id", "client-secret", "https://redirect")
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, saved)
	assert.Equal(t, "enc(tk2)", saved.AccessToken)
	assert.Equal(t, "enc(rt)", saved.RefreshToken)
	client.AssertExpectations(t)
}

func TestHandleCallback_RefreshError(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "abc").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk", RefreshToken: "rt", ExpiresIn: 3600}, nil).Once()
	client.On("RefreshAccessToken", mock.Anything, mock.Anything, mock.Anything, "rt").
		Return(nil, errors.New("refresh boom")).Once()

	svc := newTestMPConnectService(&mockSellerConnectionDao{}, client, &mockEncryptor{})
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error al refrescar tokens")
	client.AssertExpectations(t)
}

func TestHandleCallback_TokenExchangeError(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "abc").
		Return(nil, errors.New("boom")).Once()

	svc := newTestMPConnectService(&mockSellerConnectionDao{}, client, &mockEncryptor{})
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error al obtener tokens")
	client.AssertExpectations(t)
}

func TestHandleCallback_GetUserInfoError(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "abc").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk", RefreshToken: "rt", ExpiresIn: 3600}, nil).Once()
	client.On("RefreshAccessToken", mock.Anything, mock.Anything, mock.Anything, "rt").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk2", RefreshToken: "rt2", ExpiresIn: 5400}, nil).Once()
	client.On("GetUserInfo", mock.Anything, "tk2").Return(nil, errors.New("userinfo boom")).Once()

	svc := newTestMPConnectService(&mockSellerConnectionDao{}, client, &mockEncryptor{})
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error al obtener info de usuario")
	client.AssertExpectations(t)
}

func TestHandleCallback_EncryptError(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "abc").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk", RefreshToken: "rt", ExpiresIn: 3600}, nil).Once()
	client.On("RefreshAccessToken", mock.Anything, mock.Anything, mock.Anything, "rt").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk2", RefreshToken: "rt2", ExpiresIn: 5400}, nil).Once()
	client.On("GetUserInfo", mock.Anything, "tk2").
		Return(&mercadopagoclient.UserInfoResponse{ID: 123}, nil).Once()

	enc := &mockEncryptor{encryptFn: func(p string) (string, error) { return "", errors.New("crypt boom") }}
	svc := newTestMPConnectService(&mockSellerConnectionDao{}, client, enc)
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error cifrando access_token")
	client.AssertExpectations(t)
}

func TestHandleCallback_UpsertError(t *testing.T) {
	ctx := &gin.Context{}
	state := fmt.Sprintf("%d-%d", 7, time.Now().UnixNano())

	client := new(mockMercadoPagoClient)
	client.On("ExchangeCodeForToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "abc").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk", RefreshToken: "rt", ExpiresIn: 3600}, nil).Once()
	client.On("RefreshAccessToken", mock.Anything, mock.Anything, mock.Anything, "rt").
		Return(&mercadopagoclient.OAuthTokenResponse{AccessToken: "tk2", RefreshToken: "rt2", ExpiresIn: 5400}, nil).Once()
	client.On("GetUserInfo", mock.Anything, "tk2").
		Return(&mercadopagoclient.UserInfoResponse{ID: 123}, nil).Once()

	connDao := &mockSellerConnectionDao{
		upsertFn: func(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error) {
			return nil, errors.New("upsert boom")
		},
	}
	svc := newTestMPConnectService(connDao, client, &mockEncryptor{})
	resp, err := svc.HandleCallback(ctx, &mpconnect.CallbackRequest{Code: "abc", State: state})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error guardando conexión")
	client.AssertExpectations(t)
}

func TestGetStatus_Success(t *testing.T) {
	ctx := &gin.Context{}

	connDao := &mockSellerConnectionDao{
		findByUserFn: func(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
			return &dbs.SellerConnection{UserID: userID, Status: string(constants.SellerConnectionStatusAuthorized)}, nil
		},
	}
	svc := newTestMPConnectService(connDao, new(mockMercadoPagoClient), &mockEncryptor{})
	resp, err := svc.GetStatus(ctx, 5)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Connected)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), resp.AccountStatus)
}

func TestGetStatus_NotConnected(t *testing.T) {
	ctx := &gin.Context{}
	connDao := &mockSellerConnectionDao{
		findByUserFn: func(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
			return &dbs.SellerConnection{UserID: userID, Status: string(constants.SellerConnectionStatusDeauthorized)}, nil
		},
	}
	svc := newTestMPConnectService(connDao, new(mockMercadoPagoClient), &mockEncryptor{})
	resp, err := svc.GetStatus(ctx, 5)
	require.NoError(t, err)
	assert.False(t, resp.Connected)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), resp.AccountStatus)
}

func TestGetStatus_NoConnection(t *testing.T) {
	ctx := &gin.Context{}
	connDao := &mockSellerConnectionDao{
		findByUserFn: func(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
			return nil, nil
		},
	}
	svc := newTestMPConnectService(connDao, new(mockMercadoPagoClient), &mockEncryptor{})
	resp, err := svc.GetStatus(ctx, 5)
	require.NoError(t, err)
	assert.False(t, resp.Connected)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), resp.AccountStatus)
}

func TestGetStatus_DaoError(t *testing.T) {
	ctx := &gin.Context{}
	connDao := &mockSellerConnectionDao{
		findByUserFn: func(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
			return nil, errors.New("boom")
		},
	}
	svc := newTestMPConnectService(connDao, new(mockMercadoPagoClient), &mockEncryptor{})
	resp, err := svc.GetStatus(ctx, 5)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error consultando conexión")
}

func TestHandleDeauthorization_Success(t *testing.T) {
	ctx := &gin.Context{}
	var capturedUser int64
	var capturedStatus string
	connDao := &mockSellerConnectionDao{
		setStatusByMPUserFn: func(ctx *gin.Context, mpUserID int64, status string) error {
			capturedUser = mpUserID
			capturedStatus = status
			return nil
		},
	}
	svc := newTestMPConnectService(connDao, new(mockMercadoPagoClient), &mockEncryptor{})
	err := svc.HandleDeauthorization(ctx, 987)
	assert.NoError(t, err)
	assert.Equal(t, int64(987), capturedUser)
	assert.Equal(t, string(constants.SellerConnectionStatusDeauthorized), capturedStatus)
}

func TestHandleDeauthorization_DaoError(t *testing.T) {
	ctx := &gin.Context{}
	connDao := &mockSellerConnectionDao{
		setStatusByMPUserFn: func(ctx *gin.Context, mpUserID int64, status string) error {
			return errors.New("boom")
		},
	}
	svc := newTestMPConnectService(connDao, new(mockMercadoPagoClient), &mockEncryptor{})
	err := svc.HandleDeauthorization(ctx, 987)
	assert.Error(t, err)
	assert.EqualError(t, err, "error actualizando estado")
}

func TestServiceMPConnectImplementsInterface(t *testing.T) {
	svc := newTestMPConnectService(&mockSellerConnectionDao{}, new(mockMercadoPagoClient), &mockEncryptor{})
	var iface MPConnectServiceInterface = svc
	_ = iface
}
