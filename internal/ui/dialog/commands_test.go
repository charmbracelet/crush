package dialog

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
)

// configWorkspace is the minimum the commands dialog needs: defaultCommands
// reads configuration through common.Common. The embedded interface panics on
// anything else, which is the point — building the command list must not
// reach the workspace for anything but config.
type configWorkspace struct {
	workspace.Workspace
}

func (configWorkspace) Config() *config.Config { return &config.Config{} }

func hasCommandAction(items []*CommandItem, action Action) bool {
	for _, item := range items {
		if item.Action() == action {
			return true
		}
	}
	return false
}

// TestDefaultCommandsClearQueueRequiresQueue pins the only queue-discard path
// the TUI has: esc moves the whole queue into the input field and shift+up
// moves one message at a time, so this entry is how a queue is thrown away
// instead of pasted back. It must be offered whenever prompts are queued —
// and never when there is nothing to discard.
func TestDefaultCommandsClearQueueRequiresQueue(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	com := &common.Common{Workspace: configWorkspace{}, Styles: &s}

	withQueue := &Commands{com: com, sessionID: "s1", hasSession: true, hasQueue: true}
	require.True(t, hasCommandAction(withQueue.defaultCommands(), ActionClearQueue{}),
		"a non-empty queue must offer a bulk clear")

	withoutQueue := &Commands{com: com, sessionID: "s1", hasSession: true}
	require.False(t, hasCommandAction(withoutQueue.defaultCommands(), ActionClearQueue{}),
		"an empty queue must not offer a clear")
}
