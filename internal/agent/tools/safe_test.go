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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := containsCommandChaining(tt.input)
			assert.Equal(t, tt.expected, got, "containsCommandChaining(%q)", tt.input)
		})
	}
}

func TestIsSafeReadOnlyCommand_GitQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command string
		safe    bool
	}{
		{command: "git status", safe: true},
		{command: "git diff --stat", safe: true},
		{command: "git log --oneline", safe: true},
		{command: "git branch", safe: true},
		{command: "git branch --show-current", safe: true},
		{command: "git branch --list 'feature/*'", safe: true},
		{command: "git branch -a", safe: true},
		{command: "git branch new-branch", safe: false},
		{command: "git branch -D old-branch", safe: false},
		{command: "git branch -m old new", safe: false},
		{command: "git branch --all -D old-branch", safe: false},
		{command: "git branch --list $BRANCH_FLAGS", safe: false},
		{command: "git tag", safe: true},
		{command: "git tag --list 'v*'", safe: true},
		{command: "git tag -l 'v*'", safe: true},
		{command: "git tag v1.0.0", safe: false},
		{command: "git tag -d v1.0.0", safe: false},
		{command: "git tag --list --force v1.0.0", safe: false},
		{command: "git remote", safe: true},
		{command: "git remote -v", safe: true},
		{command: "git remote show origin", safe: true},
		{command: "git remote get-url origin", safe: true},
		{command: "git remote add origin https://example.com/repo.git", safe: false},
		{command: "git remote remove origin", safe: false},
		{command: "git remote set-url origin https://example.com/repo.git", safe: false},
		{command: "git remote rename origin upstream", safe: false},
		{command: "git status && git branch new-branch", safe: false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.safe, isSafeReadOnlyCommand(tt.command))
		})
	}
}
