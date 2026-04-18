package model

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// testMessageItem is a minimal chat item used to populate the chat list
// without pulling in full message rendering machinery.
type testMessageItem struct {
	id   string
	text string
}

func (m testMessageItem) ID() string           { return m.id }
func (m testMessageItem) Render(int) string    { return m.text }
func (m testMessageItem) RawRender(int) string { return m.text }

var _ chat.MessageItem = testMessageItem{}

// newTestUI builds a focused uiChat model with dynamic textarea sizing enabled.
// It intentionally keeps dependencies minimal so layout behavior can be tested
// in isolation.
func newTestUI() *UI {
	com := common.DefaultCommon(nil)
	keyMap := DefaultKeyMap()

	ta := textarea.New()
	ta.SetStyles(com.Styles.TextArea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = TextareaMinHeight
	ta.MaxHeight = TextareaMaxHeight
	configureTextareaKeyMap(&ta)
	ta.Focus()

	u := &UI{
		com:    com,
		dialog: dialog.NewOverlay(),
		status: NewStatus(com, nil),
		chat:   NewChat(com),
		completions: completions.New(
			com.Styles.Completions.Normal,
			com.Styles.Completions.Focused,
			com.Styles.Completions.Match,
		),
		attachments: attachments.New(
			attachments.NewRenderer(
				com.Styles.Attachments.Normal,
				com.Styles.Attachments.Deleting,
				com.Styles.Attachments.Image,
				com.Styles.Attachments.Text,
			),
			attachments.Keymap{
				DeleteMode: keyMap.Editor.AttachmentDeleteMode,
				DeleteAll:  keyMap.Editor.DeleteAllAttachments,
				Escape:     keyMap.Editor.Escape,
			},
		),
		keyMap:   keyMap,
		textarea: ta,
		state:    uiChat,
		focus:    uiFocusEditor,
		width:    140,
		height:   45,
	}

	return u
}

func TestUpdateLayoutAndSize_EditorGrowthShrinksChat(t *testing.T) {
	t.Parallel()

	// Baseline layout at min textarea height.
	u := newTestUI()
	u.updateLayoutAndSize()

	initialEditorHeight := u.layout.editor.Dy()
	initialChatHeight := u.layout.main.Dy()

	// Increase textarea content enough to trigger growth, then run the
	// same resize hook used in the real update path.
	prevHeight := u.textarea.Height()
	u.textarea.SetValue(strings.Repeat("line\n", 8))
	u.textarea.MoveToEnd()
	_ = u.handleTextareaHeightChange(prevHeight)

	if got := u.layout.editor.Dy(); got <= initialEditorHeight {
		t.Fatalf("expected editor to grow: got %d, want > %d", got, initialEditorHeight)
	}

	if got := u.layout.main.Dy(); got >= initialChatHeight {
		t.Fatalf("expected chat to shrink: got %d, want < %d", got, initialChatHeight)
	}
}

func TestHandleTextareaHeightChange_FollowModeStaysAtBottom(t *testing.T) {
	t.Parallel()

	// Use enough messages to make the chat scrollable so AtBottom/Follow
	// assertions are meaningful.
	u := newTestUI()

	msgs := make([]chat.MessageItem, 0, 60)
	for i := range 60 {
		msgs = append(msgs, testMessageItem{
			id:   "m-" + strconv.Itoa(i),
			text: "message " + strconv.Itoa(i),
		})
	}
	u.chat.SetMessages(msgs...)
	u.updateLayoutAndSize()

	// Enter follow mode and verify we're anchored at the bottom first.
	u.chat.ScrollToBottom()
	if !u.chat.AtBottom() {
		t.Fatal("expected chat to start at bottom")
	}

	// Grow the editor; follow mode should keep the chat pinned to the end
	// even as the chat viewport shrinks.
	prevHeight := u.textarea.Height()
	u.textarea.SetValue(strings.Repeat("line\n", 10))
	u.textarea.MoveToEnd()
	_ = u.handleTextareaHeightChange(prevHeight)

	if !u.chat.Follow() {
		t.Fatal("expected follow mode to remain enabled")
	}
	if !u.chat.AtBottom() {
		t.Fatal("expected chat to remain at bottom after editor resize in follow mode")
	}
}

func TestMoveTextareaGroupAcrossWords(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("one two three")
	u.textarea.MoveToEnd()

	u.moveTextareaGroupBackward()
	if got := u.textarea.Column(); got != 8 {
		t.Fatalf("expected ctrl+left to move to the start of the previous word, got column %d", got)
	}

	u.moveTextareaGroupBackward()
	if got := u.textarea.Column(); got != 4 {
		t.Fatalf("expected second ctrl+left to move to the prior word, got column %d", got)
	}

	u.textarea.MoveToEnd()
	u.moveTextareaGroupBackward()
	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("one two three") {
		t.Fatalf("expected ctrl+right to move to the end of the current word, got column %d", got)
	}
}

func TestMoveTextareaGroupCamelHumps(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("parseHTTPResponse")
	u.textarea.MoveToBegin()

	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("parse") {
		t.Fatalf("expected first ctrl+right to stop at the end of the first hump, got %d", got)
	}

	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("parseHTTP") {
		t.Fatalf("expected second ctrl+right to stop after acronym hump, got %d", got)
	}

	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("parseHTTPResponse") {
		t.Fatalf("expected third ctrl+right to stop at the end of the identifier, got %d", got)
	}

	u.moveTextareaGroupBackward()
	if got := u.textarea.Column(); got != len("parseHTTP") {
		t.Fatalf("expected ctrl+left to return to the start of the Response hump, got %d", got)
	}

	u.moveTextareaGroupBackward()
	if got := u.textarea.Column(); got != len("parse") {
		t.Fatalf("expected ctrl+left to return to the start of the acronym hump, got %d", got)
	}
}

func TestMoveTextareaGroupSnakeCaseAndCamelCase(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("foo_barBaz")
	u.textarea.MoveToBegin()

	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("foo") {
		t.Fatalf("expected ctrl+right to stop at the end of the snake_case head, got %d", got)
	}

	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("foo_bar") {
		t.Fatalf("expected ctrl+right to skip underscore and stop at the end of the next hump, got %d", got)
	}

	u.moveTextareaGroupForward()
	if got := u.textarea.Column(); got != len("foo_barBaz") {
		t.Fatalf("expected ctrl+right to stop at the final hump, got %d", got)
	}

	u.moveTextareaGroupBackward()
	if got := u.textarea.Column(); got != len("foo_bar") {
		t.Fatalf("expected ctrl+left to return to the start of the Baz hump, got %d", got)
	}

	u.moveTextareaGroupBackward()
	if got := u.textarea.Column(); got != len("foo_") {
		t.Fatalf("expected ctrl+left at hump boundary to return to the previous snake_case segment, got %d", got)
	}
}

func TestDeleteTextareaGroupRemovesWords(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("one two three")
	u.textarea.MoveToBegin()

	_ = u.deleteTextareaGroupForward()
	if got := u.textarea.Value(); got != " two three" {
		t.Fatalf("expected ctrl+delete to remove the next word, got %q", got)
	}

	u.textarea.MoveToEnd()
	_ = u.deleteTextareaGroupBackward()
	if got := u.textarea.Value(); got != " two " {
		t.Fatalf("expected ctrl+backspace to remove the previous word, got %q", got)
	}
}

func TestDeleteTextareaGroupWhitespaceOnly(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("one   two")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("one"))

	_ = u.deleteTextareaGroupForward()
	if got := u.textarea.Value(); got != "onetwo" {
		t.Fatalf("expected forward delete in whitespace to remove only spaces, got %q", got)
	}

	u.textarea.SetValue("one   two")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("one   "))

	_ = u.deleteTextareaGroupBackward()
	if got := u.textarea.Value(); got != "onetwo" {
		t.Fatalf("expected backward delete in whitespace to remove only spaces, got %q", got)
	}
}

func TestDeleteTextareaGroupSymbolOnly(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("foo...bar")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("foo..."))

	_ = u.deleteTextareaGroupBackward()
	if got := u.textarea.Value(); got != "foobar" {
		t.Fatalf("expected backward delete after symbols to remove only the symbol group, got %q", got)
	}

	u.textarea.SetValue("foo...bar")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("foo"))

	_ = u.deleteTextareaGroupForward()
	if got := u.textarea.Value(); got != "foobar" {
		t.Fatalf("expected forward delete before symbols to remove only the symbol group, got %q", got)
	}
}

func TestDeleteTextareaGroupCamelHumps(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("parseHTTPResponse")
	u.textarea.MoveToEnd()

	_ = u.deleteTextareaGroupBackward()
	if got := u.textarea.Value(); got != "parseHTTP" {
		t.Fatalf("expected ctrl+backspace to remove only the Response hump, got %q", got)
	}

	u.textarea.SetValue("parseHTTPResponse")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("parse"))
	_ = u.deleteTextareaGroupForward()
	if got := u.textarea.Value(); got != "parseResponse" {
		t.Fatalf("expected ctrl+delete to remove only the acronym hump, got %q", got)
	}
}

func TestDeleteTextareaGroupSnakeCaseAndSeparators(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("foo_barBaz")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("foo"))

	_ = u.deleteTextareaGroupForward()
	if got := u.textarea.Value(); got != "fooBaz" {
		t.Fatalf("expected ctrl+delete at underscore boundary to remove the following subword and separator, got %q", got)
	}

	u.textarea.SetValue("foo_barBaz")
	u.textarea.MoveToBegin()
	u.textarea.SetCursorColumn(len("foo_bar"))
	_ = u.deleteTextareaGroupBackward()
	if got := u.textarea.Value(); got != "foo_Baz" {
		t.Fatalf("expected ctrl+backspace at hump boundary to remove only the previous subword, got %q", got)
	}
}

func TestHandleKeyPressMsgRoutesCtrlLeftThroughCamelHumps(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("parseHTTPResponse")
	u.textarea.MoveToEnd()

	_ = u.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl, Text: "ctrl+left"})
	if got := u.textarea.Column(); got != len("parseHTTP") {
		t.Fatalf("expected handleKeyPressMsg to route ctrl+left through CamelHumps movement, got %d", got)
	}
}

func TestHandleKeyPressMsgCtrlBackspaceUpdatesDraftAndCompletions(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("@fooBar")
	u.textarea.MoveToEnd()
	u.completionsOpen = true
	u.completionsStartIndex = 0
	u.completionsQuery = "fooBar"

	_ = u.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModCtrl, Text: "ctrl+backspace"})

	if got := u.textarea.Value(); got != "@foo" {
		t.Fatalf("expected ctrl+backspace to delete only the previous hump through handleKeyPressMsg, got %q", got)
	}
	if got := u.promptHistory.draft; got != "@foo" {
		t.Fatalf("expected ctrl+backspace to refresh prompt history draft, got %q", got)
	}
	if !u.completionsOpen {
		t.Fatal("expected completions to stay open for a remaining @-mention")
	}
	if got := u.completionsQuery; got != "foo" {
		t.Fatalf("expected completions query to be updated after ctrl+backspace, got %q", got)
	}
}
