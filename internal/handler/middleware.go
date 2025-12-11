// Package handler provides HTTP handlers for the API router.
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns a middleware that enables permissive CORS.
// This allows web applications to call the API directly.
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

// LoggingMiddleware returns a middleware that logs request details in JSON format.
// It tracks the key used for each request (essential for debugging).
func LoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get key from context (set by ProxyHandler)
		keyUsed, _ := c.Get("key_used")
		keyName, _ := keyUsed.(string)

		// Get attempt count
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
	}
}

// RecoveryMiddleware returns a middleware that recovers from panics.
// It logs the error and returns a 500 response in OpenAI-compatible format.
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
						"message": "Internal server error",
						"type":    "server_error",
						"code":    "internal_error",
					},
				})
			}
		}()

		c.Next()
	}
}

// StripAuthHeadersMiddleware removes original Authorization headers.
// The ProxyHandler will inject the rotated API key.
func StripAuthHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Store original auth header for logging (masked)
		if auth := c.GetHeader("Authorization"); auth != "" {
			c.Set("original_auth", "***STRIPPED***")
		}

		c.Next()
	}
}

// maskKey returns a masked version of the API key for logging.
// Shows first 8 and last 4 characters.
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}
