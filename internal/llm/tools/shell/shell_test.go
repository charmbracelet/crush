package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Benchmark to measure CPU efficiency
func BenchmarkShellQuickCommands(b *testing.B) {
	shell := newPersistentShell(b.TempDir())

	b.ReportAllocs()

	for b.Loop() {
		_, _, err := shell.Exec(context.Background(), "echo test")
		exitCode := ExitCode(err)
		if err != nil || exitCode != 0 {
			b.Fatalf("Command failed: %v, exit code: %d", err, exitCode)
		}
	}
}

func TestTestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	t.Cleanup(cancel)

	shell := newPersistentShell(t.TempDir())
	_, _, err := shell.Exec(ctx, "sleep 10")
	if status := ExitCode(err); status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if !IsInterrupt(err) {
		t.Fatalf("Expected command to be interrupted, but it was not")
	}
	if err == nil {
		t.Fatalf("Expected an error due to timeout, but got none")
	}
}

func TestTestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // immediately cancel the context

	shell := newPersistentShell(t.TempDir())
	_, _, err := shell.Exec(ctx, "sleep 10")
	if status := ExitCode(err); status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if !IsInterrupt(err) {
		t.Fatalf("Expected command to be interrupted, but it was not")
	}
	if err == nil {
		t.Fatalf("Expected an error due to cancel, but got none")
	}
}

func TestRunCommandError(t *testing.T) {
	shell := newPersistentShell(t.TempDir())
	_, _, err := shell.Exec(t.Context(), "nopenopenope")
	if status := ExitCode(err); status == 0 {
		t.Fatalf("Expected non-zero exit status, got %d", status)
	}
	if IsInterrupt(err) {
		t.Fatalf("Expected command to not be interrupted, but it was")
	}
	if err == nil {
		t.Fatalf("Expected an error, got nil")
	}
}

func TestRunContinuity(t *testing.T) {
	shell := newPersistentShell(t.TempDir())
	shell.Exec(t.Context(), "export FOO=bar")
	dst := t.TempDir()
	shell.Exec(t.Context(), "cd "+dst)
	out, _, _ := shell.Exec(t.Context(), "echo $FOO ; pwd")
	expect := "bar\n" + dst + "\n"
	if out != expect {
		t.Fatalf("Expected output %q, got %q", expect, out)
	}
}

// New tests for Windows shell support

func TestShellTypeDetection(t *testing.T) {
	shell := &PersistentShell{}
	
	tests := []struct {
		command    string
		expected   ShellType
		windowsOnly bool
	}{
		// Windows-specific commands
		{"dir", ShellTypeCmd, true},
		{"type file.txt", ShellTypeCmd, true},
		{"copy file1.txt file2.txt", ShellTypeCmd, true},
		{"del file.txt", ShellTypeCmd, true},
		{"md newdir", ShellTypeCmd, true},
		{"tasklist", ShellTypeCmd, true},
		
		// PowerShell commands
		{"Get-Process", ShellTypePowerShell, true},
		{"Get-ChildItem", ShellTypePowerShell, true},
		{"Set-Location C:\\", ShellTypePowerShell, true},
		{"Get-Content file.txt | Where-Object {$_ -match 'pattern'}", ShellTypePowerShell, true},
		{"$files = Get-ChildItem", ShellTypePowerShell, true},
		
		// Unix/cross-platform commands
		{"ls -la", ShellTypePOSIX, false},
		{"cat file.txt", ShellTypePOSIX, false},
		{"grep pattern file.txt", ShellTypePOSIX, false},
		{"echo hello", ShellTypePOSIX, false},
		{"git status", ShellTypePOSIX, false},
		{"go build", ShellTypePOSIX, false},
	}
	
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			result := shell.determineShellType(test.command)
			
			if test.windowsOnly && runtime.GOOS != "windows" {
				// On non-Windows systems, everything should use POSIX
				if result != ShellTypePOSIX {
					t.Errorf("On non-Windows, command %q should use POSIX shell, got %v", test.command, result)
				}
			} else if runtime.GOOS == "windows" {
				// On Windows, check the expected shell type
				if result != test.expected {
					t.Errorf("Command %q should use %v shell, got %v", test.command, test.expected, result)
				}
			}
		})
	}
}

func TestWindowsCDHandling(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows CD handling test only runs on Windows")
	}
	
	shell := &PersistentShell{
		cwd: "C:\\Users",
		env: []string{},
	}
	
	tests := []struct {
		command     string
		expectedCwd string
		shouldError bool
	}{
		{"cd ..", "C:\\", false},
		{"cd Documents", "C:\\Users\\Documents", false},
		{"cd C:\\Windows", "C:\\Windows", false},
		{"cd", "", true}, // Missing argument
	}
	
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			originalCwd := shell.cwd
			stdout, stderr, err := shell.handleWindowsCD(test.command)
			
			if test.shouldError {
				if err == nil {
					t.Errorf("Command %q should have failed", test.command)
				}
			} else {
				if err != nil {
					t.Errorf("Command %q failed: %v", test.command, err)
				}
				if shell.cwd != test.expectedCwd {
					t.Errorf("Command %q: expected cwd %q, got %q", test.command, test.expectedCwd, shell.cwd)
				}
			}
			
			// Reset for next test
			shell.cwd = originalCwd
			_ = stdout
			_ = stderr
		})
	}
}

func TestCrossPlatformExecution(t *testing.T) {
	shell := newPersistentShell(".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test a simple command that should work on all platforms
	stdout, stderr, err := shell.Exec(ctx, "echo hello")
	if err != nil {
		t.Fatalf("Echo command failed: %v, stderr: %s", err, stderr)
	}
	
	if stdout == "" {
		t.Error("Echo command produced no output")
	}
	
	// The output should contain "hello" regardless of platform
	if !strings.Contains(strings.ToLower(stdout), "hello") {
		t.Errorf("Echo output should contain 'hello', got: %q", stdout)
	}
}

func TestWindowsNativeCommands(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows native command test only runs on Windows")
	}
	
	shell := newPersistentShell(".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test Windows dir command
	stdout, stderr, err := shell.Exec(ctx, "dir")
	if err != nil {
		t.Fatalf("Dir command failed: %v, stderr: %s", err, stderr)
	}
	
	if stdout == "" {
		t.Error("Dir command produced no output")
	}
}
