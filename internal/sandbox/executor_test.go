package sandbox

import (
	"context"
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
