// Package handler provides HTTP handlers for the API router.
package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hpn/hpn-g-router/internal/adapter"
	"github.com/hpn/hpn-g-router/internal/domain"
)

const (
	// DefaultMaxRetries is the default maximum number of retry attempts.
	DefaultMaxRetries = 3
)

// ProxyHandler handles API proxy requests with retry/failover logic.
// It implements "The Immortal Mode" - automatic key rotation on failures.
type ProxyHandler struct {
	keyManager *domain.KeyManager
	adapter    adapter.AIProvider
	logger     *slog.Logger
	maxRetries int
}

// ProxyHandlerOption is a functional option for configuring ProxyHandler.
type ProxyHandlerOption func(*ProxyHandler)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(max int) ProxyHandlerOption {
	return func(h *ProxyHandler) {
		if max > 0 {
			h.maxRetries = max
		}
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) ProxyHandlerOption {
	return func(h *ProxyHandler) {
		h.logger = logger
	}
}

// NewProxyHandler creates a new ProxyHandler.
func NewProxyHandler(
	keyManager *domain.KeyManager,
	aiAdapter adapter.AIProvider,
	opts ...ProxyHandlerOption,
) *ProxyHandler {
	h := &ProxyHandler{
		keyManager: keyManager,
		adapter:    aiAdapter,
		logger:     slog.Default(),
		maxRetries: DefaultMaxRetries,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// HandleChatCompletion handles POST /v1/chat/completions
// This is the main proxy endpoint that implements retry/failover logic.
func (h *ProxyHandler) HandleChatCompletion(c *gin.Context) {
	// Parse OpenAI-compatible request
	var req adapter.OpenAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "Invalid request body: "+err.Error())
		return
	}

	// Validate request
	if len(req.Messages) == 0 {
		h.sendOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "messages array is required")
		return
	}

	// Execute with retry logic
	resp, attempts, err := h.executeWithRetry(c, req)
	if err != nil {
		h.logger.Error("all retries exhausted",
			slog.String("error", err.Error()),
			slog.Int("attempts", attempts),
		)
		h.sendOpenAIError(c, http.StatusServiceUnavailable, "server_error", "Service temporarily unavailable. Please try again later.")
		return
	}

	// Store metadata for logging middleware
	c.Set("attempts", attempts)

	// Return OpenAI-compatible response
	c.JSON(http.StatusOK, resp)
}

// executeWithRetry attempts the request with automatic key rotation on failures.
// Returns the response, number of attempts, and any error.
func (h *ProxyHandler) executeWithRetry(c *gin.Context, req adapter.OpenAIRequest) (adapter.OpenAIResponse, int, error) {
	var lastErr error
	var usedKeys []string

	for attempt := 1; attempt <= h.maxRetries; attempt++ {
		// Get next key from KeyManager
		key, err := h.keyManager.GetNextKey()
		if err != nil {
			h.logger.Warn("no keys available",
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)
			return adapter.OpenAIResponse{}, attempt, err
		}

		usedKeys = append(usedKeys, key)
		c.Set("key_used", key)

		h.logger.Debug("attempting request",
			slog.Int("attempt", attempt),
			slog.String("key", maskKey(key)),
			slog.String("model", req.Model),
		)

		// Create a new adapter with the current key
		geminiAdapter := adapter.NewGeminiAdapter(key)

		// Execute request
		resp, err := geminiAdapter.ChatCompletion(c.Request.Context(), req)
		if err == nil {
			// Success!
			h.logger.Info("request successful",
				slog.Int("attempt", attempt),
				slog.String("model", resp.Model),
			)
			return resp, attempt, nil
		}

		// Check if error is retryable
		if h.isRetryableError(err) {
			h.logger.Warn("retryable error, rotating key",
				slog.Int("attempt", attempt),
				slog.String("key", maskKey(key)),
				slog.String("error", err.Error()),
			)

			// Mark key as dead (circuit breaker)
			h.keyManager.MarkAsDead(key)
			lastErr = err
			continue
		}

		// Non-retryable error (4xx client errors)
		h.logger.Error("non-retryable error",
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
		)
		return adapter.OpenAIResponse{}, attempt, err
	}

	h.logger.Error("max retries exhausted",
		slog.Int("max_retries", h.maxRetries),
		slog.Any("used_keys", h.maskKeys(usedKeys)),
	)

	return adapter.OpenAIResponse{}, h.maxRetries, lastErr
}

// isRetryableError determines if an error should trigger a retry.
// Retryable: 429 (Rate Limited), 5xx (Server Errors)
// Non-retryable: 4xx (Client Errors except 429)
func (h *ProxyHandler) isRetryableError(err error) bool {
	errStr := err.Error()

	// Check for rate limiting (429)
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return true
	}

	// Check for server errors (5xx)
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") {
		return true
	}

	// Check for quota exhausted
	if strings.Contains(errStr, "quota") || strings.Contains(errStr, "exhausted") {
		return true
	}

	// Default: not retryable (likely client error)
	return false
}

// sendOpenAIError sends an error response in OpenAI-compatible format.
// This ensures clients never see internal Google errors.
func (h *ProxyHandler) sendOpenAIError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	})
}

// maskKeys returns masked versions of multiple keys.
func (h *ProxyHandler) maskKeys(keys []string) []string {
	masked := make([]string, len(keys))
	for i, k := range keys {
		masked[i] = maskKey(k)
	}
	return masked
}

// HandleModels handles GET /v1/models
// Returns a list of available models (OpenAI-compatible).
func (h *ProxyHandler) HandleModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data": []gin.H{
			{
				"id":       "gpt-4",
				"object":   "model",
				"created":  1687882411,
				"owned_by": "openai",
			},
			{
				"id":       "gpt-4-turbo",
				"object":   "model",
				"created":  1687882411,
				"owned_by": "openai",
			},
			{
				"id":       "gpt-3.5-turbo",
				"object":   "model",
				"created":  1687882411,
				"owned_by": "openai",
			},
			{
				"id":       "gemini-1.5-pro",
				"object":   "model",
				"created":  1687882411,
				"owned_by": "google",
			},
			{
				"id":       "gemini-1.5-flash",
				"object":   "model",
				"created":  1687882411,
				"owned_by": "google",
			},
		},
	})
}

// HandleHealth handles GET /health
// Returns server health status.
func (h *ProxyHandler) HandleHealth(c *gin.Context) {
	activeKeys := h.keyManager.ActiveKeyCount()
	deadKeys := h.keyManager.DeadKeyCount()

	status := "healthy"
	if activeKeys == 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      status,
		"active_keys": activeKeys,
		"dead_keys":   deadKeys,
		"total_keys":  h.keyManager.TotalKeyCount(),
	})
}
