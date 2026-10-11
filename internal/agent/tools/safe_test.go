package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsCommandChaining(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"plain ls", "ls -la", false},
		{"plain echo", "echo hello world", false},
		{"plain pwd", "pwd", false},
		{"plain git status", "git status", false},
		{"ls with redirect", "ls > /tmp/out", false},
		{"ls with pipe", "ls | grep foo", true},
		{"ls with double ampersand", "ls && echo done", true},
		{"ls with semicolon", "ls; echo done", true},
		{"ls with pipe pipe", "ls || echo fail", true},
		{"ls with backticks", "ls `echo foo`", true},
		{"ls with subshell", "ls $(echo foo)", true},
		{"ls with background ampersand", "ls & echo done", false},
		{"rm -rf with && ls (rm first)", "rm -rf / && ls", true},
		{"redirect with ampersand gt", "ls &> /dev/null", false},
		{"redirect with gt ampersand", "ls >& /dev/null", false},
		{"simple kill", "kill 1234", false},
		{"kill with pipe", "kill 1234 | echo foo", true},
		{"git log", "git log --oneline", false},
		{"git log with pipe", "git log | head", true},
		{"empty string", "", false},
		{"dollar sign in argument", "echo $HOME", false},
		{"ls then newline rm", "ls\nrm -rf tmpdir", true},
		{"ls with args then newline rm", "ls -la\nrm -rf tmpdir", true},
		{"git status then newline rm", "git status\nrm -rf tmpdir", true},
		{"crlf separator", "ls\r\nrm -rf tmpdir", true},
		{"carriage return separator", "ls\rrm -rf tmpdir", true},
		{"leading newline", "\nrm -rf tmpdir", true},
		{"trailing newline", "ls\n", true},
		{"trailing crlf", "ls\r\n", true},
		{"newline inside single quotes", "echo 'a\nb'", true},
		{"newline inside double quotes", "echo \"a\nb\"", true},
		{"escaped newline continuation", "ls \\\n-la", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := containsCommandChaining(tt.input)
			assert.Equal(t, tt.expected, got, "containsCommandChaining(%q)", tt.input)
		})
	}
}
