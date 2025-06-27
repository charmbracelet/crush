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
