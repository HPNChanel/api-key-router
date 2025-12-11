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
)

// loadConfig loads the configuration from files and environment variables.
// Priority order (highest to lowest):
// 1. Environment variables (prefixed with HPN_ROUTER_)
// 2. config.yaml in the current directory
// 3. config.yaml in the configs/ directory
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

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; use defaults and environment variables only
			// This is acceptable if environment variables are set
			fmt.Fprintf(os.Stderr, "Warning: Config file not found, using defaults and environment variables\n")
		} else {
			// Config file was found but another error was produced
			return nil, &ConfigError{
				Op:  "read",
				Err: fmt.Errorf("failed to read config file: %w", err),
			}
		}
	}

	// Unmarshal configuration
	var cfg Configuration
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, &ConfigError{
			Op:  "unmarshal",
			Err: fmt.Errorf("failed to unmarshal config: %w", err),
		}
	}

	// Load API keys from environment variable if not set in config file
	if err := loadAPIKeysFromEnv(&cfg); err != nil {
		return nil, &ConfigError{
			Op:  "load_env_keys",
			Err: err,
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

// loadAPIKeysFromEnv loads API keys from environment variables.
// Environment variable format: HPN_ROUTER_API_KEY_<PROVIDER>_<INDEX>=<key>
// Example: HPN_ROUTER_API_KEY_OPENAI_0=sk-xxx
func loadAPIKeysFromEnv(cfg *Configuration) error {
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
