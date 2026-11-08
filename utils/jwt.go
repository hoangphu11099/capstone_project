package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const TokenIssuer = "pp-academy"
const TokenAudience = "pp-academy-clients"
const AccessTokenLifetime = 24 * time.Hour

var keyOnce sync.Once
var signingKey []byte
var signingKeyError error

// Initialize after main has loaded .env. Never share a hard-coded key.
func InitJWT() error {
	keyOnce.Do(func() {
		secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if secret != "" {
			if len(secret) < 32 || secret == "f49d29c3f61e7dca8f18337cf021f8a648511e8ad7e00c2474066701e9e97762" {
				signingKeyError = errors.New("JWT_SECRET phải có ít nhất 32 ký tự ngẫu nhiên và không dùng khóa mẫu công khai")
				return
			}
			signingKey = []byte(secret)
			return
		}
		if os.Getenv("GIN_MODE") == "release" || os.Getenv("APP_ENV") == "production" {
			signingKeyError = errors.New("phải cấu hình JWT_SECRET khi triển khai production")
			return
		}
		signingKey = make([]byte, 32)
		if _, err := rand.Read(signingKey); err != nil {
			signingKeyError = err
			return
		}
		log.Print("JWT_SECRET chưa được cấu hình; dùng khóa ngẫu nhiên cho phiên chạy phát triển hiện tại")
	})
	return signingKeyError
}

type Claims struct {
	UserID uint
	Role   string
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, role string) (string, error) {
	if err := InitJWT(); err != nil {
		return "", err
	}
	if userID == 0 || (role != "admin" && role != "teacher" && role != "student") {
		return "", errors.New("invalid token identity")
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: TokenIssuer, Audience: jwt.ClaimStrings{TokenAudience}, Subject: strconv.FormatUint(uint64(userID), 10), ID: hex.EncodeToString(random),
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenLifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

func ParseToken(value string) (*Claims, error) {
	if err := InitJWT(); err != nil {
		return nil, err
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (interface{}, error) { return signingKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(), jwt.WithIssuer(TokenIssuer), jwt.WithAudience(TokenAudience), jwt.WithIssuedAt())
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.UserID == 0 || claims.ID == "" || claims.IssuedAt == nil || claims.Subject != strconv.FormatUint(uint64(claims.UserID), 10) || (claims.Role != "admin" && claims.Role != "teacher" && claims.Role != "student") {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
