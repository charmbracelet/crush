package config

import (
	"fmt"
	"os"
	"log/slog"
)

// EnvironmentConfig holds environment variable configuration for Crush.
type EnvironmentConfig struct {
	// Env is a map of environment variables to set when running Crush.
	Env map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set for the application,example={\"MY_VAR\":\"value\",\"ANOTHER_VAR\":\"another_value\"}"`
}

// ValidateEnv validates that all environment variable names are valid according to POSIX standards.
func (ec *EnvironmentConfig) ValidateEnv() error {
	for key := range ec.Env {
		if err := validateEnvVarName(key); err != nil {
			return fmt.Errorf("invalid environment variable name %q: %w", key, err)
		}
	}
	return nil
}

// SetEnv sets all environment variables in the config.
func (ec *EnvironmentConfig) SetEnv() error {
	for key, value := range ec.Env {
		if err := os.Setenv(key, value); err != nil {
			slog.Warn("failed to set environment variable", "key", key, "error", err)
			return fmt.Errorf("failed to set environment variable %q: %w", key, err)
		}
	}
	return nil
}

// validateEnvVarName validates that an environment variable name follows POSIX standards.
func validateEnvVarName(name string) error {
	if name == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}

	// Check for invalid characters (POSIX compliant)
	for i, r := range name {
		if !isAlphaNumeric(r) && r != '_' {
			if i == 0 || !isDigit(r) {
				return fmt.Errorf("invalid character %q in environment variable name", r)
			}
		}
	}

	// Check if it starts with a digit
	if isDigit(rune(name[0])) {
		return fmt.Errorf("environment variable name cannot start with a digit")
	}

	return nil
}

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}