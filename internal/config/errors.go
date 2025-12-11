// Package config provides configuration management using the Singleton pattern.
package config

import (
	"fmt"
	"strings"
)

// ConfigError represents a configuration loading error.
type ConfigError struct {
	Op  string // Operation that failed (read, unmarshal, validate)
	Err error  // Underlying error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config %s error: %v", e.Op, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// ValidationError represents configuration validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("configuration validation failed: %s", e.Errors[0])
	}
	return fmt.Sprintf("configuration validation failed with %d errors:\n  - %s",
		len(e.Errors), strings.Join(e.Errors, "\n  - "))
}

// HasError checks if a specific field has a validation error.
func (e *ValidationError) HasError(field string) bool {
	for _, err := range e.Errors {
		if strings.Contains(err, field) {
			return true
		}
	}
	return false
}

// MissingKeyError represents a missing required configuration key error.
type MissingKeyError struct {
	Key string
}

func (e *MissingKeyError) Error() string {
	return fmt.Sprintf("required configuration key '%s' is missing", e.Key)
}

// InvalidValueError represents an invalid configuration value error.
type InvalidValueError struct {
	Key           string
	Value         interface{}
	AllowedValues []string
}

func (e *InvalidValueError) Error() string {
	if len(e.AllowedValues) > 0 {
		return fmt.Sprintf("invalid value '%v' for key '%s', allowed values: %s",
			e.Value, e.Key, strings.Join(e.AllowedValues, ", "))
	}
	return fmt.Sprintf("invalid value '%v' for key '%s'", e.Value, e.Key)
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsConfigError checks if an error is a ConfigError.
func IsConfigError(err error) bool {
	_, ok := err.(*ConfigError)
	return ok
}

// IsMissingKeyError checks if an error is a MissingKeyError.
func IsMissingKeyError(err error) bool {
	_, ok := err.(*MissingKeyError)
	return ok
}
