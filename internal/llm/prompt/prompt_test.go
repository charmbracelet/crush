package prompt

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected func() string
	}{
		{
			name:  "regular path unchanged",
			input: "/absolute/path",
			expected: func() string {
				return "/absolute/path"
			},
		},
		{
			name:  "tilde expansion",
			input: "~/documents",
			expected: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, "documents")
			},
		},
		{
			name:  "tilde only",
			input: "~",
			expected: func() string {
				home, _ := os.UserHomeDir()
				return home
			},
		},
		{
			name:  "environment variable expansion",
			input: "$HOME",
			expected: func() string {
				return os.Getenv("HOME")
			},
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			expected: func() string {
				return "relative/path"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			expected := tt.expected()

			// Skip test if environment variable is not set
			if strings.HasPrefix(tt.input, "$") && expected == "" {
				t.Skip("Environment variable not set")
			}

			if result != expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, expected)
			}
		})
	}
}

func TestProcessContextPaths(t *testing.T) {
	// Create a temporary directory and file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content"

	err := os.WriteFile(testFile, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with absolute path to file
	result := processContextPaths("", []string{testFile})
	expected := "# From:" + testFile + "\n" + testContent

	if result != expected {
		t.Errorf("processContextPaths with absolute path failed.\nGot: %q\nWant: %q", result, expected)
	}

	// Test with directory path (should process all files in directory)
	result = processContextPaths("", []string{tmpDir})
	if !strings.Contains(result, testContent) {
		t.Errorf("processContextPaths with directory path failed to include file content")
	}

	// Test with tilde expansion (if we can create a file in home directory)
	tmpDir = t.TempDir()
	setHomeEnv(t, tmpDir)
	homeTestFile := filepath.Join(tmpDir, "crush_test_file.txt")
	err = os.WriteFile(homeTestFile, []byte(testContent), 0o644)
	if err == nil {
		defer os.Remove(homeTestFile) // Clean up

		tildeFile := "~/crush_test_file.txt"
		result = processContextPaths("", []string{tildeFile})
		expected = "# From:" + homeTestFile + "\n" + testContent

		if result != expected {
			t.Errorf("processContextPaths with tilde expansion failed.\nGot: %q\nWant: %q", result, expected)
		}
	}
}

func setHomeEnv(tb testing.TB, path string) {
	tb.Helper()
	key := "HOME"
	if runtime.GOOS == "windows" {
		key = "USERPROFILE"
	}
	tb.Setenv(key, path)
}
