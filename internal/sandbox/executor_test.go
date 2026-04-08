package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecuteSimple(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	result, err := e.Execute(context.Background(), "echo hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result.Stdout)
	}
}

func TestExecuteDenyPath(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	_, err := e.Execute(context.Background(), "cat /etc/shadow", nil)
	if err == nil {
		t.Fatal("expected error for denied path, got nil")
	}
}

func TestExecuteTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 500 * time.Millisecond

	e := NewExecutor(cfg)
	result, _ := e.Execute(context.Background(), "sleep 10", nil)
	if !result.Killed {
		t.Error("expected process to be killed on timeout")
	}
	if result.ExitCode != 137 {
		t.Errorf("expected exit code 137, got %d", result.ExitCode)
	}
}

func TestValidateCommand(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	cfg := DefaultConfig()

	tests := []struct {
		cmd     string
		wantErr bool
	}{
		{"ls -la /var/log", false},
		{"cat /etc/shadow", true},
		{"cat /etc/sudoers", true},
		{"ls /root/.ssh", true},
		{"echo hello", false},
	}

	for _, tt := range tests {
		err := e.validateCommand(tt.cmd, &cfg)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateCommand(%q) error = %v, wantErr = %v", tt.cmd, err, tt.wantErr)
		}
	}
}

func TestValidateCommandPathTraversal(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	cfg := DefaultConfig()

	// Path-traversal attempts that should still be blocked
	traversals := []string{
		"cat /etc/../etc/shadow",
		"cat /etc/./shadow",
		"cat /etc//shadow",
	}
	for _, cmd := range traversals {
		t.Run(cmd, func(t *testing.T) {
			err := e.validateCommand(cmd, &cfg)
			if err == nil {
				t.Errorf("expected validateCommand(%q) to return an error", cmd)
			}
		})
	}
}

func TestValidateCommandCustomDenyPaths(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	cfg := DefaultConfig()
	cfg.DenyPaths = append(cfg.DenyPaths, "/custom/secret")

	if err := e.validateCommand("cat /custom/secret/key.pem", &cfg); err == nil {
		t.Error("expected error for custom deny path, got nil")
	}
	if err := e.validateCommand("cat /custom/other/file", &cfg); err != nil {
		t.Errorf("unexpected error for non-denied path: %v", err)
	}
}

func TestExecuteExitCode(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	result, _ := e.Execute(context.Background(), "exit 42", nil)
	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecuteStderr(t *testing.T) {
	e := NewExecutor(DefaultConfig())
	result, _ := e.Execute(context.Background(), "echo err >&2", nil)
	if result.Stderr == "" {
		t.Error("expected non-empty stderr")
	}
}

func TestLimitOutput(t *testing.T) {
	long := strings.Repeat("x", 50000)
	limited := limitOutput(long, 30000)
	if !strings.Contains(limited, "[output truncated") {
		t.Error("expected output to contain truncation marker")
	}

	short := "hello"
	if limitOutput(short, 30000) != short {
		t.Error("short string should not be truncated")
	}
}
