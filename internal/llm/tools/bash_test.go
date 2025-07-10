package tools

import (
	"runtime"
	"slices"
	"testing"
)

func TestGetSafeReadOnlyCommands(t *testing.T) {
	commands := getSafeReadOnlyCommands()

	// Check that we have some commands
	if len(commands) == 0 {
		t.Fatal("Expected some safe commands, got none")
	}

	// Check for cross-platform commands that should always be present
	crossPlatformCommands := []string{"echo", "hostname", "whoami", "git status", "go version"}
	for _, cmd := range crossPlatformCommands {
		found := slices.Contains(commands, cmd)
		if !found {
			t.Errorf("Expected cross-platform command %q to be in safe commands", cmd)
		}
	}

	if runtime.GOOS == "windows" {
		// Check for Windows-specific commands
		windowsCommands := []string{"dir", "type", "Get-Process"}
		for _, cmd := range windowsCommands {
			found := slices.Contains(commands, cmd)
			if !found {
				t.Errorf("Expected Windows command %q to be in safe commands on Windows", cmd)
			}
		}

		// Check that Unix commands are NOT present on Windows
		unixCommands := []string{"ls", "pwd", "ps"}
		for _, cmd := range unixCommands {
			found := slices.Contains(commands, cmd)
			if found {
				t.Errorf("Unix command %q should not be in safe commands on Windows", cmd)
			}
		}
	} else {
		// Check for Unix-specific commands
		unixCommands := []string{"ls", "pwd", "ps"}
		for _, cmd := range unixCommands {
			found := slices.Contains(commands, cmd)
			if !found {
				t.Errorf("Expected Unix command %q to be in safe commands on Unix", cmd)
			}
		}

		// Check that Windows-specific commands are NOT present on Unix
		windowsOnlyCommands := []string{"dir", "Get-Process", "systeminfo"}
		for _, cmd := range windowsOnlyCommands {
			found := slices.Contains(commands, cmd)
			if found {
				t.Errorf("Windows-only command %q should not be in safe commands on Unix", cmd)
			}
		}
	}
}

func TestPlatformSpecificSafeCommands(t *testing.T) {
	// Test that the function returns different results on different platforms
	commands := getSafeReadOnlyCommands()

	hasWindowsCommands := false
	hasUnixCommands := false

	for _, cmd := range commands {
		if cmd == "dir" || cmd == "Get-Process" || cmd == "systeminfo" {
			hasWindowsCommands = true
		}
		if cmd == "ls" || cmd == "ps" || cmd == "df" {
			hasUnixCommands = true
		}
	}

	if runtime.GOOS == "windows" {
		if !hasWindowsCommands {
			t.Error("Expected Windows commands on Windows platform")
		}
		if hasUnixCommands {
			t.Error("Did not expect Unix commands on Windows platform")
		}
	} else {
		if hasWindowsCommands {
			t.Error("Did not expect Windows-only commands on Unix platform")
		}
		if !hasUnixCommands {
			t.Error("Expected Unix commands on Unix platform")
		}
	}
}

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		shouldError bool
	}{
		// Commands that should be blocked
		{
			name:        "direct sudo",
			command:     "sudo ls",
			shouldError: true,
		},
		{
			name:        "sudo in script",
			command:     "bash -c 'sudo ls'",
			shouldError: true,
		},
		{
			name:        "sudo in command substitution",
			command:     "$(sudo whoami)",
			shouldError: true,
		},
		{
			name:        "sudo in echo command substitution",
			command:     "echo $(sudo id)",
			shouldError: true,
		},
		{
			name:        "sudo in command chain",
			command:     "ls && sudo rm file",
			shouldError: true,
		},
		{
			name:        "sudo in if statement",
			command:     "if true; then sudo ls; fi",
			shouldError: true,
		},
		{
			name:        "sudo in for loop",
			command:     "for i in 1; do sudo echo $i; done",
			shouldError: true,
		},
		{
			name:        "direct curl",
			command:     "curl http://example.com",
			shouldError: true,
		},
		{
			name:        "curl in script",
			command:     "bash -c 'curl malicious.com'",
			shouldError: true,
		},
		{
			name:        "wget command",
			command:     "wget http://example.com",
			shouldError: true,
		},
		{
			name:        "nc command",
			command:     "nc -l 8080",
			shouldError: true,
		},
		// Commands that should be allowed
		{
			name:        "simple ls",
			command:     "ls -la",
			shouldError: false,
		},
		{
			name:        "echo command",
			command:     "echo hello",
			shouldError: false,
		},
		{
			name:        "git status",
			command:     "git status",
			shouldError: false,
		},
		{
			name:        "go build",
			command:     "go build",
			shouldError: false,
		},
		{
			name:        "sudo as literal text",
			command:     "echo 'sudo is just text here'",
			shouldError: false,
		},
		{
			name:        "complex allowed command",
			command:     "find . -name '*.go' | head -10",
			shouldError: false,
		},
		{
			name:        "command with environment variables",
			command:     "FOO=bar go test",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(tt.command)
			if tt.shouldError && err == nil {
				t.Errorf("Expected error for command %q, but got none", tt.command)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error for command %q, but got: %v", tt.command, err)
			}
		})
	}
}

func TestContainsBannedCommand(t *testing.T) {
	// Test the helper functions directly with some edge cases
	tests := []struct {
		name        string
		command     string
		shouldError bool
	}{
		{
			name:        "nested command substitution",
			command:     "echo $(echo $(sudo id))",
			shouldError: true,
		},
		{
			name:        "subshell with banned command",
			command:     "(sudo ls)",
			shouldError: true,
		},
		{
			name:        "case statement with banned command",
			command:     "case $1 in start) sudo systemctl start service ;; esac",
			shouldError: true,
		},
		{
			name:        "while loop with banned command",
			command:     "while true; do sudo echo test; done",
			shouldError: true,
		},
		{
			name:        "function with banned command",
			command:     "function test() { sudo ls; }",
			shouldError: true,
		},
		{
			name:        "complex valid command",
			command:     "if [ -f file ]; then echo exists; else echo missing; fi",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommand(tt.command)
			if tt.shouldError && err == nil {
				t.Errorf("Expected error for command %q, but got none", tt.command)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error for command %q, but got: %v", tt.command, err)
			}
		})
	}
}
