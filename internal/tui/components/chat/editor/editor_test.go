package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
)

func TestHandleQuickCommand(t *testing.T) {
	// Create a test editor component
	editor := &editorCmp{
		session: session.Session{ID: "test-session"},
		app:     &app.App{},
	}

	tests := []struct {
		name        string
		command     string
		expectedMsg interface{}
		shouldError bool
	}{
		{
			name:        "new command",
			command:     "/new",
			expectedMsg: commands.NewSessionsMsg{},
		},
		{
			name:        "switch command",
			command:     "/switch",
			expectedMsg: commands.SwitchSessionsMsg{},
		},
		{
			name:        "model command",
			command:     "/model",
			expectedMsg: commands.SwitchModelMsg{},
		},
		{
			name:        "exit command",
			command:     "/exit",
			expectedMsg: nil, // This opens a dialog, harder to test
		},
		{
			name:        "quit command",
			command:     "/quit",
			expectedMsg: nil, // This opens a dialog, harder to test
		},
		{
			name:        "compact command",
			command:     "/compact",
			expectedMsg: commands.CompactMsg{SessionID: "test-session"},
		},
		{
			name:        "summarize command",
			command:     "/summarize",
			expectedMsg: commands.CompactMsg{SessionID: "test-session"},
		},
		{
			name:        "help command",
			command:     "/help",
			expectedMsg: commands.ToggleHelpMsg{},
		},
		{
			name:        "invalid command",
			command:     "/invalid",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := editor.handleQuickCommand(tt.command)

			if tt.shouldError {
				// For invalid commands, we expect a ReportWarn which returns a tea.Cmd
				if cmd == nil {
					t.Errorf("Expected error for command %s, but got nil", tt.command)
				}
				return
			}

			if cmd == nil && tt.expectedMsg != nil {
				t.Errorf("Expected command for %s, but got nil", tt.command)
				return
			}

			// For commands like /exit and /quit that open dialogs,
			// we just check that a command was returned
			if tt.expectedMsg == nil && cmd != nil {
				return // This is expected for dialog commands
			}
		})
	}
}

func TestHandleFileReference(t *testing.T) {
	// Create a temporary directory and file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10"

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a test editor component
	editor := &editorCmp{
		session: session.Session{ID: "test-session"},
		app:     &app.App{},
	}

	tests := []struct {
		name            string
		reference       string
		expectedContent string
		shouldError     bool
	}{
		{
			name:            "full file reference",
			reference:       "#" + testFile,
			expectedContent: "Here's the content of test.txt:",
		},
		{
			name:            "line number reference",
			reference:       "#" + testFile + ":5",
			expectedContent: "Here's the content around line 5 in test.txt:",
		},
		{
			name:        "empty reference",
			reference:   "#",
			shouldError: true,
		},
		{
			name:        "non-existent file",
			reference:   "#/non/existent/file.txt",
			shouldError: true,
		},
		{
			name:        "invalid line number",
			reference:   "#" + testFile + ":999",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := editor.handleFileReference(tt.reference, nil)

			if tt.shouldError {
				// For error cases, we expect a ReportError which returns a tea.Cmd
				if cmd == nil {
					t.Errorf("Expected error for reference %s, but got nil", tt.reference)
				}
				return
			}

			if cmd == nil {
				t.Errorf("Expected command for %s, but got nil", tt.reference)
				return
			}

			// For successful file references, we just check that a command was returned
			// The actual message content testing would require more complex integration testing
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"123", 123},
		{"", 0},
		{"abc", 0},
		{"12a", 0},
		{"a12", 0},
		{"-5", 0}, // negative numbers return 0
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSendCommandRouter(t *testing.T) {
	// Create a test editor component with minimal setup
	editor := &editorCmp{
		session: session.Session{ID: "test-session"},
		app:     &app.App{},
	}

	// Create a simple textarea for testing
	textArea := textarea.New()
	editor.textarea = textArea

	tests := []struct {
		name          string
		input         string
		expectCommand bool
	}{
		{
			name:          "quick command",
			input:         "/new",
			expectCommand: true,
		},
		{
			name:          "file reference",
			input:         "#test.txt",
			expectCommand: true,
		},
		{
			name:          "shell command (passed to AI)",
			input:         ">ls -la",
			expectCommand: true,
		},
		{
			name:          "file search (passed to AI)",
			input:         "f:*.go",
			expectCommand: true,
		},
		{
			name:          "regular text",
			input:         "hello world",
			expectCommand: true,
		},
		{
			name:          "empty input",
			input:         "",
			expectCommand: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textArea.SetValue(tt.input)
			cmd := editor.send()

			if tt.expectCommand && cmd == nil {
				t.Errorf("Expected command for input %s, but got nil", tt.input)
			}
			if !tt.expectCommand && cmd != nil {
				t.Errorf("Expected no command for input %s, but got one", tt.input)
			}
		})
	}
}
