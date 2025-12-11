// Package adapter provides implementations for external AI provider integrations.
// It uses the Adapter pattern to abstract provider-specific APIs behind a common interface.
package adapter

import (
	"context"
)

// AIProvider defines the interface for AI provider adapters.
// All provider implementations must satisfy this interface.
type AIProvider interface {
	// ChatCompletion performs a chat completion request.
	// Takes an OpenAI-compatible request and returns an OpenAI-compatible response.
	// This abstraction allows clients to use a consistent API regardless of the underlying provider.
	ChatCompletion(ctx context.Context, req OpenAIRequest) (OpenAIResponse, error)

	// Name returns the provider's identifier string.
	Name() string
}
