package security

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // Check if result contains this (since full redaction varies)
		excludes string // Check if result does NOT contain this
	}{
		{
			name:     "OpenAI key",
			input:    "Using key sk-1234567890abcdefghijklmnopqrstuvwxyz",
			contains: RedactedPlaceholder,
			excludes: "sk-1234567890",
		},
		{
			name:     "Google AI key",
			input:    "API key: AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ123456789",
			contains: RedactedPlaceholder,
			excludes: "AIzaSy",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer sk-abcdef1234567890abcdef1234567890",
			contains: RedactedPlaceholder,
			excludes: "sk-abcdef",
		},
		{
			name:     "No sensitive data",
			input:    "Normal log message",
			contains: "Normal log message",
			excludes: RedactedPlaceholder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Redact(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Redact() = %q, should contain %q", result, tt.contains)
			}
			if tt.excludes != "" && strings.Contains(result, tt.excludes) {
				t.Errorf("Redact() = %q, should NOT contain %q", result, tt.excludes)
			}
		})
	}
}

func TestRedactedHandler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	redactedHandler := NewRedactedHandler(baseHandler)
	logger := slog.New(redactedHandler)

	// Log a message with a sensitive key attribute name
	logger.Info("request completed", slog.String("api_key", "sk-testtesttesttesttesttesttest1234"))

	output := buf.String()

	// api_key is a sensitive key name, so it should be redacted
	if strings.Contains(output, "sk-test") {
		t.Errorf("Log output contains raw API key: %s", output)
	}

	// Message should still be there
	if !strings.Contains(output, "request completed") {
		t.Errorf("Log output missing message: %s", output)
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"authorization", true},
		{"api_key", true},
		{"password", true},
		{"token", true},
		{"user_name", false},
		{"status", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			if result != tt.expected {
				t.Errorf("isSensitiveKey(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestRedactedHandlerEnabled(t *testing.T) {
	baseHandler := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	redactedHandler := NewRedactedHandler(baseHandler)

	if redactedHandler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Should not be enabled for Info level when base is Warn")
	}

	if !redactedHandler.Enabled(context.Background(), slog.LevelError) {
		t.Error("Should be enabled for Error level when base is Warn")
	}
}
