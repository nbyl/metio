package agent

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

type AgentClaims struct {
	jwt.RegisteredClaims
}

var jwtSigningKey []byte

func init() {
	key := viper.GetString("AGENT_JWT_SECRET")
	if key != "" {
		jwtSigningKey = []byte(key)
	}
}

func MintToken(instanceName string) (string, error) {
	now := time.Now()
	claims := AgentClaims{
		jwt.RegisteredClaims{
			Subject:   instanceName,
			Issuer:    "controller",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(90 * 24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSigningKey)
}

func SetSigningKey(key []byte) {
	jwtSigningKey = key
}

func VerifyToken(tokenStr string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AgentClaims{}, func(token *jwt.Token) (interface{}, error) {
		if method, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || method.Name != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSigningKey, nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*AgentClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token claims")
	}

	return claims.Subject, nil
}
