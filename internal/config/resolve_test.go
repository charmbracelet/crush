package config

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/assert"
)

// mockShell implements the Shell interface for testing
type mockShell struct {
	execFunc func(ctx context.Context, command string) (stdout, stderr string, err error)
}

func (m *mockShell) Exec(ctx context.Context, command string) (stdout, stderr string, err error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, command)
	}
	return "", "", nil
}

func TestShellVariableResolver_ResolveValue(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		envVars     map[string]string
		shellFunc   func(ctx context.Context, command string) (stdout, stderr string, err error)
		expected    string
		expectError bool
	}{
		{
			name:     "non-variable string returns as-is",
			value:    "plain-string",
			expected: "plain-string",
		},
		{
			name:     "environment variable resolution",
			value:    "$HOME",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "/home/user",
		},
		{
			name:        "missing environment variable returns error",
			value:       "$MISSING_VAR",
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:  "shell command execution",
			value: "$(echo hello)",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				if command == "echo hello" {
					return "hello\n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "hello",
		},
		{
			name:  "shell command with whitespace trimming",
			value: "$(echo '  spaced  ')",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				if command == "echo '  spaced  '" {
					return "  spaced  \n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "spaced",
		},
		{
			name:  "shell command execution error",
			value: "$(false)",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				return "", "", errors.New("command failed")
			},
			expectError: true,
		},
		{
			name:        "invalid format returns error",
			value:       "$",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := env.NewFromMap(tt.envVars)
			resolver := &shellVariableResolver{
				shell: &mockShell{execFunc: tt.shellFunc},
				env:   testEnv,
			}

			result, err := resolver.ResolveValue(tt.value)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestEnvironmentVariableResolver_ResolveValue(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		envVars     map[string]string
		expected    string
		expectError bool
	}{
		{
			name:     "non-variable string returns as-is",
			value:    "plain-string",
			expected: "plain-string",
		},
		{
			name:     "environment variable resolution",
			value:    "$HOME",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "/home/user",
		},
		{
			name:     "environment variable with complex value",
			value:    "$PATH",
			envVars:  map[string]string{"PATH": "/usr/bin:/bin:/usr/local/bin"},
			expected: "/usr/bin:/bin:/usr/local/bin",
		},
		{
			name:        "missing environment variable returns error",
			value:       "$MISSING_VAR",
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:        "empty environment variable returns error",
			value:       "$EMPTY_VAR",
			envVars:     map[string]string{"EMPTY_VAR": ""},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := env.NewFromMap(tt.envVars)
			resolver := NewEnvironmentVariableResolver(testEnv)

			result, err := resolver.ResolveValue(tt.value)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNewShellVariableResolver(t *testing.T) {
	testEnv := env.NewFromMap(map[string]string{"TEST": "value"})
	resolver := NewShellVariableResolver(testEnv)

	assert.NotNil(t, resolver)
	assert.Implements(t, (*VariableResolver)(nil), resolver)
}

func TestNewEnvironmentVariableResolver(t *testing.T) {
	testEnv := env.NewFromMap(map[string]string{"TEST": "value"})
	resolver := NewEnvironmentVariableResolver(testEnv)

	assert.NotNil(t, resolver)
	assert.Implements(t, (*VariableResolver)(nil), resolver)
}
