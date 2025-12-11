package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hpn/hpn-g-router/internal/adapter"
	"github.com/hpn/hpn-g-router/internal/domain"
	"github.com/hpn/hpn-g-router/internal/ui"
)

const DefaultMaxRetries = 3

// ProxyHandler proxies OpenAI-compatible requests with automatic key rotation.
type ProxyHandler struct {
	km         *domain.KeyManager
	adapter    adapter.AIProvider
	logger     *slog.Logger
	maxRetries int
}

// ProxyHandlerOption configures a ProxyHandler.
type ProxyHandlerOption func(*ProxyHandler)

// WithMaxRetries sets retry count.
func WithMaxRetries(n int) ProxyHandlerOption {
	return func(h *ProxyHandler) {
		if n > 0 {
			h.maxRetries = n
		}
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) ProxyHandlerOption {
	return func(h *ProxyHandler) { h.logger = l }
}

// NewProxyHandler creates a configured ProxyHandler.
func NewProxyHandler(km *domain.KeyManager, ai adapter.AIProvider, opts ...ProxyHandlerOption) *ProxyHandler {
	h := &ProxyHandler{
		km:         km,
		adapter:    ai,
		logger:     slog.Default(),
		maxRetries: DefaultMaxRetries,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// HandleChatCompletion proxies /v1/chat/completions with retry logic.
func (h *ProxyHandler) HandleChatCompletion(c *gin.Context) {
	var req adapter.OpenAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request_error", "invalid request body: "+err.Error())
		return
	}

	if len(req.Messages) == 0 {
		h.sendError(c, http.StatusBadRequest, "invalid_request_error", "messages array is required")
		return
	}

	var input strings.Builder
	for _, m := range req.Messages {
		input.WriteString(m.Content)
		input.WriteString(" ")
	}

	resp, attempts, err := h.executeWithRetry(c, req)
	if err != nil {
		h.logger.Error("retries exhausted",
			slog.String("error", err.Error()),
			slog.Int("attempts", attempts),
		)
		h.sendError(c, http.StatusServiceUnavailable, "server_error", "service temporarily unavailable")
		return
	}

	c.Set("attempts", attempts)

	var output string
	if len(resp.Choices) > 0 {
		output = resp.Choices[0].Message.Content
	}

	c.Set("cost_metrics", CalculateRequestCost(input.String(), output))
	c.JSON(http.StatusOK, resp)
}

func (h *ProxyHandler) executeWithRetry(c *gin.Context, req adapter.OpenAIRequest) (adapter.OpenAIResponse, int, error) {
	var lastErr error
	var used []string

	for attempt := 1; attempt <= h.maxRetries; attempt++ {
		key, err := h.km.GetNextKey()
		if err != nil {
			h.logger.Warn("no keys available", slog.Int("attempt", attempt), slog.String("error", err.Error()))
			return adapter.OpenAIResponse{}, attempt, err
		}

		used = append(used, key)
		c.Set("key_used", key)

		h.logger.Debug("trying request",
			slog.Int("attempt", attempt),
			slog.String("key", maskKey(key)),
			slog.String("model", req.Model),
		)

		gemini := adapter.NewGeminiAdapter(key)
		resp, err := gemini.ChatCompletion(c.Request.Context(), req)
		if err == nil {
			h.logger.Info("request ok", slog.Int("attempt", attempt), slog.String("model", resp.Model))
			return resp, attempt, nil
		}

		if h.isRetryable(err) {
			h.logger.Warn("rotating key",
				slog.Int("attempt", attempt),
				slog.String("key", maskKey(key)),
				slog.String("error", err.Error()),
			)
			ui.PrintDeadKey(key, err.Error())
			h.km.MarkAsDead(key)
			lastErr = err
			continue
		}

		h.logger.Error("non-retryable error",
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
		)
		return adapter.OpenAIResponse{}, attempt, err
	}

	h.logger.Error("max retries reached",
		slog.Int("max", h.maxRetries),
		slog.Any("used_keys", h.maskAll(used)),
	)
	return adapter.OpenAIResponse{}, h.maxRetries, lastErr
}

func (h *ProxyHandler) isRetryable(err error) bool {
	s := err.Error()

	// rate limiting
	if strings.Contains(s, "429") || strings.Contains(s, "rate limit") {
		return true
	}

	// server errors
	if strings.Contains(s, "500") || strings.Contains(s, "502") ||
		strings.Contains(s, "503") || strings.Contains(s, "504") {
		return true
	}

	// quota exhausted
	if strings.Contains(s, "quota") || strings.Contains(s, "exhausted") {
		return true
	}

	return false
}

func (h *ProxyHandler) sendError(c *gin.Context, status int, errType, msg string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    errType,
			"param":   nil,
			"code":    nil,
		},
	})
}

func (h *ProxyHandler) maskAll(keys []string) []string {
	res := make([]string, len(keys))
	for i, k := range keys {
		res[i] = maskKey(k)
	}
	return res
}

// HandleModels returns available models (OpenAI format).
func (h *ProxyHandler) HandleModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data": []gin.H{
			{"id": "gpt-4", "object": "model", "created": 1687882411, "owned_by": "openai"},
			{"id": "gpt-4-turbo", "object": "model", "created": 1687882411, "owned_by": "openai"},
			{"id": "gpt-3.5-turbo", "object": "model", "created": 1687882411, "owned_by": "openai"},
			{"id": "gemini-1.5-pro", "object": "model", "created": 1687882411, "owned_by": "google"},
			{"id": "gemini-1.5-flash", "object": "model", "created": 1687882411, "owned_by": "google"},
		},
	})
}

// HandleHealth reports server health status.
func (h *ProxyHandler) HandleHealth(c *gin.Context) {
	active := h.km.ActiveKeyCount()
	dead := h.km.DeadKeyCount()

	status := "healthy"
	if active == 0 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      status,
		"active_keys": active,
		"dead_keys":   dead,
		"total_keys":  h.km.TotalKeyCount(),
	})
}
