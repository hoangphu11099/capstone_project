package utils

import (
	"crypto/rand"
	"log"
	"strings"
	"student-management/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func jwtSecret() []byte {
	if secret := strings.TrimSpace(config.GetEnv("JWT_SECRET", "f49d29c3f61e7dca8f18337cf021f8a648511e8ad7e00c2474066701e9e97762")); secret != "" {
		return []byte(secret)
	}

	// Keep local development usable without shipping a universal hard-coded secret.
	// Tokens become invalid after a server restart until JWT_SECRET is configured.
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic("cannot generate temporary JWT secret: " + err.Error())
	}
	log.Print("WARNING: JWT_SECRET is not set; using an ephemeral secret. Configure JWT_SECRET before deployment.")
	return secret
}

var SecretKey = jwtSecret()

type Claims struct {
	UserID uint
	Role   string
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(SecretKey)
}
