package critic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCriticPrompt_DefaultTemplate(t *testing.T) {
	t.Parallel()
	cp := Checkpoint{
		Type:        CheckpointEdit,
		PrimaryDiff: "-old\n+new",
		LSPDiagnostics: []DiagnosticSnapshot{
			{Path: "foo.go", Severity: "error", Message: "undefined", Line: 42},
		},
	}
	prompt, err := BuildCriticPrompt(cp, "")
	require.NoError(t, err)
	require.Contains(t, prompt, "senior staff engineer")
	require.Contains(t, prompt, "-old")
	require.Contains(t, prompt, "error")
	require.Contains(t, prompt, "undefined")
	require.Contains(t, prompt, "approve | revise | halt")
}

func TestBuildCriticPrompt_NoDiffNoDiags(t *testing.T) {
	t.Parallel()
	cp := Checkpoint{Type: CheckpointEdit, PrimaryPlan: "do X then Y"}
	prompt, err := BuildCriticPrompt(cp, "")
	require.NoError(t, err)
	require.Contains(t, prompt, "do X then Y")
	require.NotContains(t, prompt, "Objective Signals")
}

func TestBuildCriticPrompt_WithProjectContext(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("Use gofumpt."), 0o644))

	cp := Checkpoint{Type: CheckpointEdit, PrimaryDiff: "-old\n+new"}
	prompt, err := BuildCriticPrompt(cp, tmp)
	require.NoError(t, err)
	require.Contains(t, prompt, "Use gofumpt.")
	require.Contains(t, prompt, "<<<CONTEXT_BEGIN>>>")
}

func TestLoadProjectContext_Truncation(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	large := make([]byte, maxProjectContextBytes+100)
	for i := range large {
		large[i] = 'x'
	}
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "AGENTS.md"), large, 0o644))

	ctx := loadProjectContext(tmp)
	require.True(t, strings.HasSuffix(ctx, "... (truncated)\n"))
	require.LessOrEqual(t, len(ctx), maxProjectContextBytes+20)
}

func TestLoadTemplate_Fallback(t *testing.T) {
	t.Parallel()
	text, err := loadTemplate()
	require.NoError(t, err)
	require.NotEmpty(t, text)
	require.True(t, strings.Contains(text, "senior staff engineer"))
}
