package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetTrustedProxies(nil)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	r.POST("/login", loginRateLimit(func() time.Time { return now }), func(c *gin.Context) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if c.ShouldBindJSON(&body) != nil {
			c.Status(400)
			return
		}
		if body.Password == "correct" {
			c.Status(200)
		} else if body.Password == "server-error" {
			c.Status(500)
		} else {
			c.Status(401)
		}
	})
	requestLogin := func(username, password, ip string, want int) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest("POST", "/login", bytes.NewBufferString(`{"username":"`+username+`","password":"`+password+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = ip + ":1234"
		request.Header.Set("X-Forwarded-For", "198.51.100.2")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s: want %d got %d", username, want, response.Code)
		}
		return response
	}
	for i := 0; i < 60; i++ {
		requestLogin("successful", "correct", "192.0.2.1", 200)
	}
	for i := 0; i < 29; i++ {
		requestLogin("reset", "wrong", "192.0.2.1", 401)
	}
	requestLogin("reset", "correct", "192.0.2.1", 200)
	for i := 0; i < 29; i++ {
		requestLogin("reset", "wrong", "192.0.2.1", 401)
	}
	for i := 0; i < 40; i++ {
		requestLogin("reset", "server-error", "192.0.2.1", 500)
	}
	requestLogin("reset", "wrong", "192.0.2.1", 401)
	response := requestLogin("reset", "correct", "192.0.2.1", 429)
	if response.Header().Get("Retry-After") != "300" {
		t.Fatal("incorrect initial cooldown")
	}
	requestLogin("RESET", "wrong", "192.0.2.1", 429)
	requestLogin("another-user", "correct", "192.0.2.1", 200)
	requestLogin("reset", "correct", "192.0.2.2", 200)
	now = now.Add(time.Minute)
	response = requestLogin("reset", "wrong", "192.0.2.1", 429)
	if response.Header().Get("Retry-After") != "240" {
		t.Fatal("blocked retries must not extend cooldown")
	}
	now = now.Add(4 * time.Minute)
	requestLogin("reset", "correct", "192.0.2.1", 200)
}
