package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// fakeClipboardText returns a clipboard reader stub with the given content.
func fakeClipboardText(content string) func() ([]byte, error) {
	return func() ([]byte, error) { return []byte(content), nil }
}

// rightClick builds a right-button mouse click at the given position.
func rightClick(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: uv.MouseRight})
}

// newPasteTestUI builds a chat UI with a computed layout and a stubbed
// clipboard reader so tests never touch the real clipboard.
func newPasteTestUI(t *testing.T, clipboardText string) *UI {
	t.Helper()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	sty := u.com.Styles.Attachments
	u.attachments = attachments.New(
		attachments.NewRenderer(sty.Normal, sty.Deleting, sty.Image, sty.Text, sty.Skill, sty.Remove),
		attachments.Keymap{},
	)
	u.readClipboardText = fakeClipboardText(clipboardText)
	u.updateLayoutAndSize()
	return u
}

// runMsg feeds a command result back through Update like Bubble Tea would.
func runMsg(u *UI, msg tea.Msg) tea.Cmd {
	_, cmd := u.Update(msg)
	return cmd
}

func TestRightClickInEditorSchedulesPasteAndFocuses(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")
	// Simulate focus elsewhere (e.g. chat output).
	u.focus = uiFocusMain
	u.textarea.Blur()

	x := u.layout.editor.Min.X + 1
	y := u.layout.editor.Min.Y + 1 // below the attachments row

	_, cmd := u.Update(rightClick(x, y))
	require.NotNil(t, cmd, "right-click in the editor must schedule the clipboard read")
	require.Equal(t, uiFocusEditor, u.focus)
	require.True(t, u.textarea.Focused(), "the textarea must be focused by the click")

	// The command must read the clipboard and produce a tea.PasteMsg.
	msg := cmd()
	require.IsType(t, tea.PasteMsg{}, msg)
	require.Equal(t, "hello", msg.(tea.PasteMsg).Content)
}

func TestRightClickPasteWorksOnLandingScreen(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")
	u.state = uiLanding

	_, cmd := u.Update(rightClick(u.layout.editor.Min.X+1, u.layout.editor.Min.Y+1))
	require.NotNil(t, cmd, "right-click paste must also work on the landing screen")

	runMsg(u, cmd())
	require.Equal(t, "hello", u.textarea.Value())
}

func TestRightClickPasteFlowsThroughPasteMsgPath(t *testing.T) {
	t.Parallel()

	// CRLF must be normalized by handlePasteMsg, proving the clipboard
	// content travels through tea.PasteMsg handling rather than being
	// inserted directly into the textarea.
	u := newPasteTestUI(t, "one\r\ntwo")

	_, cmd := u.Update(rightClick(u.layout.editor.Min.X+1, u.layout.editor.Min.Y+1))
	require.NotNil(t, cmd)

	runMsg(u, cmd())
	require.Equal(t, "one\ntwo", u.textarea.Value())
}

func TestRightClickInChatDoesNotPaste(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")

	_, cmd := u.Update(rightClick(u.layout.main.Min.X+1, u.layout.main.Min.Y+1))
	require.Nil(t, cmd, "right-click in chat output must not schedule a paste")
	require.Empty(t, u.textarea.Value())
}

func TestRightClickInSidebarDoesNotPaste(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")

	_, cmd := u.Update(rightClick(u.layout.sidebar.Min.X+1, u.layout.sidebar.Min.Y+1))
	require.Nil(t, cmd, "right-click in the sidebar must not schedule a paste")
	require.Empty(t, u.textarea.Value())
}

func TestRightClickOnAttachmentRemoveButtonDoesNotRemove(t *testing.T) {
	t.Parallel()

	u, removeX := newAttachmentClickTestUI(t)
	u.readClipboardText = fakeClipboardText("hello")

	_, cmd := u.Update(rightClick(u.layout.editor.Min.X+removeX, u.layout.editor.Min.Y))

	require.Len(t, u.attachments.List(), 1, "right-click must not remove attachments")
	if cmd != nil {
		_, isPaste := cmd().(tea.PasteMsg)
		require.False(t, isPaste, "right-click on an attachment chip must not paste")
	}
}

func TestLeftClickInEditorIsNotPaste(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")

	x := u.layout.editor.Min.X + 1
	y := u.layout.editor.Min.Y + 1

	_, cmd := u.Update(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: uv.MouseLeft}))
	if cmd != nil {
		_, isPaste := cmd().(tea.PasteMsg)
		require.False(t, isPaste, "left-click must not produce a paste")
	}
	require.Empty(t, u.textarea.Value())
	require.True(t, u.textareaMouseSelecting, "left-click still starts a selection gesture")
}

// stubPasteableInline is a minimal inline editor that accepts paste events.
type stubPasteableInline struct {
	dialog.InlineEditor
	focused bool
	pasted  []tea.PasteMsg
}

func (s *stubPasteableInline) SetFocused(focused bool) { s.focused = focused }
func (s *stubPasteableInline) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	s.pasted = append(s.pasted, msg)
	return nil
}

var _ dialog.PasteableEditor = (*stubPasteableInline)(nil)

// stubPlainInline is a minimal inline editor without paste support.
type stubPlainInline struct {
	dialog.InlineEditor
	focused bool
}

func (s *stubPlainInline) SetFocused(focused bool) { s.focused = focused }

func TestRightClickPasteIntoPasteableInlineEditor(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "inline")
	inline := &stubPasteableInline{}
	u.activeInline = inline
	u.textarea.Blur() // production blurs the textarea while an inline editor is active

	_, cmd := u.Update(rightClick(u.layout.editor.Min.X+1, u.layout.editor.Min.Y+1))
	require.NotNil(t, cmd)
	require.True(t, inline.focused, "the inline editor must be focused by the click")
	require.False(t, u.textarea.Focused(), "the textarea must not steal focus from the inline editor")

	runMsg(u, cmd())
	require.Len(t, inline.pasted, 1)
	require.Equal(t, "inline", inline.pasted[0].Content)
	require.Empty(t, u.textarea.Value(), "the textarea must not receive the paste")
}

func TestRightClickNonPasteableInlineEditorDoesNotPaste(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")
	inline := &stubPlainInline{}
	u.activeInline = inline

	_, cmd := u.Update(rightClick(u.layout.editor.Min.X+1, u.layout.editor.Min.Y+1))
	require.Nil(t, cmd, "a non-pasteable inline editor must not receive text")
	require.False(t, inline.focused, "a non-pasteable inline editor must not be focused by the click")
	require.Empty(t, u.textarea.Value())
}

// pasteDialog is a dialog that records tea.PasteMsg deliveries.
type pasteDialog struct {
	pasted []tea.PasteMsg
}

func (*pasteDialog) ID() string { return "paste-dialog" }
func (d *pasteDialog) HandleMsg(msg tea.Msg) dialog.Action {
	if pm, ok := msg.(tea.PasteMsg); ok {
		d.pasted = append(d.pasted, pm)
	}
	return nil
}
func (*pasteDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

var _ dialog.Dialog = (*pasteDialog)(nil)

func TestRightClickWithDialogKeepsDialogRouting(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "hello")
	d := &pasteDialog{}
	u.dialog.OpenDialog(d)

	// Even over the editor area, an open dialog keeps event priority and
	// the click is forwarded to it, not turned into a paste.
	_, _ = u.Update(rightClick(u.layout.editor.Min.X+1, u.layout.editor.Min.Y+1))
	require.Empty(t, d.pasted)
	require.Empty(t, u.textarea.Value())

	// The dialog still receives pastes through the ordinary path.
	runMsg(u, tea.PasteMsg{Content: "via dialog"})
	require.Len(t, d.pasted, 1)
	require.Equal(t, "via dialog", d.pasted[0].Content)
}

func TestRightClickWithEmptyClipboardReportsError(t *testing.T) {
	t.Parallel()

	u := newPasteTestUI(t, "")

	_, cmd := u.Update(rightClick(u.layout.editor.Min.X+1, u.layout.editor.Min.Y+1))
	require.NotNil(t, cmd)

	msg := cmd()
	require.IsType(t, util.InfoMsg{}, msg, "an empty clipboard must surface the existing error feedback")
	require.Empty(t, u.textarea.Value())
}
