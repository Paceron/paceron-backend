package app

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/utils"
)

func TestSetRequestId(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: make(http.Header)}

	SetRequestID()(ctx)

	requestID, exists := ctx.Get(_XrequestID)
	assert.True(t, exists)
	assert.NotEmpty(t, requestID)
}

func TestSetRequestId_ExistingHeader(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: http.Header{_XrequestID: []string{"existing-id"}}}

	SetRequestID()(ctx)

	requestID, _ := ctx.Get(_XrequestID)
	assert.Equal(t, "existing-id", requestID)
}

func setTestAuthConfig() {
	config.JWTSecret = "test-secret"
	config.JWTIssuer = "paceron-backend"
	config.JWTAudience = "paceron-app"
	config.AccessTokenDuration = 15 * time.Minute
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	setTestAuthConfig()
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: make(http.Header)}

	AuthMiddleware()(ctx)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Contains(t, response.Body.String(), "unauthorized")
	assert.True(t, ctx.IsAborted())
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	setTestAuthConfig()
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: http.Header{"Authorization": []string{"NotBearer sometoken"}}}

	AuthMiddleware()(ctx)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.True(t, ctx.IsAborted())
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	setTestAuthConfig()
	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: http.Header{"Authorization": []string{"Bearer not.a.valid.token"}}}

	AuthMiddleware()(ctx)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Contains(t, response.Body.String(), "unauthorized")
	assert.True(t, ctx.IsAborted())
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	setTestAuthConfig()
	config.AccessTokenDuration = -1 * time.Minute
	defer setTestAuthConfig()

	token, err := utils.GenerateAccessToken(42, "session-1", []string{"corredor"})
	assert.NoError(t, err)

	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: http.Header{"Authorization": []string{"Bearer " + token}}}

	AuthMiddleware()(ctx)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Contains(t, response.Body.String(), "token_expired")
	assert.True(t, ctx.IsAborted())
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	setTestAuthConfig()
	token, err := utils.GenerateAccessToken(42, "session-1", []string{"corredor", "entrenador"})
	assert.NoError(t, err)

	gin.SetMode(gin.ReleaseMode)
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = &http.Request{Header: http.Header{"Authorization": []string{"Bearer " + token}}}

	AuthMiddleware()(ctx)

	assert.False(t, ctx.IsAborted())
	assert.Equal(t, http.StatusOK, response.Code)

	userID, exists := ctx.Get(utils.AuthUserIDKey)
	assert.True(t, exists)
	assert.Equal(t, int64(42), userID)

	sessionID, exists := ctx.Get(utils.AuthSessionIDKey)
	assert.True(t, exists)
	assert.Equal(t, "session-1", sessionID)

	roles, exists := ctx.Get(utils.AuthRolesKey)
	assert.True(t, exists)
	assert.Equal(t, []string{"corredor", "entrenador"}, roles)
}

func TestMaxBodySize_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(MaxBodySize())
	router.POST("/bind", func(c *gin.Context) {
		var dto map[string]string
		if err := c.ShouldBindJSON(&dto); err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "Cuerpo de solicitud inválido",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{})
	})

	oversized := `{"d":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Bad request")
}

func TestMaxBodySize_AllowsBodyUnderLimit(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(MaxBodySize())
	router.POST("/bind", func(c *gin.Context) {
		var dto map[string]string
		if err := c.ShouldBindJSON(&dto); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": dto["k"]})
	})

	req := httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(`{"k":"v"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "v")
}

func newIPRateTestContext(ip string) (*gin.Context, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	ctx.Request.RemoteAddr = ip + ":55555"
	return ctx, response
}

func TestIPRateLimiter_ExceedsLimitGets429(t *testing.T) {
	limiter := newIPRateLimiter()
	handler := ipRateLimiterHandler(limiter)

	var lastRecorder *httptest.ResponseRecorder
	var lastCtx *gin.Context
	for i := 0; i <= ipRateBurst; i++ {
		lastCtx, lastRecorder = newIPRateTestContext("9.9.9.9")
		handler(lastCtx)
		if i < ipRateBurst {
			assert.False(t, lastCtx.IsAborted())
		}
	}

	assert.Equal(t, http.StatusTooManyRequests, lastRecorder.Code)
	assert.True(t, lastCtx.IsAborted())
	assert.Contains(t, lastRecorder.Body.String(), "too_many_requests")

	// Otra IP no comparte el tope del taller.
	ctx, response := newIPRateTestContext("8.8.8.8")
	handler(ctx)
	assert.False(t, ctx.IsAborted())
	assert.Equal(t, http.StatusOK, response.Code)
}

// TestWebhookAndWSWithoutRateLimiter prueba el wiring completo: rutas reales con
// mapUrls — las de webhooks de MP y ws NO van a 429 aunque la IP ya venía
// quemando crédito; son públicas a propósito (MP reentiende).
func TestWebhookAndWSWithoutRateLimiter(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	app := NewApplication()
	mapUrls(router, app)

	for i := 0; i < ipRateBurst*2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusTooManyRequests, w.Code, "webhook de pagos no debe pasar por el rate limiter")

		req = httptest.NewRequest(http.MethodPost, "/api/v1/mercadopago/webhook/connect", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusTooManyRequests, w.Code, "webhook de MP Connect no debe pasar por el rate limiter")

		req = httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusTooManyRequests, w.Code, "el upgrade ws no debe pasar por el rate limiter")
	}
}

// wiring de S9: un body gigante sobre una ruta real bindeada con BindJSON
// Gin 1.12 detecta \*http.MaxBytesError y responde 413 (no 500 ni panic).
func TestOversizedJSONRouteGives413Not500(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	app := NewApplication()
	mapUrls(router, app)

	oversized := `{"email":"a@b.c","password":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func newIPRateTestRouter(limiter *ipRateLimiter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	_ = router.SetTrustedProxies(trustedProxies)
	router.POST("/check", ipRateLimiterHandler(limiter), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"client_ip": c.ClientIP()})
	})
	return router
}

// El algoritmo de Gin: XFF solo se consulta si RemoteAddr es proxy de
// confianza; el walk va de derecha a izquierda saltando entradas
// privadas/loopback (proxy interno del LB) y devuelve el primer IP público.
func TestIPRateLimiter_ForgedXFFFromUntrustedPeerDoesNotEvade(t *testing.T) {
	router := newIPRateTestRouter(newIPRateLimiter())

	var lastRecorder *httptest.ResponseRecorder
	var lastBody string
	for i := 0; i <= ipRateBurst; i++ {
		// Peer remota NO confiable (no es RFC1918 ni loopback). Cada request
		// forja un XFF distinto: si el limiter usara el XFF, cada una caería en
		// un bucket distinto y nunca habría 429.
		req := httptest.NewRequest(http.MethodPost, "/check", nil)
		req.RemoteAddr = "203.0.113.50:1000"
		req.Header.Set("X-Forwarded-For", "1.1.1."+strconv.Itoa(i))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		lastRecorder = w
		lastBody = w.Body.String()
	}

	assert.Equal(t, http.StatusTooManyRequests, lastRecorder.Code, "el XFF forjado no debe dar bucket nuevo: %s", lastBody)

	// La misma RemoteAddr sin forjar XFF comparte el mismo bucket.
	req := httptest.NewRequest(http.MethodPost, "/check", nil)
	req.RemoteAddr = "203.0.113.50:1000"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestIPRateLimiter_PrivateProxyXFFResolvesRealClient(t *testing.T) {
	router := newIPRateTestRouter(newIPRateLimiter())

	// Proxy interno RFC1918 (peers confiables) que ya appendió el IP real del
	// cliente a la derecha: con walk-from-right se recupera 203.0.113.7.
	req := httptest.NewRequest(http.MethodPost, "/check", nil)
	req.RemoteAddr = "10.42.0.7:1000"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.42.0.7")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "203.0.113.7")

	// Request directa desde localhost con XFF forjado: loopback NO está en los
	// proxies de confianza (RFC1918), así que el XFF se ignora y ClientIP
	// queda en 127.0.0.1 — el spoofing de IP lejana no rota bucket.
	req = httptest.NewRequest(http.MethodPost, "/check", nil)
	req.RemoteAddr = "127.0.0.1:1000"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "127.0.0.1")
}
