package model

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
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

type todoBusyWorkspace struct {
	workspace.Workspace
	busySessions map[string]bool
}

func (w *todoBusyWorkspace) AgentIsReady() bool {
	return true
}

func (w *todoBusyWorkspace) AgentIsBusy() bool {
	return true
}

func (w *todoBusyWorkspace) AgentIsSessionBusy(sessionID string) bool {
	return w.busySessions[sessionID]
}

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

func TestAutoExpandPillsIfReasonable(t *testing.T) {
	t.Parallel()

	t.Run("expands when terminal is tall enough and todos exist", func(t *testing.T) {
		t.Parallel()

		u := newTestUI()
		u.height = 50
		u.todoActive = true
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
		u.todoActive = true
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
		u.todoActive = true
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
		u.todoActive = true
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

func TestTodoActivityFollowsTurnLifecycle(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Status: session.TodoStatusInProgress, Content: "do work"},
		{Status: session.TodoStatusPending, Content: "do more"},
	}}

	u.noteTodoUpdate(nil, true)
	require.True(t, u.hasActiveTodos())
	require.Equal(t, pillHeightWithBorder, u.pillsAreaHeight())

	u.finishTodoTurn("other-session", false)
	require.True(t, u.hasActiveTodos())

	wantTodos := slices.Clone(u.session.Todos)
	u.finishTodoTurn("s1", false)
	require.False(t, u.hasActiveTodos())
	require.Zero(t, u.pillsAreaHeight())
	require.Equal(t, wantTodos, u.session.Todos)
}

func TestTodoActivityStartsForRepeatedToolSnapshot(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Status: session.TodoStatusInProgress, Content: "do work"},
	}}
	toolMessage := message.Message{}
	toolMessage.AddToolCall(message.ToolCall{Name: tools.TodosToolName, Finished: true})

	u.noteTodoMessage(toolMessage, true)
	require.True(t, u.hasActiveTodos())

	u.finishTodoTurn("s1", false)
	u.noteTodoMessage(toolMessage, false)
	require.False(t, u.hasActiveTodos())
}

func TestTodoTerminalEventDoesNotCloseNewBusyTurn(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Status: session.TodoStatusInProgress, Content: "do work"},
	}}
	u.todoActive = true

	u.finishTodoTurn("s1", true)
	require.True(t, u.hasActiveTodos())

	u.finishTodoTurn("s1", false)
	require.False(t, u.hasActiveTodos())
}

func TestRestoreTodoActivityForBusySession(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Status: session.TodoStatusInProgress, Content: "do work"},
	}}

	u.restoreTodoActivity(true)
	require.True(t, u.hasActiveTodos())

	u.restoreTodoActivity(false)
	require.False(t, u.hasActiveTodos())
}

func TestTodoTurnCompletionFocusesRemainingQueue(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Status: session.TodoStatusInProgress, Content: "do work"},
	}}
	u.todoActive = true
	u.promptQueue = 1
	u.pillsExpanded = true
	u.focusedPillSection = pillSectionTodos

	u.finishTodoTurn("s1", false)
	require.False(t, u.hasActiveTodos())
	require.True(t, u.pillsExpanded)
	require.Equal(t, pillSectionQueue, u.focusedPillSection)
}

func TestTodoBusyStateIsScopedToSession(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = &todoBusyWorkspace{busySessions: map[string]bool{
		"busy-session": true,
	}}

	require.True(t, u.isTodoSessionBusy("busy-session"))
	require.False(t, u.isTodoSessionBusy("idle-session"))
}

func TestNormalizePillFocusWhenQueueDisappears(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Status: session.TodoStatusInProgress, Content: "do work"},
		{Status: session.TodoStatusPending, Content: "do more"},
	}}
	u.todoActive = true
	u.pillsExpanded = true
	u.focusedPillSection = pillSectionQueue

	u.updateLayoutAndSize()
	require.Equal(t, pillSectionTodos, u.focusedPillSection)
	require.Equal(t, u.pillsAreaHeight(), u.layout.pills.Dy())
}
