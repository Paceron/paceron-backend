package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/config"
)

func setTestJWTConfig() {
	config.JWTSecret = "test-secret-key-for-testing"
	config.JWTIssuer = "paceron-backend"
	config.JWTAudience = "paceron-app"
	config.AccessTokenDuration = 15 * time.Minute
}

func TestGenerateAccessToken_Success(t *testing.T) {
	setTestJWTConfig()
	token, err := GenerateAccessToken(1, "session-123", []string{"corredor"})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := ParseAccessToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "1", claims.Subject)
	assert.Equal(t, "session-123", claims.SessionID)
	assert.Equal(t, []string{"corredor"}, claims.Roles)
	assert.Equal(t, "paceron-backend", claims.Issuer)
	assert.Contains(t, claims.Audience, "paceron-app")
}

func TestGenerateAccessToken_MissingSecret(t *testing.T) {
	setTestJWTConfig()
	config.JWTSecret = ""
	_, err := GenerateAccessToken(1, "session-123", []string{"corredor"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET no configurado")
}

func TestAccessTokenExpiration(t *testing.T) {
	setTestJWTConfig()
	token, err := GenerateAccessToken(1, "session-123", []string{"corredor"})
	assert.NoError(t, err)

	claims, err := ParseAccessToken(token)
	assert.NoError(t, err)

	exp := claims.ExpiresAt.Time
	iat := claims.IssuedAt.Time
	assert.Equal(t, int64(900), int64(exp.Sub(iat).Seconds()))
}

func TestParseAccessToken_WrongIssuer(t *testing.T) {
	setTestJWTConfig()
	token, err := GenerateAccessToken(1, "session-123", []string{"corredor"})
	assert.NoError(t, err)

	config.JWTIssuer = "otro-issuer"
	_, err = ParseAccessToken(token)
	assert.Error(t, err)
}

func TestParseAccessToken_InvalidToken(t *testing.T) {
	setTestJWTConfig()
	_, err := ParseAccessToken("not-a-valid-token")
	assert.Error(t, err)
}

func TestGenerateOpaqueToken_Success(t *testing.T) {
	token, err := GenerateOpaqueToken()
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenerateOpaqueToken_Unique(t *testing.T) {
	token1, err := GenerateOpaqueToken()
	assert.NoError(t, err)
	token2, err := GenerateOpaqueToken()
	assert.NoError(t, err)
	assert.NotEqual(t, token1, token2)
}

func TestHashToken_Deterministic(t *testing.T) {
	hash1 := HashToken("mismo-token")
	hash2 := HashToken("mismo-token")
	assert.Equal(t, hash1, hash2)
}

func TestHashToken_DifferentInputsDifferentHashes(t *testing.T) {
	assert.NotEqual(t, HashToken("token-a"), HashToken("token-b"))
}
