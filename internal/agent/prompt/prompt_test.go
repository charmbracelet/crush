package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Collecting the git status spawns git twice, so a prompt only pays for it
// when its template can render it.
func TestNewPromptNeedsGitStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		want     bool
	}{
		{name: "field", template: "{{.GitStatus}}", want: true},
		{name: "conditional", template: "{{if .GitStatus}}x{{end}}", want: true},
		{name: "index", template: `{{index . "GitStatus"}}`, want: true},
		{name: "repo flag only", template: "{{if .IsGitRepo}}yes{{end}}", want: false},
		{name: "plain", template: "hello", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := NewPrompt(tt.name, tt.template)
			require.NoError(t, err)
			require.Equal(t, tt.want, p.needsGitStatus)
		})
	}
}

// The task subagent prompt is rebuilt on every turn, so it must not run git
// for a status it never shows. The coder prompt does show it.
func TestBuiltinTemplatesNeedGitStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"coder.md.tpl":                true,
		"task.md.tpl":                 false,
		"plan.md.tpl":                 false,
		"initialize.md.tpl":           false,
		"agentic_fetch_prompt.md.tpl": false,
	}
	for file, want := range tests {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			text, err := os.ReadFile(filepath.Join("..", "templates", file))
			require.NoError(t, err)
			p, err := NewPrompt(file, string(text))
			require.NoError(t, err)
			require.Equal(t, want, p.needsGitStatus)
		})
	}
}
