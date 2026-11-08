package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func LoginRateLimit() gin.HandlerFunc {
	return loginRateLimit(time.Now)
}

// Only failed credentials count. Successful login resets this account/IP pair.
func loginRateLimit(clock func() time.Time) gin.HandlerFunc {
	type bucket struct {
		count int
		until time.Time
	}
	entries := map[string]bucket{}
	var mu sync.Mutex
	return func(c *gin.Context) {
		var identity struct {
			Username string `json:"username"`
		}
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
		if err != nil || json.Unmarshal(body, &identity) != nil {
			c.AbortWithStatusJSON(400, gin.H{"message": "Dữ liệu đăng nhập không hợp lệ"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		username := strings.ToLower(strings.TrimSpace(identity.Username))
		if username == "" {
			c.Next()
			return
		}
		key := c.ClientIP() + "\x00" + username
		now := clock()
		mu.Lock()
		for entryKey, value := range entries {
			if !now.Before(value.until) {
				delete(entries, entryKey)
			}
		}
		value := entries[key]
		if value.count >= 30 {
			mu.Unlock()
			seconds := int(math.Ceil(value.until.Sub(now).Seconds()))
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(429, gin.H{"message": "Tài khoản này đã đăng nhập sai quá nhiều lần từ thiết bị/mạng hiện tại. Vui lòng thử lại sau " + strconv.Itoa(seconds) + " giây."})
			return
		}
		if value.count == 0 && len(entries) >= 10000 {
			mu.Unlock()
			c.AbortWithStatusJSON(503, gin.H{"message": "Hệ thống đăng nhập đang bận. Vui lòng thử lại sau."})
			return
		}
		mu.Unlock()
		c.Next()
		mu.Lock()
		defer mu.Unlock()
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			delete(entries, key)
			return
		}
		if c.Writer.Status() != http.StatusUnauthorized {
			return
		}
		value = entries[key]
		if !clock().Before(value.until) {
			value.count = 0
		}
		if value.count == 0 && len(entries) >= 10000 {
			return
		}
		value.count++
		value.until = clock().Add(5 * time.Minute)
		entries[key] = value
	}
}
