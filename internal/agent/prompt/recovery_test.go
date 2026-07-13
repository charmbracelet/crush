package prompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderRecoveryContextKeepsCriticalStateWithinBound(t *testing.T) {
	t.Parallel()

	result := renderRecoveryContext(
		"C:\\work\\example",
		"windows",
		[]ContextFile{{Path: "C:\\work\\example\\AGENTS.md", Content: strings.Repeat("project rule ", 100)}},
		240,
	)

	require.LessOrEqual(t, len([]rune(result)), 240)
	require.Contains(t, result, "C:/work/example")
	require.Contains(t, result, "Host platform: windows")
	require.Contains(t, result, "AGENTS.md")
	require.Contains(t, result, "[project context truncated]")
}

func TestRenderRecoveryContextWithoutFilesStillIdentifiesWorkspace(t *testing.T) {
	t.Parallel()

	result := renderRecoveryContext("/workspace/project", "linux", nil, 500)

	require.Contains(t, result, "Current working directory: /workspace/project")
	require.Contains(t, result, "Host platform: linux")
}
