// Package domain contains the core business entities and value objects.
// These structs are framework-agnostic and represent the heart of the application.
package domain

// ProviderType represents the type of API provider (e.g., OpenAI, Anthropic, Google).
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderGoogle    ProviderType = "google"
	ProviderAzure     ProviderType = "azure"
)

// Provider represents an API provider with its configuration.
type Provider struct {
	// Name is the human-readable name of the provider.
	Name string `json:"name" mapstructure:"name"`

	// Type identifies the provider type for routing logic.
	Type ProviderType `json:"type" mapstructure:"type"`

	// BaseURL is the base endpoint for the provider's API.
	BaseURL string `json:"base_url" mapstructure:"base_url"`

	// Enabled indicates whether this provider is active.
	Enabled bool `json:"enabled" mapstructure:"enabled"`

	// RateLimitPerMinute is the maximum requests per minute for this provider.
	RateLimitPerMinute int `json:"rate_limit_per_minute" mapstructure:"rate_limit_per_minute"`
}

// IsValid checks if the provider has all required fields.
func (p *Provider) IsValid() bool {
	return p.Name != "" && p.Type != "" && p.BaseURL != ""
}
