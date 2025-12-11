// Package config provides configuration management using the Singleton pattern.
// It loads configuration from environment variables and config.yaml using Viper.
package config

import (
	"fmt"
	"sync"

	"github.com/hpn/hpn-g-router/internal/domain"
)

// Configuration holds all application configuration values.
type Configuration struct {
	// Server configuration
	Server ServerConfig `json:"server" mapstructure:"server"`

	// API Keys pool configuration
	KeyPool KeyPoolConfig `json:"key_pool" mapstructure:"key_pool"`

	// Providers configuration
	Providers []domain.Provider `json:"providers" mapstructure:"providers"`

	// Logging configuration
	Logging LoggingConfig `json:"logging" mapstructure:"logging"`
}

// ServerConfig holds server-specific configuration.
type ServerConfig struct {
	// Host is the server bind address.
	Host string `json:"host" mapstructure:"host"`

	// Port is the server port number.
	Port int `json:"port" mapstructure:"port"`

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeoutSeconds int `json:"read_timeout_seconds" mapstructure:"read_timeout_seconds"`

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeoutSeconds int `json:"write_timeout_seconds" mapstructure:"write_timeout_seconds"`

	// ShutdownTimeout is the maximum duration to wait for active connections to finish.
	ShutdownTimeoutSeconds int `json:"shutdown_timeout_seconds" mapstructure:"shutdown_timeout_seconds"`
}

// KeyPoolConfig holds API key pool configuration.
type KeyPoolConfig struct {
	// Strategy defines how keys are rotated (round-robin, random, weighted, least-used).
	Strategy domain.RotationStrategy `json:"strategy" mapstructure:"strategy"`

	// Keys is the list of API keys.
	Keys []domain.APIKey `json:"keys" mapstructure:"keys"`

	// RetryCount is the number of times to retry with a different key on failure.
	RetryCount int `json:"retry_count" mapstructure:"retry_count"`

	// CooldownSeconds is the duration to wait before retrying an exhausted key.
	CooldownSeconds int `json:"cooldown_seconds" mapstructure:"cooldown_seconds"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string `json:"level" mapstructure:"level"`

	// Format is the log format (json, text).
	Format string `json:"format" mapstructure:"format"`

	// OutputPath is the file path for log output (empty for stdout).
	OutputPath string `json:"output_path" mapstructure:"output_path"`
}

// configInstance holds the singleton configuration instance.
var (
	configInstance *Configuration
	configOnce     sync.Once
	configErr      error
)

// GetConfig returns the singleton Configuration instance.
// It initializes the configuration on first call using the default config path.
// Returns an error if configuration loading fails.
func GetConfig() (*Configuration, error) {
	configOnce.Do(func() {
		configInstance, configErr = loadConfig("")
	})
	return configInstance, configErr
}

// GetConfigWithPath returns the singleton Configuration instance with a custom config path.
// This should be used when you need to specify a non-default configuration file path.
// Returns an error if configuration loading fails.
func GetConfigWithPath(configPath string) (*Configuration, error) {
	configOnce.Do(func() {
		configInstance, configErr = loadConfig(configPath)
	})
	return configInstance, configErr
}

// MustGetConfig returns the singleton Configuration instance.
// It panics if the configuration cannot be loaded.
// Use this only when configuration is absolutely required and the application
// cannot proceed without it.
func MustGetConfig() *Configuration {
	cfg, err := GetConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}

// ResetConfig resets the singleton instance.
// This is primarily used for testing purposes.
func ResetConfig() {
	configOnce = sync.Once{}
	configInstance = nil
	configErr = nil
}

// Validate validates the configuration and returns an error if required fields are missing.
func (c *Configuration) Validate() error {
	var validationErrors []string

	// Validate server configuration
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		validationErrors = append(validationErrors, "server.port must be between 1 and 65535")
	}

	// Validate key pool configuration
	if c.KeyPool.Strategy == "" {
		validationErrors = append(validationErrors, "key_pool.strategy is required")
	}

	if !isValidStrategy(c.KeyPool.Strategy) {
		validationErrors = append(validationErrors, fmt.Sprintf(
			"key_pool.strategy '%s' is invalid, must be one of: round-robin, random, weighted, least-used",
			c.KeyPool.Strategy,
		))
	}

	if len(c.KeyPool.Keys) == 0 {
		validationErrors = append(validationErrors, "key_pool.keys cannot be empty, at least one API key is required")
	}

	// Validate each API key
	for i, key := range c.KeyPool.Keys {
		if key.Key == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("key_pool.keys[%d].key is required", i))
		}
		if key.Provider == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("key_pool.keys[%d].provider is required", i))
		}
	}

	// Validate providers if specified
	for i, provider := range c.Providers {
		if provider.Name == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("providers[%d].name is required", i))
		}
		if provider.Type == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("providers[%d].type is required", i))
		}
		if provider.BaseURL == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("providers[%d].base_url is required", i))
		}
	}

	// Validate logging configuration
	if c.Logging.Level != "" && !isValidLogLevel(c.Logging.Level) {
		validationErrors = append(validationErrors, fmt.Sprintf(
			"logging.level '%s' is invalid, must be one of: debug, info, warn, error",
			c.Logging.Level,
		))
	}

	if len(validationErrors) > 0 {
		return &ValidationError{Errors: validationErrors}
	}

	return nil
}

// isValidStrategy checks if the rotation strategy is valid.
func isValidStrategy(strategy domain.RotationStrategy) bool {
	switch strategy {
	case domain.StrategyRoundRobin, domain.StrategyRandom, domain.StrategyWeighted, domain.StrategyLeastUsed:
		return true
	default:
		return false
	}
}

// isValidLogLevel checks if the log level is valid.
func isValidLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// GetActiveKeys returns all enabled API keys.
func (c *Configuration) GetActiveKeys() []domain.APIKey {
	activeKeys := make([]domain.APIKey, 0)
	for _, key := range c.KeyPool.Keys {
		if key.Enabled {
			activeKeys = append(activeKeys, key)
		}
	}
	return activeKeys
}

// GetKeysByProvider returns all API keys for a specific provider.
func (c *Configuration) GetKeysByProvider(provider domain.ProviderType) []domain.APIKey {
	keys := make([]domain.APIKey, 0)
	for _, key := range c.KeyPool.Keys {
		if key.Provider == provider && key.Enabled {
			keys = append(keys, key)
		}
	}
	return keys
}

// GetProvider returns a provider by its type.
func (c *Configuration) GetProvider(providerType domain.ProviderType) (*domain.Provider, bool) {
	for _, provider := range c.Providers {
		if provider.Type == providerType {
			return &provider, true
		}
	}
	return nil, false
}
