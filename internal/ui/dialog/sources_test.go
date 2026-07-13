package dialog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestSources() *Sources {
	sty := styles.CharmtonePantera()
	return NewSources(&common.Common{Styles: &sty}, []session.Source{{
		ID:       "source-1",
		Kind:     session.SourceKindFile,
		Label:    "notes.md",
		Location: `C:\project\notes.md`,
	}})
}

func TestSourcesActions(t *testing.T) {
	t.Parallel()

	t.Run("add", func(t *testing.T) {
		action, ok := newTestSources().HandleMsg(keyMsg('a')).(ActionOpenSourceAdd)
		require.True(t, ok)
		require.Equal(t, ActionOpenSourceAdd{}, action)
	})

	t.Run("view", func(t *testing.T) {
		action, ok := newTestSources().HandleMsg(keyMsg('v')).(ActionSourceView)
		require.True(t, ok)
		require.Equal(t, "source-1", action.Source.ID)
	})
}

func TestSourcesRemoveRequiresConfirmation(t *testing.T) {
	t.Parallel()

	dialog := newTestSources()
	require.Nil(t, dialog.HandleMsg(keyMsg('x')))
	require.True(t, dialog.confirmRemove)

	help := dialog.ShortHelp()
	require.Len(t, help, 2)
	require.Equal(t, "confirm", help[0].Help().Desc)
	require.Equal(t, "cancel", help[1].Help().Desc)

	action, ok := dialog.HandleMsg(keyMsg('y')).(ActionSourceRemove)
	require.True(t, ok)
	require.Equal(t, "source-1", action.ID)
	require.False(t, dialog.confirmRemove)
}

func TestSourcePreviewReadsTextFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(path, []byte("source preview body"), 0o600))

	preview := sourcePreviewText(session.Source{
		Kind:     session.SourceKindFile,
		Location: path,
	})
	require.Equal(t, "source preview body", preview)
}

func TestSourcePreviewLeavesURLLazy(t *testing.T) {
	t.Parallel()

	preview := sourcePreviewText(session.Source{
		Kind:     session.SourceKindURL,
		Location: "https://example.com/docs",
	})
	require.Contains(t, preview, "remain lazy")
}
