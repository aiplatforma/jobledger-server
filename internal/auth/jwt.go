package auth

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("signingsecret")

func GenerateUserToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"expire":   time.Now().Add(6 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

const (
	DURATION_DAY          = time.Hour * 24
	DURATION_WEEK         = time.Hour * 24 * 7
	DURATION_MONTH        = time.Hour * 24 * 30
	DURATION_THREE_MONTHS = time.Hour * 24 * 30 * 3
	DURATION_SIX_MONTHS   = time.Hour * 24 * 30 * 6
)

func GenerateAppToken(comment string, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"comment": comment,
		"expire":  time.Now().Add(duration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func ValidateToken(tokenStr string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return jwtKey, nil
	})
	if err != nil {
		slog.Error("Failed to validate jwt token", "auth", "user", "err", err)
		return nil, fmt.Errorf("failed to validate jwt token: %w", err)
	}
	if !token.Valid {
		slog.Info("Token not valid", "auth", "user", "err", err)
		return nil, fmt.Errorf("token not valid: %w", err)
	}

	expireFloat, ok := claims["expire"].(float64)
	if !ok {
		slog.Error("Failed to cast expiration claim", "auth", "user")
		return nil, fmt.Errorf("failed to cast expiration claim")
	}

	expire := time.Unix(int64(expireFloat), 0)
	if time.Now().After(expire) {
		slog.Info("Token expired", "auth", "user", "err", err)
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}
