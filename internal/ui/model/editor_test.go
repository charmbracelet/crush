package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func TestEditorFileMsg(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		content string
		want    string
	}{
		{"emptied buffer reports empty text", "", ""},
		{"whitespace-only buffer reports empty text", "\n \n\t\n", ""},
		{"edited text keeps inner newlines", "line one\nline two\n", "line one\nline two"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "msg.md")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o644))

			msg, ok := editorFileMsg(path).(openEditorMsg)
			require.True(t, ok, "editor result must reach the composer even when empty")
			require.Equal(t, tt.want, msg.Text)
		})
	}
}

func TestEditorFileMsgUnreadableFile(t *testing.T) {
	t.Parallel()

	msg := editorFileMsg(filepath.Join(t.TempDir(), "missing.md"))
	_, isEditorMsg := msg.(openEditorMsg)
	require.False(t, isEditorMsg, "a failed read must not overwrite the composer")
}

func TestOpenEditorMsgClearsComposer(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	u.textarea.SetValue("a message typed before opening the editor")

	updated, _ := u.Update(openEditorMsg{Text: ""})
	ui, ok := updated.(*UI)
	require.True(t, ok)
	require.Empty(t, ui.textarea.Value())
}

func TestOpenEditorMsgReplacesComposer(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	u.textarea.SetValue("original")

	updated, _ := u.Update(openEditorMsg{Text: "edited in vim"})
	ui, ok := updated.(*UI)
	require.True(t, ok)
	require.Equal(t, "edited in vim", ui.textarea.Value())
}
