package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/mpconnect"
)

type mockMPConnectService struct {
	getAuthURLFn          func(ctx *gin.Context, userID int64, target string) (*mpconnect.AuthURLResponse, error)
	handleCallbackFn      func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error)
	getStatusFn           func(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error)
	handleDeauthWebhookFn func(ctx *gin.Context, mpUserID int64) error
}

func (m *mockMPConnectService) GetAuthURL(ctx *gin.Context, userID int64, target string) (*mpconnect.AuthURLResponse, error) {
	if m.getAuthURLFn != nil {
		return m.getAuthURLFn(ctx, userID, target)
	}
	return nil, nil
}

func (m *mockMPConnectService) HandleCallback(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
	if m.handleCallbackFn != nil {
		return m.handleCallbackFn(ctx, req)
	}
	return nil, nil
}

func (m *mockMPConnectService) GetStatus(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockMPConnectService) HandleDeauthorization(ctx *gin.Context, mpUserID int64) error {
	if m.handleDeauthWebhookFn != nil {
		return m.handleDeauthWebhookFn(ctx, mpUserID)
	}
	return nil
}

const (
	testWebReturnURL = "https://front.test/mp-connect/callback"
	testAppReturnURL = "paceron-test://mp-connect/callback"
)

func newTestMPConnectController(svc *mockMPConnectService) MPConnectControllerInterface {
	return NewMPConnectController(svc, testWebReturnURL, testAppReturnURL)
}

// newCallbackRequest arma el contexto de un callback de MP: es una navegación
// del navegador, sin header Authorization.
func newCallbackRequest(query string) (*gin.Context, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/callback?"+query, nil)
	return c, response
}

func TestMPConnectController_GetAuthURL_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		getAuthURLFn: func(ctx *gin.Context, userID int64, target string) (*mpconnect.AuthURLResponse, error) {
			return &mpconnect.AuthURLResponse{AuthURL: "https://auth.mercadopago.com/authorization?client_id=1", State: "123-456"}, nil
		},
	}

	controller := newTestMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect", nil)
	setAuthUserID(c, 1)

	controller.GetAuthURL(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result mpconnect.AuthURLResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "https://auth.mercadopago.com/authorization?client_id=1", result.AuthURL)
	assert.Equal(t, "123-456", result.State)
}

// El platform de la query es lo que decide a dónde vuelve el navegador después
// del callback, así que tiene que llegar al service tal cual.
func TestMPConnectController_GetAuthURL_PropagaPlatform(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{name: "platform app", query: "?platform=app", want: mpconnect.TargetApp},
		{name: "platform web explícito", query: "?platform=web", want: mpconnect.TargetWeb},
		{name: "sin platform", query: "", want: mpconnect.TargetWeb},
		{name: "platform desconocido llega crudo y lo normaliza el dominio", query: "?platform=escritorio", want: "escritorio"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received string
			mockSvc := &mockMPConnectService{
				getAuthURLFn: func(ctx *gin.Context, userID int64, target string) (*mpconnect.AuthURLResponse, error) {
					received = target
					return &mpconnect.AuthURLResponse{AuthURL: "https://mp/auth", State: "1-2-web"}, nil
				},
			}

			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect"+tc.query, nil)
			setAuthUserID(c, 1)

			newTestMPConnectController(mockSvc).GetAuthURL(c)

			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, tc.want, received)
		})
	}
}

func TestMPConnectController_GetAuthURL_Unauthorized(t *testing.T) {
	controller := newTestMPConnectController(&mockMPConnectService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect", nil)

	controller.GetAuthURL(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestMPConnectController_GetAuthURL_ServiceError(t *testing.T) {
	mockSvc := &mockMPConnectService{
		getAuthURLFn: func(ctx *gin.Context, userID int64, target string) (*mpconnect.AuthURLResponse, error) {
			return nil, errors.New("configuración de Mercado Pago incompleta")
		},
	}

	controller := newTestMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect", nil)
	setAuthUserID(c, 1)

	controller.GetAuthURL(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

// assertRedirect verifica que la respuesta sea un 302 al destino esperado y
// devuelve la query del Location ya parseada — comparar el string entero sería
// frágil porque url.Values.Encode() ordena las claves alfabéticamente.
func assertRedirect(t *testing.T, response *httptest.ResponseRecorder, expectedBase string) url.Values {
	t.Helper()

	assert.Equal(t, http.StatusFound, response.Code)
	location := response.Header().Get("Location")
	parsed, err := url.Parse(location)
	assert.NoError(t, err)

	base := *parsed
	base.RawQuery = ""
	assert.Equal(t, expectedBase, base.String())

	return parsed.Query()
}

func TestMPConnectController_HandleCallback_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return &mpconnect.CallbackResponse{Success: true, Message: "connected"}, nil
		},
	}

	c, response := newCallbackRequest("code=CODE&state=" + mpconnect.BuildState(1, 123, mpconnect.TargetWeb))
	newTestMPConnectController(mockSvc).HandleCallback(c)

	query := assertRedirect(t, response, testWebReturnURL)
	assert.Equal(t, "success", query.Get("status"))
	assert.Empty(t, query.Get("reason"))
	// El code de autorización no puede viajar a una URL del frontend: termina en
	// el historial del navegador y en los logs de acceso del hosting.
	assert.Empty(t, query.Get("code"))
}

func TestMPConnectController_HandleCallback_ServiceError(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return nil, errors.New("state inválido")
		},
	}

	c, response := newCallbackRequest("code=&state=bad")
	newTestMPConnectController(mockSvc).HandleCallback(c)

	query := assertRedirect(t, response, testWebReturnURL)
	assert.Equal(t, "error", query.Get("status"))
	assert.Equal(t, "invalid_state", query.Get("reason"))
}

// El target del state decide el destino: desde la app nativa hay que volver por
// deep link, no al origen web.
func TestMPConnectController_HandleCallback_TargetApp(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return &mpconnect.CallbackResponse{Success: true}, nil
		},
	}

	c, response := newCallbackRequest("code=CODE&state=" + mpconnect.BuildState(1, 123, mpconnect.TargetApp))
	newTestMPConnectController(mockSvc).HandleCallback(c)

	query := assertRedirect(t, response, testAppReturnURL)
	assert.Equal(t, "success", query.Get("status"))
}

// Un state ilegible igual tiene que terminar en una pantalla de resultado: cae a
// web, que es el destino por defecto.
func TestMPConnectController_HandleCallback_StateMalformadoCaeAWeb(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return nil, errors.New("formato de state inválido")
		},
	}

	c, response := newCallbackRequest("code=CODE&state=basura")
	newTestMPConnectController(mockSvc).HandleCallback(c)

	query := assertRedirect(t, response, testWebReturnURL)
	assert.Equal(t, "invalid_state", query.Get("reason"))
}

// Guard: un entorno sin URLs de retorno configuradas degrada al JSON de antes,
// en vez de redirigir a "" y dejar al usuario en una página en blanco.
func TestMPConnectController_HandleCallback_SinReturnURLs_RespondeJSON(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return &mpconnect.CallbackResponse{Success: true, Message: "connected"}, nil
		},
	}

	c, response := newCallbackRequest("code=CODE&state=1-123-web")
	NewMPConnectController(mockSvc, "", "").HandleCallback(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result mpconnect.CallbackResponse
	assert.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	assert.True(t, result.Success)
}

func TestMPConnectController_HandleCallback_SinReturnURLs_ErrorRespondeJSON(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return nil, errors.New("state inválido")
		},
	}

	c, response := newCallbackRequest("code=CODE&state=bad")
	NewMPConnectController(mockSvc, "", "").HandleCallback(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

// La URL de retorno puede traer query propia (ej. un flag de entorno); el status
// se suma sin pisarla.
func TestMPConnectController_HandleCallback_PreservaQueryDeLaBase(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleCallbackFn: func(ctx *gin.Context, req *mpconnect.CallbackRequest) (*mpconnect.CallbackResponse, error) {
			return &mpconnect.CallbackResponse{Success: true}, nil
		},
	}

	c, response := newCallbackRequest("code=CODE&state=1-123-web")
	NewMPConnectController(mockSvc, "https://front.test/cb?env=preview", "").HandleCallback(c)

	query := assertRedirect(t, response, "https://front.test/cb")
	assert.Equal(t, "preview", query.Get("env"))
	assert.Equal(t, "success", query.Get("status"))
}

func TestMapCallbackReason(t *testing.T) {
	cases := []struct {
		err  string
		want string
	}{
		{err: "parámetros code y state requeridos", want: "missing_params"},
		{err: "state inválido", want: "invalid_state"},
		{err: "formato de state inválido", want: "invalid_state"},
		{err: "state expirado", want: "expired_state"},
		{err: "error en autorización: usuario canceló", want: "authorization_denied"},
		{err: "configuración de Mercado Pago incompleta", want: "config_error"},
		{err: "error al obtener tokens: boom", want: "exchange_failed"},
		{err: "error al refrescar tokens: boom", want: "exchange_failed"},
		{err: "error al obtener info de usuario: boom", want: "exchange_failed"},
		{err: "error cifrando access_token", want: "save_failed"},
		{err: "error cifrando refresh_token", want: "save_failed"},
		{err: "error guardando conexión", want: "save_failed"},
		{err: "algo totalmente inesperado", want: "unknown_error"},
	}

	for _, tc := range cases {
		t.Run(tc.err, func(t *testing.T) {
			assert.Equal(t, tc.want, mapCallbackReason(errors.New(tc.err)))
		})
	}
}

func TestMPConnectController_GetStatus_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		getStatusFn: func(ctx *gin.Context, userID int64) (*mpconnect.StatusResponse, error) {
			return &mpconnect.StatusResponse{Connected: true, AccountStatus: "authorized"}, nil
		},
	}

	controller := newTestMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/status", nil)
	setAuthUserID(c, 1)

	controller.GetStatus(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result mpconnect.StatusResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.True(t, result.Connected)
	assert.Equal(t, "authorized", result.AccountStatus)
}

func TestMPConnectController_GetStatus_Unauthorized(t *testing.T) {
	controller := newTestMPConnectController(&mockMPConnectService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mercadopago/connect/status", nil)

	controller.GetStatus(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestMPConnectController_HandleDeauthWebhook_Success(t *testing.T) {
	mockSvc := &mockMPConnectService{
		handleDeauthWebhookFn: func(ctx *gin.Context, mpUserID int64) error {
			assert.Equal(t, int64(987), mpUserID)
			return nil
		},
	}

	controller := newTestMPConnectController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"user_id":987}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/mercadopago/webhook/connect", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.HandleDeauthWebhook(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestMPConnectController_HandleDeauthWebhook_InvalidBody(t *testing.T) {
	controller := newTestMPConnectController(&mockMPConnectService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/mercadopago/webhook/connect", strings.NewReader(`{"user_id":0}`))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.HandleDeauthWebhook(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}