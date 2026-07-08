package utils

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/config"
)

func TestGenerateAccessToken_Success(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	token, err := GenerateAccessToken(1, "john@test.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := jwt.ParseWithClaims(token, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecret), nil
	})
	assert.NoError(t, err)

	claims, ok := parsed.Claims.(*AccessTokenClaims)
	assert.True(t, ok)
	assert.Equal(t, "1", claims.Subject)
	assert.Equal(t, "john@test.com", claims.Email)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)
}

func TestGenerateAccessToken_MissingSecret(t *testing.T) {
	config.JWTSecret = ""
	_, err := GenerateAccessToken(1, "john@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET no configurado")
}

func TestGenerateRefreshToken_Success(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	token, err := GenerateRefreshToken(1)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := jwt.ParseWithClaims(token, &RefreshTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecret), nil
	})
	assert.NoError(t, err)

	claims, ok := parsed.Claims.(*RefreshTokenClaims)
	assert.True(t, ok)
	assert.Equal(t, "1", claims.Subject)
	assert.Equal(t, "refresh", claims.Type)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)
}

func TestGenerateRefreshToken_MissingSecret(t *testing.T) {
	config.JWTSecret = ""
	_, err := GenerateRefreshToken(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET no configurado")
}

func TestAccessTokenExpiration(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	token, err := GenerateAccessToken(1, "john@test.com")
	assert.NoError(t, err)

	parsed, err := jwt.ParseWithClaims(token, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecret), nil
	})
	assert.NoError(t, err)

	claims := parsed.Claims.(*AccessTokenClaims)
	exp := claims.ExpiresAt.Time
	iat := claims.IssuedAt.Time
	assert.Equal(t, int64(3600), int64(exp.Sub(iat).Seconds()))
}

func TestRefreshTokenExpiration(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	token, err := GenerateRefreshToken(1)
	assert.NoError(t, err)

	parsed, err := jwt.ParseWithClaims(token, &RefreshTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecret), nil
	})
	assert.NoError(t, err)

	claims := parsed.Claims.(*RefreshTokenClaims)
	exp := claims.ExpiresAt.Time
	iat := claims.IssuedAt.Time
	assert.Equal(t, int64(604800), int64(exp.Sub(iat).Seconds()))
}
