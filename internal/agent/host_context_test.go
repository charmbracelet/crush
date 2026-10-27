package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestHostContextPrompt(t *testing.T) {
	t.Parallel()

	require.Empty(t, hostContextPrompt(proto.HostContext{}))
	require.Contains(t, hostContextPrompt(proto.HostContext{
		ActiveFile: "/src/main.go",
		Line:       42,
		OpenFiles:  []string{"/src/main.go", "/src/util.go"},
	}), "Current file: /src/main.go, cursor on line 42\nOpen files:\n- /src/main.go\n- /src/util.go\n</host_context>")
}
