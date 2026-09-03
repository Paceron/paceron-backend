package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/mpconnect"
	"simple-arq-golang/cmd/api/infrastructure/crypto"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/mercadopagoclient"
)

// MPConnectServiceInterface define las operaciones de conexión OAuth con Mercado Pago.
type MPConnectServiceInterface interface {
	GetAuthURL(ctx *gin.Context, userID int64) (*mpconnect.AuthURLResponse, error)
	HandleCallback(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error)
	GetStatus(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error)
	HandleDeauthorization(ctx *gin.Context, mpUserID int64) error
}

type mpConnectService struct {
	sellerConnDao daos.SellerConnectionDaoInterface
	mpClient      mercadopagoclient.MercadoPagoClientInterface
	encryptor     crypto.EncryptorInterface
	clientID      string
	clientSecret  string
	redirectURI   string
}

// NewMPConnectService crea una nueva instancia de MPConnectService.
func NewMPConnectService(
	sellerConnDao daos.SellerConnectionDaoInterface,
	mpClient mercadopagoclient.MercadoPagoClientInterface,
	encryptor crypto.EncryptorInterface,
	clientID, clientSecret, redirectURI string,
) MPConnectServiceInterface {
	return &mpConnectService{
		sellerConnDao: sellerConnDao,
		mpClient:      mpClient,
		encryptor:     encryptor,
		clientID:      clientID,
		clientSecret:  clientSecret,
		redirectURI:   redirectURI,
	}
}

// GetAuthURL genera la URL de autorización de Mercado Pago con un state CSRF.
func (s *mpConnectService) GetAuthURL(ctx *gin.Context, userID int64) (*mpconnect.AuthURLResponse, error) {
	// Generar state único para CSRF (userID + timestamp + random)
	state := fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())

	// Usar el redirect_uri configurado
	redirectURI := "https://paceron-frontend.vercel.app/mercadopago/callback"
	authURL := s.mpClient.GetAuthURL(redirectURI, state)
	if authURL == "" {
		customlogger.Error(ctx, "MP OAuth client not configured", fmt.Errorf("missing client ID"),
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetAuthURL"))
		return nil, fmt.Errorf("configuración de Mercado Pago incompleta")
	}

	return &mpconnect.AuthURLResponse{AuthURL: authURL, State: state}, nil
}

// HandleCallback procesa el callback de OAuth de Mercado Pago.
func (s *mpConnectService) HandleCallback(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
	if req.Error != "" {
		customlogger.Warn(ctx, "MP OAuth callback error",
			customlogger.Tag("error", req.Error),
			customlogger.Tag("error_description", req.ErrorDescription),
			customlogger.TagMethod("HandleCallback"))
		return nil, fmt.Errorf("error en autorización: %s", req.ErrorDescription)
	}

	if req.Code == "" || req.State == "" {
		return nil, fmt.Errorf("parámetros code y state requeridos")
	}

	// Validar state: debe pertenecer al usuario autenticado
	// El state tiene formato "userID-timestamp"
	userID, err := s.validateState(ctx, req.State)
	if err != nil {
		customlogger.Warn(ctx, "MP OAuth invalid state",
			customlogger.Tag("state", req.State),
			customlogger.TagMethod("HandleCallback"))
		return nil, fmt.Errorf("state inválido")
	}

	if s.clientID == "" || s.clientSecret == "" || s.redirectURI == "" {
		customlogger.Error(ctx, "MP OAuth credentials not configured", fmt.Errorf("missing credentials"))
		return nil, fmt.Errorf("configuración de Mercado Pago incompleta")
	}

	tokenResp, err := s.mpClient.ExchangeCodeForToken(ctx, s.clientID, s.clientSecret, s.redirectURI, req.Code)
	if err != nil {
		customlogger.Error(ctx, "MP OAuth token exchange failed", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("HandleCallback"))
		return nil, fmt.Errorf("error al obtener tokens: %w", err)
	}

	// Obtener info del usuario de MP para confirmar mp_user_id
	userInfo, err := s.mpClient.GetUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		customlogger.Error(ctx, "MP OAuth get user info failed", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("HandleCallback"))
		return nil, fmt.Errorf("error al obtener info de usuario: %w", err)
	}

	// Cifrar tokens antes de guardar
	encryptedAccessToken, err := s.encryptor.Encrypt(tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("error cifrando access_token")
	}
	encryptedRefreshToken, err := s.encryptor.Encrypt(tokenResp.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("error cifrando refresh_token")
	}

	// Upsert seller_connection
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	conn := &dbs.SellerConnection{
		UserID:          userID,
		MPUserID:        fmt.Sprintf("%d", userInfo.ID),
		AccessToken:     encryptedAccessToken,
		RefreshToken:    encryptedRefreshToken,
		Status:          string(constants.SellerConnectionStatusAuthorized),
		TokenExpiresAt:  &expiresAt,
	}

	_, err = s.sellerConnDao.Upsert(ctx, conn)
	if err != nil {
		customlogger.Error(ctx, "error upserting seller connection", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("HandleCallback"))
		return nil, fmt.Errorf("error guardando conexión")
	}

	return &mpconnect.CallbackResponse{Success: true, Message: "Cuenta de Mercado Pago conectada exitosamente"}, nil
}

// GetStatus devuelve el estado de conexión del entrenador.
func (s *mpConnectService) GetStatus(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error) {
	conn, err := s.sellerConnDao.FindByUser(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding seller connection", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetStatus"))
		return nil, fmt.Errorf("error consultando conexión")
	}

	if conn == nil {
		return &mpconnect.StatusResponse{Connected: false, AccountStatus: string(constants.SellerConnectionStatusDeauthorized)}, nil
	}

	return &mpconnect.StatusResponse{
		Connected:     conn.Status == string(constants.SellerConnectionStatusAuthorized),
		AccountStatus: conn.Status,
	}, nil
}

// HandleDeauthorization procesa la notificación de desautorización desde MP.
// El webhook de MP solo conoce el MP user id del vendedor, por eso se localiza
// la conexión por `mp_user_id` (idempotente: MP reenvía la notificación).
func (s *mpConnectService) HandleDeauthorization(ctx *gin.Context, mpUserID int64) error {
	if err := s.sellerConnDao.SetStatusByMPUser(ctx, mpUserID, string(constants.SellerConnectionStatusDeauthorized)); err != nil {
		customlogger.Error(ctx, "error deauthorizing seller connection", err,
			customlogger.Tag("mp_user_id", fmt.Sprintf("%d", mpUserID)),
			customlogger.TagMethod("HandleDeauthorization"))
		return fmt.Errorf("error actualizando estado")
	}
	return nil
}

// validateState valida que el state pertenece al usuario y no es muy viejo.
func (s *mpConnectService) validateState(ctx *gin.Context, state string) (int64, error) {
	// Formato: "userID-timestamp"
	var userID int64
	var ts int64
	_, err := fmt.Sscanf(state, "%d-%d", &userID, &ts)
	if err != nil {
		return 0, fmt.Errorf("formato de state inválido")
	}

	// Validar que no sea muy viejo (10 min)
	if time.Now().UnixNano()-ts > 10*60*1_000_000_000 {
		return 0, fmt.Errorf("state expirado")
	}

	return userID, nil
}