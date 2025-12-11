// Package domain contains the core business entities and value objects.
package domain

import (
	"sync"
	"time"
)

// RotationStrategy defines how API keys are selected from the pool.
type RotationStrategy string

const (
	// StrategyRoundRobin cycles through keys sequentially.
	StrategyRoundRobin RotationStrategy = "round-robin"

	// StrategyRandom selects a random key from the pool.
	StrategyRandom RotationStrategy = "random"

	// StrategyWeighted selects keys based on their weight.
	StrategyWeighted RotationStrategy = "weighted"

	// StrategyLeastUsed selects the key with the fewest recent uses.
	StrategyLeastUsed RotationStrategy = "least-used"
)

// APIKey represents a single API key with its metadata.
type APIKey struct {
	// Key is the actual API key string.
	Key string `json:"key" mapstructure:"key"`

	// Name is a human-readable identifier for this key.
	Name string `json:"name" mapstructure:"name"`

	// Provider associates this key with a specific provider.
	Provider ProviderType `json:"provider" mapstructure:"provider"`

	// Weight is used for weighted rotation strategy (higher = more likely to be selected).
	Weight int `json:"weight" mapstructure:"weight"`

	// Enabled indicates whether this key is active.
	Enabled bool `json:"enabled" mapstructure:"enabled"`

	// RateLimitPerMinute overrides the provider's rate limit for this specific key.
	RateLimitPerMinute int `json:"rate_limit_per_minute" mapstructure:"rate_limit_per_minute"`

	// UsageCount tracks how many times this key has been used (runtime only).
	UsageCount int64 `json:"-" mapstructure:"-"`

	// LastUsedAt tracks when this key was last used (runtime only).
	LastUsedAt time.Time `json:"-" mapstructure:"-"`

	// IsExhausted indicates if this key has hit its rate limit (runtime only).
	IsExhausted bool `json:"-" mapstructure:"-"`

	// ExhaustedUntil indicates when the key will be available again (runtime only).
	ExhaustedUntil time.Time `json:"-" mapstructure:"-"`
}

// IsValid checks if the API key has all required fields.
func (k *APIKey) IsValid() bool {
	return k.Key != "" && k.Provider != ""
}

// IsAvailable checks if the key is enabled and not exhausted.
func (k *APIKey) IsAvailable() bool {
	if !k.Enabled {
		return false
	}
	if k.IsExhausted && time.Now().Before(k.ExhaustedUntil) {
		return false
	}
	// Reset exhausted status if cooldown has passed
	if k.IsExhausted && time.Now().After(k.ExhaustedUntil) {
		k.IsExhausted = false
	}
	return true
}

// KeyPool manages a collection of API keys with rotation logic.
type KeyPool struct {
	// Keys is the list of API keys in this pool.
	Keys []*APIKey `json:"keys" mapstructure:"keys"`

	// Strategy defines how keys are rotated.
	Strategy RotationStrategy `json:"strategy" mapstructure:"strategy"`

	// currentIndex is used for round-robin rotation (runtime only).
	currentIndex int

	// mu protects concurrent access to the pool.
	mu sync.RWMutex
}

// NewKeyPool creates a new KeyPool with the specified strategy.
func NewKeyPool(strategy RotationStrategy) *KeyPool {
	return &KeyPool{
		Keys:         make([]*APIKey, 0),
		Strategy:     strategy,
		currentIndex: 0,
	}
}

// AddKey adds an API key to the pool.
func (p *KeyPool) AddKey(key *APIKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Keys = append(p.Keys, key)
}

// GetAvailableKeys returns all keys that are currently available.
func (p *KeyPool) GetAvailableKeys() []*APIKey {
	p.mu.RLock()
	defer p.mu.RUnlock()

	available := make([]*APIKey, 0)
	for _, key := range p.Keys {
		if key.IsAvailable() {
			available = append(available, key)
		}
	}
	return available
}

// Size returns the total number of keys in the pool.
func (p *KeyPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.Keys)
}

// AvailableSize returns the number of currently available keys.
func (p *KeyPool) AvailableSize() int {
	return len(p.GetAvailableKeys())
}

// GetKeysByProvider returns all keys for a specific provider.
func (p *KeyPool) GetKeysByProvider(provider ProviderType) []*APIKey {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keys := make([]*APIKey, 0)
	for _, key := range p.Keys {
		if key.Provider == provider && key.IsAvailable() {
			keys = append(keys, key)
		}
	}
	return keys
}
