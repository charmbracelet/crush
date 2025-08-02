package shell

import (
	"context"
	"strings"
	"testing"
)

func TestCommandBlocking(t *testing.T) {
	tests := []struct {
		name        string
		blockFuncs  []BlockFunc
		command     string
		shouldBlock bool
	}{
		{
			name: "block simple command",
			blockFuncs: []BlockFunc{
				func(args []string) bool {
					return len(args) > 0 && args[0] == "curl"
				},
			},
			command:     "curl https://example.com",
			shouldBlock: true,
		},
		{
			name: "allow non-blocked command",
			blockFuncs: []BlockFunc{
				func(args []string) bool {
					return len(args) > 0 && args[0] == "curl"
				},
			},
			command:     "echo hello",
			shouldBlock: false,
		},
		{
			name: "block subcommand",
			blockFuncs: []BlockFunc{
				func(args []string) bool {
					return len(args) >= 2 && args[0] == "brew" && args[1] == "install"
				},
			},
			command:     "brew install wget",
			shouldBlock: true,
		},
		{
			name: "allow different subcommand",
			blockFuncs: []BlockFunc{
				func(args []string) bool {
					return len(args) >= 2 && args[0] == "brew" && args[1] == "install"
				},
			},
			command:     "brew list",
			shouldBlock: false,
		},
		{
			name: "block npm global install with -g",
			blockFuncs: []BlockFunc{
				ArgumentsBlocker([][]string{
					{"npm", "install", "-g"},
					{"npm", "install", "--global"},
				}),
			},
			command:     "npm install -g typescript",
			shouldBlock: true,
		},
		{
			name: "block npm global install with --global",
			blockFuncs: []BlockFunc{
				ArgumentsBlocker([][]string{
					{"npm", "install", "-g"},
					{"npm", "install", "--global"},
				}),
			},
			command:     "npm install --global typescript",
			shouldBlock: true,
		},
		{
			name: "allow npm local install",
			blockFuncs: []BlockFunc{
				ArgumentsBlocker([][]string{
					{"npm", "install", "-g"},
					{"npm", "install", "--global"},
				}),
			},
			command:     "npm install typescript",
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for each test
			tmpDir := t.TempDir()

			shell := NewShell(&Options{
				WorkingDir: tmpDir,
				BlockFuncs: tt.blockFuncs,
			})

			_, _, err := shell.Exec(context.Background(), tt.command)

			if tt.shouldBlock {
				if err == nil {
					t.Errorf("Expected command to be blocked, but it was allowed")
				} else if !strings.Contains(err.Error(), "not allowed for security reasons") {
					t.Errorf("Expected security error, got: %v", err)
				}
			} else {
				// For non-blocked commands, we might get other errors (like command not found)
				// but we shouldn't get the security error
				if err != nil && strings.Contains(err.Error(), "not allowed for security reasons") {
					t.Errorf("Command was unexpectedly blocked: %v", err)
				}
			}
		})
	}
}
