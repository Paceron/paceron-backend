package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/utils"
)

func TestParseSessionChannel(t *testing.T) {
	cases := []struct {
		channel string
		wantID  int64
		wantOK  bool
	}{
		{"session:12", 12, true},
		{"session:0", 0, false},
		{"session:-3", 0, false},
		{"session:abc", 0, false},
		{"session:7abc", 0, false},
		{"session:07", 0, false},
		{"session:+7", 0, false},
		{"session: 7", 0, false},
		{"session:1e3", 0, false},
		{"session:", 0, false},
		{"session", 0, false},
		{"team:12", 0, false},
		{"", 0, false},
		{"Session:12", 0, false},
	}
	for _, tc := range cases {
		id, ok := parseSessionChannel(tc.channel)
		assert.Equal(t, tc.wantOK, ok, "canal %q", tc.channel)
		assert.Equal(t, tc.wantID, id, "canal %q", tc.channel)
	}
}

// channelAuthorizerStubDAO implementa daos.SessionInstanceDaoInterface
// completo: solo HasInstanceAccess registra la consulta.
type channelAuthorizerStubDAO struct {
	lastInstanceID int64
	lastCallerID   int64
	queried        bool
}

func (s *channelAuthorizerStubDAO) HasInstanceAccess(_ *gin.Context, instanceID, callerID int64) (bool, error) {
	s.queried = true
	s.lastInstanceID, s.lastCallerID = instanceID, callerID
	return true, nil
}

func (s *channelAuthorizerStubDAO) Create(*gin.Context, *dbs.SessionInstance) error { return nil }
func (s *channelAuthorizerStubDAO) FindByID(*gin.Context, int64) (*dbs.SessionInstance, error) {
	return nil, nil
}
func (s *channelAuthorizerStubDAO) FindByIDs(*gin.Context, []int64) ([]dbs.SessionInstance, error) {
	return nil, nil
}
func (s *channelAuthorizerStubDAO) Delete(*gin.Context, int64) error { return nil }
func (s *channelAuthorizerStubDAO) HasFeedback(*gin.Context, int64) (bool, error) {
	return false, nil
}

func TestChannelAuthorizer(t *testing.T) {
	stub := &channelAuthorizerStubDAO{}
	authorizer := newChannelAuthorizer(stub)

	// Canales sin patrón / patrón inválido: ni toca el DAO.
	for _, channel := range []string{"team:5", "session:abc", "session:0", "session:"} {
		ok, err := authorizer.Authorize(channel, 42)
		assert.False(t, ok, "canal %q", channel)
		require.NoError(t, err, "canal %q", channel)
	}
	assert.False(t, stub.queried)

	// session:{id} válido delega en HasInstanceAccess.
	ok, err := authorizer.Authorize("session:7", 42)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(7), stub.lastInstanceID)
	assert.Equal(t, int64(42), stub.lastCallerID)
}

func setupWsUpgradeTest(t *testing.T) gin.HandlerFunc {
	t.Helper()
	oldSecret, oldIssuer, oldAudience, oldDuration := config.JWTSecret, config.JWTIssuer, config.JWTAudience, config.AccessTokenDuration
	t.Cleanup(func() {
		config.JWTSecret, config.JWTIssuer, config.JWTAudience, config.AccessTokenDuration = oldSecret, oldIssuer, oldAudience, oldDuration
	})
	config.JWTSecret = "test-secret-key-for-testing"
	config.JWTIssuer = "paceron-backend"
	config.JWTAudience = "paceron-app"
	config.AccessTokenDuration = 15 * time.Minute

	gateway := realtime.NewGateway(realtime.NewHub(), newChannelAuthorizer(&channelAuthorizerStubDAO{}), allowedOrigins())
	return wsUpgrade(gateway)
}

func TestWSUpgradeMissingToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil)

	setupWsUpgradeTest(t)(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "falta el query param token")
}

func TestWSUpgradeInvalidToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/ws?token=garbage", nil)

	setupWsUpgradeTest(t)(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"unauthorized"`)
}

func TestWSUpgradeExpiredToken(t *testing.T) {
	handler := setupWsUpgradeTest(t)
	config.AccessTokenDuration = -1 * time.Minute
	t.Cleanup(func() { config.AccessTokenDuration = 15 * time.Minute })
	expired, err := utils.GenerateAccessToken(42, "sess-1", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+expired, nil)

	handler(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "token_expired")
}

func TestWSUpgradeValidTokenRejectsNonWSHandshake(t *testing.T) {
	handler := setupWsUpgradeTest(t)
	token, err := utils.GenerateAccessToken(42, "sess-1", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+token, nil)

	handler(c)

	// El handler Upgrade devuelve error de handshake (no es un request WS) sin
	// cortar nada: la respuesta HTTP de error la escribió el upgrader/gorilla.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
