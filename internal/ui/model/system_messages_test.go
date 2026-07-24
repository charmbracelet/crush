package model

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// fakeMessageItem is a minimal chat.MessageItem for ordering tests.
type fakeMessageItem struct {
	*list.Versioned
	id string
}

func newFakeItem(id string) *fakeMessageItem {
	return &fakeMessageItem{Versioned: list.NewVersioned(), id: id}
}

func (f *fakeMessageItem) Render(int) string    { return f.id }
func (f *fakeMessageItem) RawRender(int) string { return f.id }
func (f *fakeMessageItem) Finished() bool       { return true }
func (f *fakeMessageItem) ID() string           { return f.id }

func TestSystemMessagesAnchoredAtEnd(t *testing.T) {
	t.Parallel()

	c := NewChat(&common.Common{}, config.ScrollbarDefault)
	sty := styles.CharmtonePantera()
	sys := chat.NewSystemMessageItem(&sty, chat.SystemMessageContextWarning, "warn", "body")

	c.SetMessages(newFakeItem("a"), newFakeItem("b"))
	c.SetSystemMessages(sys)
	require.Equal(t, 2, c.idInxMap[sys.ID()], "advisory sits after the transcript")

	// A new transcript message keeps the advisory anchored last.
	c.AppendMessages(newFakeItem("c"))
	require.Equal(t, 3, c.idInxMap[sys.ID()], "advisory stays last after append")
	require.Equal(t, 2, c.idInxMap["c"], "new message lands before the advisory")

	// Removing the advisory leaves only the transcript.
	c.SetSystemMessages()
	_, ok := c.idInxMap[sys.ID()]
	require.False(t, ok, "advisory cleared")
	require.Equal(t, 2, c.idInxMap["c"])
}

func TestFormatContextWindow(t *testing.T) {
	t.Parallel()

	require.Equal(t, "5K", formatContextWindow(5000))
	require.Equal(t, "200K", formatContextWindow(200_000))
	require.Equal(t, "128K", formatContextWindow(128_000))
	require.Equal(t, "1.5K", formatContextWindow(1500))
	require.Equal(t, "800", formatContextWindow(800))
}

func TestComputeSystemMessages(t *testing.T) {
	t.Parallel()

	kinds := func(items []chat.MessageItem) []chat.SystemMessageKind {
		out := make([]chat.SystemMessageKind, 0, len(items))
		for _, it := range items {
			out = append(out, it.(*chat.SystemMessageItem).Kind())
		}
		return out
	}

	t.Run("small context model warns", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 5000, permission.PermissionModeNormal)
		require.Equal(t, []chat.SystemMessageKind{chat.SystemMessageContextWarning}, kinds(ui.computeSystemMessages()))
	})

	t.Run("large context model is silent", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 400_000, permission.PermissionModeNormal)
		require.Empty(t, ui.computeSystemMessages())
	})

	t.Run("unknown context window is silent", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 0, permission.PermissionModeNormal)
		require.Empty(t, ui.computeSystemMessages())
	})

	t.Run("super yolo warns", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 400_000, permission.PermissionModeSysadmin)
		require.Equal(t, []chat.SystemMessageKind{chat.SystemMessageSuperYolo}, kinds(ui.computeSystemMessages()))
	})

	t.Run("both conditions warn together", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 5000, permission.PermissionModeSysadmin)
		require.Equal(t, []chat.SystemMessageKind{
			chat.SystemMessageContextWarning,
			chat.SystemMessageSuperYolo,
		}, kinds(ui.computeSystemMessages()))
	})

	t.Run("disable option silences everything", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 5000, permission.PermissionModeSysadmin)
		ui.com.Workspace.(*sysTestWorkspace).cfg.Options.DisableSystemWarnings = true
		require.Empty(t, ui.computeSystemMessages())
	})
}

func TestSystemMessageSuppression(t *testing.T) {
	t.Parallel()

	t.Run("dismiss hides the advisory until re-triggered", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 5000, permission.PermissionModeNormal)
		require.Len(t, ui.computeSystemMessages(), 1, "warning shows before dismissal")

		ui.dismissActiveSystemMessages()
		require.Empty(t, ui.computeSystemMessages(), "warning hidden after dismissal")

		// Switching models re-triggers the context advisory.
		ui.retriggerSystemMessage(chat.SystemMessageContextWarning)
		require.Len(t, ui.computeSystemMessages(), 1, "warning shows again after model switch")
	})

	t.Run("reset clears every dismissal", func(t *testing.T) {
		t.Parallel()
		ui := newSystemMessagesUI(t, 5000, permission.PermissionModeSysadmin)
		ui.dismissActiveSystemMessages()
		require.Empty(t, ui.computeSystemMessages())

		ui.resetSystemMessageSuppression()
		require.Len(t, ui.computeSystemMessages(), 2, "both advisories return after reset")
	})
}

func newSystemMessagesUI(t *testing.T, contextWindow int64, mode permission.PermissionMode) *UI {
	t.Helper()
	sty := styles.CharmtonePantera()
	com := &common.Common{
		Styles: &sty,
		Workspace: &sysTestWorkspace{
			cfg:  &config.Config{Options: &config.Options{}},
			mode: mode,
			model: workspace.AgentModel{
				CatwalkCfg: catwalk.Model{Name: "Test Model", ContextWindow: contextWindow},
				ModelCfg:   config.SelectedModel{Provider: "test"},
			},
		},
	}
	return &UI{
		com:                      com,
		chat:                     NewChat(com, config.ScrollbarDefault),
		keyMap:                   DefaultKeyMap(),
		suppressedSystemMessages: make(map[chat.SystemMessageKind]bool),
	}
}

// sysTestWorkspace is a workspace stub exposing just enough for the
// system-message computation to run.
type sysTestWorkspace struct {
	workspace.Workspace
	cfg   *config.Config
	mode  permission.PermissionMode
	model workspace.AgentModel
}

func (w *sysTestWorkspace) Config() *config.Config                    { return w.cfg }
func (w *sysTestWorkspace) AgentIsReady() bool                        { return true }
func (w *sysTestWorkspace) AgentModel() workspace.AgentModel          { return w.model }
func (w *sysTestWorkspace) PermissionMode() permission.PermissionMode { return w.mode }
