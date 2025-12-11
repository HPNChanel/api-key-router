// Package config provides configuration management using the Singleton pattern.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/hpn/hpn-g-router/internal/domain"
	"github.com/spf13/viper"
)

const (
	defaultConfigName = "config"
	defaultConfigType = "yaml"
	envPrefix         = "HPN_ROUTER"

	// EnvAPIKeys is the primary environment variable for API keys (comma-separated).
	// This takes PRIORITY over file configuration for Zero-Trust security.
	EnvAPIKeys = "HPN_API_KEYS"
)

// loadConfig loads the configuration from environment variables and files.
// Priority order (ZERO-TRUST - highest to lowest):
// 1. HPN_API_KEYS env var (comma-separated) - PRIMARY SOURCE
// 2. Environment variables (prefixed with HPN_ROUTER_)
// 3. config.yaml - FALLBACK for local development ONLY
// 4. Default values
func loadConfig(configPath string) (*Configuration, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure Viper
	v.SetConfigName(defaultConfigName)
	v.SetConfigType(defaultConfigType)

	// Add config search paths
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/hpn-g-router")
		v.AddConfigPath("$HOME/.hpn-g-router")
	}

	// Enable environment variable override
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Read configuration file (fallback only)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found is OK - we prefer env vars anyway
			fmt.Fprintf(os.Stderr, "[SECURITY] Config file not found, using environment variables only (recommended)\n")
		} else {
			return nil, &ConfigError{
				Op:  "read",
				Err: fmt.Errorf("failed to read config file: %w", err),
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "[SECURITY] Warning: Using config.yaml - prefer HPN_API_KEYS env var in production\n")
	}

	// Unmarshal configuration
	var cfg Configuration
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, &ConfigError{
			Op:  "unmarshal",
			Err: fmt.Errorf("failed to unmarshal config: %w", err),
		}
	}

	// PRIORITY: Load API keys from HPN_API_KEYS env var first
	envKeysLoaded, err := loadAPIKeysFromPrimaryEnv(&cfg)
	if err != nil {
		return nil, &ConfigError{
			Op:  "load_primary_env_keys",
			Err: err,
		}
	}

	// If primary env var was used, clear any file-based keys for security
	if envKeysLoaded {
		fmt.Fprintf(os.Stderr, "[SECURITY] Using HPN_API_KEYS env var (file config keys ignored)\n")
	} else {
		// Fallback: Load API keys from legacy HPN_ROUTER_API_KEY_* format
		if err := loadAPIKeysFromLegacyEnv(&cfg); err != nil {
			return nil, &ConfigError{
				Op:  "load_legacy_env_keys",
				Err: err,
			}
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults sets default configuration values.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout_seconds", 30)
	v.SetDefault("server.write_timeout_seconds", 30)
	v.SetDefault("server.shutdown_timeout_seconds", 15)

	// Key pool defaults
	v.SetDefault("key_pool.strategy", "round-robin")
	v.SetDefault("key_pool.retry_count", 3)
	v.SetDefault("key_pool.cooldown_seconds", 60)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output_path", "")
}

// loadAPIKeysFromPrimaryEnv loads API keys from the HPN_API_KEYS environment variable.
// This is the PRIMARY and PREFERRED method for production deployments.
// Format: comma-separated list of API keys (e.g., "key1,key2,key3")
// Returns true if keys were loaded from this source.
func loadAPIKeysFromPrimaryEnv(cfg *Configuration) (bool, error) {
	envValue := os.Getenv(EnvAPIKeys)
	if envValue == "" {
		return false, nil
	}

	// Parse comma-separated keys
	keys := strings.Split(envValue, ",")
	if len(keys) == 0 {
		return false, nil
	}

	// Clear existing keys from file config (env takes priority)
	cfg.KeyPool.Keys = make([]domain.APIKey, 0, len(keys))

	for i, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		// Auto-detect provider from key prefix
		provider := detectProviderFromKey(key)

		cfg.KeyPool.Keys = append(cfg.KeyPool.Keys, domain.APIKey{
			Key:      key,
			Name:     fmt.Sprintf("env_key_%d", i),
			Provider: provider,
			Enabled:  true,
			Weight:   1,
		})
	}

	return len(cfg.KeyPool.Keys) > 0, nil
}

// detectProviderFromKey attempts to identify the provider from key format.
func detectProviderFromKey(key string) domain.ProviderType {
	switch {
	case strings.HasPrefix(key, "sk-"):
		return domain.ProviderType("openai")
	case strings.HasPrefix(key, "sk-ant-"):
		return domain.ProviderType("anthropic")
	case strings.HasPrefix(key, "AIza"):
		return domain.ProviderType("google")
	default:
		// Default to google since we're routing to Gemini
		return domain.ProviderType("google")
	}
}

// loadAPIKeysFromLegacyEnv loads API keys from legacy HPN_ROUTER_API_KEY_* format.
// This is kept for backward compatibility but HPN_API_KEYS is preferred.
func loadAPIKeysFromLegacyEnv(cfg *Configuration) error {
	envKeys := os.Environ()
	prefix := envPrefix + "_API_KEY_"

	for _, env := range envKeys {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		keyName := strings.TrimPrefix(parts[0], prefix)
		keyValue := parts[1]

		if keyValue == "" {
			continue
		}

		// Parse provider from key name (e.g., OPENAI_0 -> openai)
		providerParts := strings.Split(keyName, "_")
		if len(providerParts) < 1 {
			continue
		}

		providerName := strings.ToLower(providerParts[0])
		keyExists := false

		// Check if key already exists in config
		for _, existingKey := range cfg.KeyPool.Keys {
			if existingKey.Key == keyValue {
				keyExists = true
				break
			}
		}

		if !keyExists {
			cfg.KeyPool.Keys = append(cfg.KeyPool.Keys, domain.APIKey{
				Key:      keyValue,
				Name:     fmt.Sprintf("env_%s", keyName),
				Provider: domain.ProviderType(providerName),
				Enabled:  true,
				Weight:   1,
			})
		}
	}

	return nil
}
