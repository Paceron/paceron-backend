package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"simple-arq-golang/cmd/api/config"
)

// AccessTokenClaims son los claims del access token. Deliberadamente no incluye datos
// mutables del usuario (nombre, email, teléfono) — solo identidad, sesión y roles.
type AccessTokenClaims struct {
	SessionID string   `json:"sid"`
	Roles     []string `json:"roles"`
	jwt.RegisteredClaims
}

// GenerateAccessToken firma un access token para userID, atado a una sesión (sessionID)
// y con los roles vigentes del usuario al momento de emitirlo.
func GenerateAccessToken(userID int64, sessionID string, roles []string) (string, error) {
	secret := config.JWTSecret
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET no configurado")
	}

	now := time.Now()
	claims := AccessTokenClaims{
		SessionID: sessionID,
		Roles:     roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(config.AccessTokenDuration)),
			Issuer:    config.JWTIssuer,
			Audience:  jwt.ClaimStrings{config.JWTAudience},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseAccessToken valida firma, expiración, issuer y audience de un access token, y
// devuelve sus claims. Wrapper de producción — antes este parseo solo existía en tests.
func ParseAccessToken(tokenString string) (*AccessTokenClaims, error) {
	secret := config.JWTSecret
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET no configurado")
	}

	claims := &AccessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithIssuer(config.JWTIssuer), jwt.WithAudience(config.JWTAudience))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("token inválido")
	}
	return claims, nil
}

// GenerateOpaqueToken genera un refresh token aleatorio de alta entropía. No es un JWT:
// es un secreto opaco sin estructura propia, que solo cobra sentido buscado por su hash
// contra la tabla refresh_tokens — el backend es la única fuente de verdad de a qué
// usuario/sesión corresponde.
func GenerateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("error generando token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken calcula el SHA256 de un token en hex. Se persiste el hash, nunca el token
// en texto plano — es un secreto de alta entropía ya aleatorio, no una contraseña
// elegida por una persona, así que un hash simple y determinístico (buscable por
// igualdad exacta) alcanza; bcrypt sería más lento sin aportar nada acá.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
