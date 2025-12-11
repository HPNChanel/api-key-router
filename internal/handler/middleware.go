package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hpn/hpn-g-router/internal/ui"
)

// CORSMiddleware enables permissive CORS for web clients.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// LoggingMiddleware logs request details and cost savings.
func LoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		keyUsed, _ := c.Get("key_used")
		keyName, _ := keyUsed.(string)
		attempts, _ := c.Get("attempts")
		attemptCount, _ := attempts.(int)

		logger.Info("request completed",
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", latency),
			slog.String("client_ip", c.ClientIP()),
			slog.String("key_used", maskKey(keyName)),
			slog.Int("attempts", attemptCount),
			slog.String("user_agent", c.Request.UserAgent()),
		)

		ui.PrintRequest(c.Request.Method, path, c.Writer.Status(), latency, keyName)

		if c.Writer.Status() == http.StatusOK {
			if m, ok := c.Get("cost_metrics"); ok {
				if cm, ok := m.(CostMetrics); ok {
					ui.PrintChaChing(FormatMoneySaved(cm.MoneySaved), FormatTotalSaved(cm.TotalSaved))
				}
			}
		}
	}
}

// RecoveryMiddleware recovers from panics and returns OpenAI-compatible errors.
func RecoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered",
					slog.Any("error", err),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": "internal server error",
						"type":    "server_error",
						"code":    "internal_error",
					},
				})
			}
		}()
		c.Next()
	}
}

// StripAuthHeadersMiddleware removes client auth headers; we inject our own keys.
// SECURITY: This prevents clients from injecting fake Authorization headers.
func StripAuthHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Strip Authorization header - we use our own keys
		if auth := c.GetHeader("Authorization"); auth != "" {
			c.Set("original_auth", "***STRIPPED***")
			c.Request.Header.Del("Authorization") // CRITICAL: Actually remove the header
		}

		// Also strip other potentially dangerous headers
		c.Request.Header.Del("X-Api-Key")
		c.Request.Header.Del("Api-Key")

		c.Next()
	}
}


func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}
