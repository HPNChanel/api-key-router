// Package security provides data leakage prevention utilities.
package security

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// Redaction placeholder for sensitive data.
const RedactedPlaceholder = "[REDACTED_KEY_XYZ]"

// sensitivePatterns contains regex patterns for common API key formats.
var sensitivePatterns = []*regexp.Regexp{
	// OpenAI keys: sk-... (varies 32-100+ chars)
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
	// Google AI keys: AIza...
	regexp.MustCompile(`AIza[a-zA-Z0-9_-]{30,}`),
	// Anthropic keys: sk-ant-...
	regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{20,}`),
	// Generic Bearer tokens in strings
	regexp.MustCompile(`Bearer\s+[a-zA-Z0-9_-]{20,}`),
	// API keys in query params: key=...
	regexp.MustCompile(`key=[a-zA-Z0-9_-]{20,}`),
	// Generic long alphanumeric strings that look like keys (40+ chars)
	regexp.MustCompile(`[a-zA-Z0-9_-]{40,}`),
}

// Redact scans a string for sensitive patterns and replaces them.
// This is the primary function for sanitizing log output.
func Redact(s string) string {
	result := s
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, RedactedPlaceholder)
	}
	return result
}

// RedactedHandler wraps an slog.Handler and redacts sensitive data from log records.
type RedactedHandler struct {
	inner slog.Handler
}

// NewRedactedHandler creates a new handler that wraps an existing handler
// and redacts sensitive data from all log output.
func NewRedactedHandler(inner slog.Handler) *RedactedHandler {
	return &RedactedHandler{inner: inner}
}

// Enabled reports whether the handler handles records at the given level.
func (h *RedactedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle processes a log record, redacting sensitive data.
func (h *RedactedHandler) Handle(ctx context.Context, r slog.Record) error {
	// Redact the message
	r = slog.Record{
		Time:    r.Time,
		Message: Redact(r.Message),
		Level:   r.Level,
		PC:      r.PC,
	}

	// Redact attributes
	attrs := make([]slog.Attr, 0)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, redactAttr(a))
		return true
	})

	for _, a := range attrs {
		r.AddAttrs(a)
	}

	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes added.
func (h *RedactedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return &RedactedHandler{inner: h.inner.WithAttrs(redacted)}
}

// WithGroup returns a new handler with the given group name.
func (h *RedactedHandler) WithGroup(name string) slog.Handler {
	return &RedactedHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr redacts sensitive data from a single attribute.
func redactAttr(a slog.Attr) slog.Attr {
	// Check for known sensitive keys
	key := strings.ToLower(a.Key)
	if isSensitiveKey(key) {
		return slog.String(a.Key, RedactedPlaceholder)
	}

	// Redact string values
	switch v := a.Value.Any().(type) {
	case string:
		return slog.String(a.Key, Redact(v))
	case []string:
		redacted := make([]string, len(v))
		for i, s := range v {
			redacted[i] = Redact(s)
		}
		return slog.Any(a.Key, redacted)
	}

	return a
}

// isSensitiveKey checks if an attribute key is known to contain sensitive data.
func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"authorization",
		"api_key",
		"apikey",
		"api-key",
		"secret",
		"password",
		"token",
		"bearer",
		"credential",
	}

	for _, k := range sensitiveKeys {
		if strings.Contains(key, k) {
			return true
		}
	}
	return false
}
