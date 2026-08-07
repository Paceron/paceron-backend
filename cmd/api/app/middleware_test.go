package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/config"
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
