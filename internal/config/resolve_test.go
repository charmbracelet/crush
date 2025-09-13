package config

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/stretchr/testify/require"
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestShellVariableResolver_EnhancedResolveValue(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		envVars     map[string]string
		shellFunc   func(ctx context.Context, command string) (stdout, stderr string, err error)
		expected    string
		expectError bool
	}{
		{
			name:  "command substitution within string",
			value: "Bearer $(echo token123)",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				if command == "echo token123" {
					return "token123\n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "Bearer token123",
		},
		{
			name:     "environment variable within string",
			value:    "Bearer $TOKEN",
			envVars:  map[string]string{"TOKEN": "sk-ant-123"},
			expected: "Bearer sk-ant-123",
		},
		{
			name:     "environment variable with braces within string",
			value:    "Bearer ${TOKEN}",
			envVars:  map[string]string{"TOKEN": "sk-ant-456"},
			expected: "Bearer sk-ant-456",
		},
		{
			name:  "mixed command and environment substitution",
			value: "$USER-$(date +%Y)-$HOST",
			envVars: map[string]string{
				"USER": "testuser",
				"HOST": "localhost",
			},
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				if command == "date +%Y" {
					return "2024\n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "testuser-2024-localhost",
		},
		{
			name:  "multiple command substitutions",
			value: "$(echo hello) $(echo world)",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				switch command {
				case "echo hello":
					return "hello\n", "", nil
				case "echo world":
					return "world\n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "hello world",
		},
		{
			name:  "nested parentheses in command",
			value: "$(echo $(echo inner))",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				if command == "echo $(echo inner)" {
					return "nested\n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "nested",
		},
		{
			name:        "lone dollar with non-variable chars",
			value:       "prefix$123suffix", // Numbers can't start variable names
			expectError: true,
		},
		{
			name:        "dollar with special chars",
			value:       "a$@b$#c", // Special chars aren't valid in variable names
			expectError: true,
		},
		{
			name:        "empty environment variable substitution",
			value:       "Bearer $EMPTY_VAR",
			envVars:     map[string]string{},
			expectError: true,
		},
		{
			name:        "unmatched command substitution opening",
			value:       "Bearer $(echo test",
			expectError: true,
		},
		{
			name:        "unmatched environment variable braces",
			value:       "Bearer ${TOKEN",
			expectError: true,
		},
		{
			name:  "command substitution with error",
			value: "Bearer $(false)",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				return "", "", errors.New("command failed")
			},
			expectError: true,
		},
		{
			name:  "complex real-world example",
			value: "Bearer $(cat /tmp/token.txt | base64 -w 0)",
			shellFunc: func(ctx context.Context, command string) (stdout, stderr string, err error) {
				if command == "cat /tmp/token.txt | base64 -w 0" {
					return "c2stYW50LXRlc3Q=\n", "", nil
				}
				return "", "", errors.New("unexpected command")
			},
			expected: "Bearer c2stYW50LXRlc3Q=",
		},
		{
			name:     "environment variable with underscores and numbers",
			value:    "Bearer $API_KEY_V2",
			envVars:  map[string]string{"API_KEY_V2": "sk-test-123"},
			expected: "Bearer sk-test-123",
		},
		{
			name:     "no substitution needed",
			value:    "Bearer sk-ant-static-token",
			expected: "Bearer sk-ant-static-token",
		},
		{
			name:        "incomplete variable at end",
			value:       "Bearer $",
			expectError: true,
		},
		{
			name:        "variable with invalid character",
			value:       "Bearer $VAR-NAME", // Hyphen not allowed in variable names
			expectError: true,
		},
		{
			name:        "multiple invalid variables",
			value:       "$1$2$3",
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNewShellVariableResolver(t *testing.T) {
	testEnv := env.NewFromMap(map[string]string{"TEST": "value"})
	resolver := NewShellVariableResolver(testEnv)

	require.NotNil(t, resolver)
	require.Implements(t, (*VariableResolver)(nil), resolver)
}

func TestNewEnvironmentVariableResolver(t *testing.T) {
	testEnv := env.NewFromMap(map[string]string{"TEST": "value"})
	resolver := NewEnvironmentVariableResolver(testEnv)

	require.NotNil(t, resolver)
	require.Implements(t, (*VariableResolver)(nil), resolver)
}

func TestMCPConfig_ResolvedCommand(t *testing.T) {
	homeDir := home.Dir()

	tests := []struct {
		name     string
		command  string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "plain command",
			command:  "gopls",
			expected: "gopls",
		},
		{
			name:     "command with HOME variable",
			command:  "$HOME/bin/gopls",
			envVars:  map[string]string{"HOME": "/Users/testuser"},
			expected: "/Users/testuser/bin/gopls",
		},
		{
			name:     "command with tilde expansion",
			command:  "~/bin/gopls",
			expected: filepath.Join(homeDir, "bin/gopls"),
		},
		{
			name:     "command with just tilde",
			command:  "~",
			expected: homeDir,
		},
		{
			name:     "command with braced variable",
			command:  "${HOME}/bin/gopls",
			envVars:  map[string]string{"HOME": "/Users/testuser"},
			expected: "/Users/testuser/bin/gopls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			config := MCPConfig{
				Command: tt.command,
			}

			result := config.ResolvedCommand()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestLSPConfig_ResolvedCommand(t *testing.T) {
	homeDir := home.Dir()

	tests := []struct {
		name     string
		command  string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "plain command",
			command:  "gopls",
			expected: "gopls",
		},
		{
			name:     "command with HOME variable",
			command:  "$HOME/bin/gopls",
			envVars:  map[string]string{"HOME": "/Users/testuser"},
			expected: "/Users/testuser/bin/gopls",
		},
		{
			name:     "command with tilde expansion",
			command:  "~/bin/gopls",
			expected: filepath.Join(homeDir, "bin/gopls"),
		},
		{
			name:     "command with just tilde",
			command:  "~",
			expected: homeDir,
		},
		{
			name:     "command with XDG_CONFIG_HOME variable",
			command:  "$XDG_CONFIG_HOME/lsp/gopls",
			envVars:  map[string]string{"XDG_CONFIG_HOME": "/Users/testuser/.config"},
			expected: "/Users/testuser/.config/lsp/gopls",
		},
		{
			name:     "command with braced variable",
			command:  "${HOME}/bin/gopls",
			envVars:  map[string]string{"HOME": "/Users/testuser"},
			expected: "/Users/testuser/bin/gopls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			config := LSPConfig{
				Command: tt.command,
			}

			result := config.ResolvedCommand()
			require.Equal(t, tt.expected, result)
		})
	}
}
