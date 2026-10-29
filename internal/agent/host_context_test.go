package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
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

func TestNotesAreKeptUntilTheNextPrompt(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{notes: csync.NewMap[string, []string]()}
	a.AddNote("s1", "")
	require.Empty(t, a.takeNotes("s1"))

	a.AddNote("s1", "first")
	a.AddNote("s1", "second")
	a.AddNote("s2", "other")
	require.Empty(t, notesPrompt(nil))
	require.Equal(t, []string{"first", "second"}, a.takeNotes("s1"))
	require.Empty(t, a.takeNotes("s1"), "notes are given once")
	require.Equal(t, []string{"other"}, a.takeNotes("s2"))
	require.Contains(t, notesPrompt([]string{"first", "second"}), "first\n\nsecond\n</user_notes>")
}
