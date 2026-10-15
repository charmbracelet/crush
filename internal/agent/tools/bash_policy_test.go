package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// Lệnh bash gọi thẳng CLI agent khác bị chặn ngay trong shell (cả khi lồng
// trong chuỗi lệnh); lệnh thường vẫn chạy.
func TestBashBlocksExternalAgentCLIs(t *testing.T) {
	t.Parallel()
	sh := shell.NewShell(&shell.Options{WorkingDir: t.TempDir(), BlockFuncs: blockFuncs()})
	for _, cmd := range []string{
		"claude -p hi",
		"true && codex exec x",
		"env FOO=1 gemini -p x",
		"npx -y @anthropic-ai/claude-code -p x",
	} {
		_, _, err := sh.Exec(t.Context(), cmd)
		require.Errorf(t, err, "%q", cmd)
		require.Containsf(t, err.Error(), "not allowed", "%q", cmd)
	}
	out, _, err := sh.Exec(t.Context(), "echo claude")
	require.NoError(t, err)
	require.Equal(t, "claude\n", out)
}
