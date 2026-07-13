package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolPolicyTemplatesPreferMeasuredHostFacts(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"templates/coder.md.tpl",
		"templates/task.md.tpl",
		"templates/plan.md.tpl",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(path)
			require.NoError(t, err)
			text := string(content)

			require.Contains(t, text, "shell for host/runtime facts")
			require.True(t,
				strings.Contains(text, "finite measured command output") ||
					strings.Contains(text, "bounded shell commands that produce finite measured output"),
				"host facts policy should require measured command output",
			)
			require.Contains(t, text, "fetch/web_fetch only for HTTP(S) URLs")
		})
	}
}
