package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"simple-arq-golang/cmd/api/config"
)

type AccessTokenClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type RefreshTokenClaims struct {
	Type string `json:"type"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userID int64, email string) (string, error) {
	secret := config.JWTSecret
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET no configurado")
	}

	now := time.Now()
	claims := AccessTokenClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func GenerateRefreshToken(userID int64) (string, error) {
	secret := config.JWTSecret
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET no configurado")
	}

	now := time.Now()
	claims := RefreshTokenClaims{
		Type: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
