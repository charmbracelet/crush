package agent

import (
	"strings"
	"testing"
	"text/template"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/stretchr/testify/require"
)

// The git status block is rebuilt from live git state on every app init,
// including session resume, so it must report when the snapshot was actually
// captured rather than claiming it is from the conversation start.
func TestCoderPromptGitStatusReportsSnapshotTime(t *testing.T) {
	tmpl, err := template.New("coder").Parse(string(coderPromptTmpl))
	require.NoError(t, err)

	var sb strings.Builder
	require.NoError(t, tmpl.Execute(&sb, prompt.PromptDat{
		IsGitRepo:     true,
		GitStatus:     "On branch main\n",
		GitStatusTime: "2025-01-01 00:00 UTC",
	}))
	out := sb.String()

	require.Contains(t, out, "snapshot at 2025-01-01 00:00 UTC")
	require.NotContains(t, out, "snapshot at conversation start")
}
