package model

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
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
func (m testMessageItem) Version() uint64      { return 0 }
func (m testMessageItem) Finished() bool       { return true }

var _ chat.MessageItem = testMessageItem{}

// mutableMessageItem is a test message item whose rendered height can grow
// over time, simulating streaming content that increases the item's height.
type mutableMessageItem struct {
	id      string
	lines   int
	version uint64
}

func (m *mutableMessageItem) ID() string { return m.id }
func (m *mutableMessageItem) Render(width int) string {
	lines := make([]string, m.lines)
	for i := range lines {
		lines[i] = "line"
	}
	return strings.Join(lines, "\n")
}
func (m *mutableMessageItem) RawRender(width int) string { return m.Render(width) }
func (m *mutableMessageItem) Version() uint64            { return m.version }
func (m *mutableMessageItem) Finished() bool             { return false }

var _ chat.MessageItem = (*mutableMessageItem)(nil)

// newTestUI builds a focused uiChat model with dynamic textarea sizing enabled.
// It intentionally keeps dependencies minimal so layout behavior can be tested
// in isolation.
func newTestUI() *UI {
	com := common.DefaultCommon(nil)

	ta := textarea.New()
	ta.SetStyles(com.Styles.Editor.Textarea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = TextareaMinHeight
	ta.MaxHeight = TextareaMaxHeight
	ta.Focus()

	u := &UI{
		com:      com,
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
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

func TestScrollByDown_EnablesFollowAtBottom(t *testing.T) {
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

	// Start at the top with follow disabled, simulating a user that
	// scrolled up to read earlier messages.
	u.chat.ScrollToTop()
	if u.chat.Follow() {
		t.Fatal("expected follow mode to be disabled after scrolling to top")
	}

	// Scroll down in small increments (like mouse wheel ticks) until we
	// reach the bottom. Follow mode should re-enable once the bottom is
	// reached so the view sticks to new content.
	for range 200 {
		if u.chat.AtBottom() {
			break
		}
		u.chat.ScrollBy(3)
	}

	if !u.chat.AtBottom() {
		t.Fatal("expected chat to be at bottom after scrolling down")
	}
	if !u.chat.Follow() {
		t.Fatal("expected follow mode to be enabled after scrolling to bottom")
	}
}

func TestFollowStaysAtBottomWhenContentGrows(t *testing.T) {
	t.Parallel()

	u := newTestUI()

	// Create enough static messages to make the chat scrollable, plus one
	// mutable item at the end that will grow (simulating streaming).
	msgs := make([]chat.MessageItem, 0, 60)
	for i := range 59 {
		msgs = append(msgs, testMessageItem{
			id:   "m-" + strconv.Itoa(i),
			text: "message " + strconv.Itoa(i),
		})
	}
	streaming := &mutableMessageItem{id: "streaming", lines: 1, version: 1}
	msgs = append(msgs, streaming)
	u.chat.SetMessages(msgs...)
	u.updateLayoutAndSize()

	// Start at the bottom in follow mode.
	u.chat.ScrollToBottom()
	if !u.chat.AtBottom() {
		t.Fatal("expected chat to start at bottom")
	}

	// Simulate streaming: grow the last item's height.
	streaming.lines = 20
	streaming.version++

	// Trigger a draw (which renders the list and updates cached heights).
	// The follow re-anchor in Draw should keep us pinned to the bottom.
	scr := uv.NewScreenBuffer(u.width, u.height)
	u.chat.Draw(scr, u.layout.main)

	if !u.chat.AtBottom() {
		t.Fatal("expected chat to remain at bottom after streaming content grew while following")
	}
}

func TestAutoExpandPillsIfReasonable(t *testing.T) {
	t.Parallel()

	t.Run("expands when terminal is tall enough and todos exist", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 50
		u.session = &session.Session{ID: "s1", Todos: []session.Todo{
			{Status: session.TodoStatusInProgress, Content: "do work"},
			{Status: session.TodoStatusPending, Content: "do more"},
		}}

		u.autoExpandPillsIfReasonable()

		if !u.pillsExpanded {
			t.Fatal("expected pillsExpanded to be true")
		}
		if u.focusedPillSection != pillSectionTodos {
			t.Fatalf("expected focusedPillSection to be pillSectionTodos, got %d", u.focusedPillSection)
		}
	})

	t.Run("does not expand when terminal is too short", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 30
		u.session = &session.Session{ID: "s1", Todos: []session.Todo{
			{Status: session.TodoStatusInProgress, Content: "do work"},
		}}

		u.autoExpandPillsIfReasonable()

		if u.pillsExpanded {
			t.Fatal("expected pillsExpanded to be false when terminal height is below threshold")
		}
	})

	t.Run("does not expand when all todos are completed", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 50
		u.session = &session.Session{ID: "s1", Todos: []session.Todo{
			{Status: session.TodoStatusCompleted, Content: "done"},
		}}

		u.autoExpandPillsIfReasonable()

		if u.pillsExpanded {
			t.Fatal("expected pillsExpanded to be false when all todos are completed")
		}
	})

	t.Run("does not expand when already expanded", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 50
		u.pillsExpanded = true
		u.session = &session.Session{ID: "s1", Todos: []session.Todo{
			{Status: session.TodoStatusInProgress, Content: "do work"},
		}}
		u.updateLayoutAndSize()

		u.autoExpandPillsIfReasonable()

		if !u.pillsExpanded {
			t.Fatal("expected pillsExpanded to stay true")
		}
	})

	t.Run("expands for prompt queue when no todos", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 50
		u.session = &session.Session{ID: "s1", Todos: []session.Todo{}}
		u.promptQueue = 2

		u.autoExpandPillsIfReasonable()

		if !u.pillsExpanded {
			t.Fatal("expected pillsExpanded to be true for prompt queue")
		}
		if u.focusedPillSection != pillSectionQueue {
			t.Fatalf("expected focusedPillSection to be pillSectionQueue, got %d", u.focusedPillSection)
		}
	})

	t.Run("does not expand when no session", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 50
		u.session = nil

		u.autoExpandPillsIfReasonable()

		if u.pillsExpanded {
			t.Fatal("expected pillsExpanded to be false when there is no session")
		}
	})
}
