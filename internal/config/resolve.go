package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/shell"
)

type VariableResolver interface {
	ResolveValue(value string) (string, error)
}

type Shell interface {
	Exec(ctx context.Context, command string) (stdout, stderr string, err error)
}

type shellVariableResolver struct {
	shell Shell
	env   env.Env
}

func NewShellVariableResolver(env env.Env) VariableResolver {
	return &shellVariableResolver{
		env: env,
		shell: shell.NewShell(
			&shell.Options{
				Env: env.Env(),
			},
		),
	}
}

// ResolveValue is a method for resolving values, such as environment variables.
// it will resolve shell-like variable substitution anywhere in the string, including:
// - $(command) for command substitution
// - $VAR or ${VAR} for environment variables
func (r *shellVariableResolver) ResolveValue(value string) (string, error) {
	// Special case: lone $ is an error (backward compatibility)
	if value == "$" {
		return "", fmt.Errorf("invalid value format: %s", value)
	}

	// If no $ found, return as-is
	if !strings.Contains(value, "$") {
		return value, nil
	}

	result := value

	// Handle command substitution: $(command)
	for {
		start := strings.Index(result, "$(")
		if start == -1 {
			break
		}

		// Find matching closing parenthesis
		depth := 0
		end := -1
		for i := start + 2; i < len(result); i++ {
			if result[i] == '(' {
				depth++
			} else if result[i] == ')' {
				if depth == 0 {
					end = i
					break
				}
				depth--
			}
		}

		if end == -1 {
			return "", fmt.Errorf("unmatched $( in value: %s", value)
		}

		command := result[start+2 : end]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		stdout, _, err := r.shell.Exec(ctx, command)
		cancel()
		if err != nil {
			return "", fmt.Errorf("command execution failed for '%s': %w", command, err)
		}

		// Replace the $(command) with the output
		replacement := strings.TrimSpace(stdout)
		result = result[:start] + replacement + result[end+1:]
	}

	// Handle environment variables: $VAR and ${VAR}
	searchStart := 0
	for {
		start := strings.Index(result[searchStart:], "$")
		if start == -1 {
			break
		}
		start += searchStart // Adjust for the offset

		// Skip if this is part of $( which we already handled
		if start+1 < len(result) && result[start+1] == '(' {
			// Skip past this $(...)
			searchStart = start + 1
			continue
		}
		var varName string
		var end int

		if start+1 < len(result) && result[start+1] == '{' {
			// Handle ${VAR} format
			closeIdx := strings.Index(result[start+2:], "}")
			if closeIdx == -1 {
				return "", fmt.Errorf("unmatched ${ in value: %s", value)
			}
			varName = result[start+2 : start+2+closeIdx]
			end = start + 2 + closeIdx + 1
		} else {
			// Handle $VAR format - variable names must start with letter or underscore
			if start+1 >= len(result) {
				return "", fmt.Errorf("incomplete variable reference at end of string: %s", value)
			}

			if result[start+1] != '_' &&
				(result[start+1] < 'a' || result[start+1] > 'z') &&
				(result[start+1] < 'A' || result[start+1] > 'Z') {
				return "", fmt.Errorf("invalid variable name starting with '%c' in: %s", result[start+1], value)
			}

			end = start + 1
			for end < len(result) && (result[end] == '_' ||
				(result[end] >= 'a' && result[end] <= 'z') ||
				(result[end] >= 'A' && result[end] <= 'Z') ||
				(result[end] >= '0' && result[end] <= '9')) {
				end++
			}
			varName = result[start+1 : end]
		}

		envValue := r.env.Get(varName)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %q not set", varName)
		}

		result = result[:start] + envValue + result[end:]
		searchStart = start + len(envValue) // Continue searching after the replacement
	}

	return result, nil
}

type environmentVariableResolver struct {
	env env.Env
}

func NewEnvironmentVariableResolver(env env.Env) VariableResolver {
	return &environmentVariableResolver{
		env: env,
	}
}

// ResolveValue resolves environment variables from the provided env.Env.
func (r *environmentVariableResolver) ResolveValue(value string) (string, error) {
	if !strings.HasPrefix(value, "$") {
		return value, nil
	}

	varName := strings.TrimPrefix(value, "$")
	resolvedValue := r.env.Get(varName)
	if resolvedValue == "" {
		return "", fmt.Errorf("environment variable %q not set", varName)
	}
	return resolvedValue, nil
}
