package model

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/keybinds"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/stretchr/testify/require"
)

// Every registry action resolves to a binding. If someone adds an ID
// without a case in bindingFor, this fails.
func TestApplyKeybinds_CoversRegistry(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	for _, id := range keybinds.Actions() {
		// completions.* and dialog.* live outside this keymap:
		// completions has its own KeyMap, dialogs apply their
		// subsets at construction.
		if scope, _, _ := strings.Cut(id, "."); scope == "completions" || scope == "dialog" {
			continue
		}
		require.NotNil(t, bindingFor(&km, id), "registry ID %q has no binding", id)
	}
}

func TestApplyKeybinds_OverridesKeysAndHelp(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"global.quit": {"ctrl+q"},
	})

	require.Empty(t, warnings)
	require.Equal(t, []string{"ctrl+q"}, km.Quit.Keys())
	require.Equal(t, "ctrl+q", km.Quit.Help().Key)
	require.Equal(t, "quit", km.Quit.Help().Desc)
}

func TestApplyKeybinds_UnknownWarnsAndKeepsDefault(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	before := km.Quit.Keys()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"bogus.action": {"x"},
	})

	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "bogus.action")
	require.Equal(t, before, km.Quit.Keys())
}

func TestApplyKeybinds_EmptyIsNoop(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	require.Empty(t, ApplyKeybinds(&km, nil))
	require.Empty(t, ApplyKeybinds(&km, map[string][]string{}))
	require.Empty(t, ApplyKeybinds(&km, map[string][]string{"global.quit": {}}))
	require.Equal(t, []string{"ctrl+c"}, km.Quit.Keys())
}

func TestApplyKeybinds_WarningsSorted(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"z.action": {"x"},
		"a.action": {"y"},
	})

	require.Len(t, warnings, 2)
	require.Less(t, warnings[0], warnings[1])
}

func TestApplyKeybinds_ConflictSameDomainWarns(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"chat.copy": {"j"},
	})

	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], `"chat.copy"`)
	require.Contains(t, warnings[0], `"chat.down"`)
	require.Contains(t, warnings[0], `"j"`)
}

func TestApplyKeybinds_GlobalOverlapsEverything(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"editor.send_message": {"ctrl+p"},
	})

	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], `"global.commands"`)
}

func TestApplyKeybinds_CrossFocusDomainSilent(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	// ctrl+n is chat.new_session's key; editor is a separate focus.
	warnings := ApplyKeybinds(&km, map[string][]string{
		"editor.send_message": {"ctrl+n"},
	})

	require.Empty(t, warnings)
}

func TestApplyKeybinds_ModalScopesSilent(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	// Dialog and completions keys are modal; overlapping globals and
	// editor keys is expected there.
	warnings := ApplyKeybinds(&km, map[string][]string{
		"dialog.close":       {"esc"},
		"completions.cancel": {"esc"},
		"completions.up":     {"ctrl+p"},
	})

	require.Empty(t, warnings)
}

func TestApplyKeybinds_NoConflictWithoutOverrides(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	// Defaults share keys across scopes by design; only user
	// overrides are checked.
	require.Empty(t, ApplyKeybinds(&km, nil))
}

func TestApplyKeybinds_EmptyDisables(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	warnings := ApplyKeybinds(&km, map[string][]string{
		"global.quit": {},
	})

	require.Empty(t, warnings)
	require.False(t, km.Quit.Enabled())
	require.True(t, km.Help.Enabled())
}

func TestClearSelection_DropsHighlights(t *testing.T) {
	t.Parallel()
	u := newTestUI()
	sty := u.com.Styles
	item := chat.NewShellItem(sty, "echo hi", "hi", 0)
	u.chat.AppendMessages(item)

	hl, ok := item.(list.Highlightable)
	require.True(t, ok, "shell item should be highlightable")
	hl.SetHighlight(0, 0, 0, 5)
	sl, sc, _, _ := hl.Highlight()
	require.Equal(t, 0, sl)
	require.Equal(t, 0, sc)

	u.chat.ClearSelection()

	sl, _, el, _ := hl.Highlight()
	require.Equal(t, -1, sl, "start line not cleared")
	require.Equal(t, -1, el, "end line not cleared")
	require.Equal(t, -1, u.chat.mouseDownItem)
}

func TestApplyKeybinds_DisablePropagatesToQuestions(t *testing.T) {
	t.Parallel()
	com := &common.Common{Workspace: &testWorkspace{cfg: &config.Config{
		Keybinds: map[string][]string{"dialog.select": {}},
	}}}
	km := DefaultKeyMap()
	ta := textarea.New()
	comp := completions.New(
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
	)

	applyUserKeybinds(com, &km, &ta, comp)

	require.False(t, dialog.QuestionSelect.Enabled(), "question select not disabled")
	require.False(t, dialog.QuestionDone.Enabled(), "question done not disabled")
	require.False(t, dialog.QuestionConfirm.Enabled(), "question confirm not disabled")
	require.False(t, dialog.QuestionSubmit.Enabled(), "question submit not disabled")
}
