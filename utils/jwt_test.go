package utils

import (
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"testing"
	"time"
)

func TestJWTValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-signing-secret-with-at-least-32-characters")
	raw, err := GenerateToken(7, "admin")
	if err != nil {
		t.Fatal(err)
	}
	base, err := ParseToken(raw)
	if err != nil {
		t.Fatal(err)
	}
	if base.UserID != 7 || base.Subject != "7" || base.ID == "" || base.IssuedAt == nil || base.ExpiresAt == nil {
		t.Fatal("incomplete identity or validity period")
	}
	other, _ := GenerateToken(7, "admin")
	claims, _ := ParseToken(other)
	if claims.ID == base.ID {
		t.Fatal("login sessions must have different IDs")
	}
	for _, tc := range []struct {
		name   string
		edit   func(*Claims)
		method jwt.SigningMethod
	}{
		{"expired", func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }, jwt.SigningMethodHS256},
		{"no expiry", func(c *Claims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256},
		{"no issued at", func(c *Claims) { c.IssuedAt = nil }, jwt.SigningMethodHS256},
		{"wrong issuer", func(c *Claims) { c.Issuer = "another-service" }, jwt.SigningMethodHS256},
		{"wrong audience", func(c *Claims) { c.Audience = jwt.ClaimStrings{"another-app"} }, jwt.SigningMethodHS256},
		{"wrong subject", func(c *Claims) { c.Subject = "99" }, jwt.SigningMethodHS256},
		{"no session", func(c *Claims) { c.ID = "" }, jwt.SigningMethodHS256},
		{"invalid role", func(c *Claims) { c.Role = "superuser" }, jwt.SigningMethodHS256},
		{"not active yet", func(c *Claims) { c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour)) }, jwt.SigningMethodHS256},
		{"different HMAC algorithm", func(c *Claims) {}, jwt.SigningMethodHS512},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := *base
			tc.edit(&copy)
			value, err := jwt.NewWithClaims(tc.method, copy).SignedString(signingKey)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseToken(value); err == nil {
				t.Fatal("accepted invalid JWT")
			}
		})
	}
	for _, value := range []string{"", "not-a-jwt", raw[:strings.LastIndex(raw, ".")+1] + "tampered-signature"} {
		if _, err := ParseToken(value); err == nil {
			t.Fatal("accepted malformed or modified token")
		}
	}
	if _, err := GenerateToken(0, "admin"); err == nil {
		t.Fatal("accepted missing user")
	}
}
