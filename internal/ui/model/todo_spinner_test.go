package model

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestTodoSpinner_StopsOnSessionUpdate(t *testing.T) {
	t.Parallel()

	t.Run("stops when all todos become completed", func(t *testing.T) {
		t.Parallel()

		u := newTodoSpinnerTestUI(t)
		u.session.Todos = []session.Todo{
			{Content: "Task 1", Status: session.TodoStatusInProgress},
			{Content: "Task 2", Status: session.TodoStatusPending},
		}
		u.todoIsSpinning = true
		u.todoSpinner = spinner.New(spinner.WithSpinner(spinner.MiniDot))

		_, cmd := u.Update(pubsub.Event[session.Session]{
			Type: pubsub.UpdatedEvent,
			Payload: session.Session{
				ID: "test-session",
				Todos: []session.Todo{
					{Content: "Task 1", Status: session.TodoStatusCompleted},
					{Content: "Task 2", Status: session.TodoStatusCompleted},
				},
			},
		})
		require.Nil(t, cmd)
		require.False(t, u.todoIsSpinning)
	})

	t.Run("stays spinning when in-progress todo remains", func(t *testing.T) {
		t.Parallel()

		u := newTodoSpinnerTestUI(t)
		u.session.Todos = []session.Todo{
			{Content: "Task 1", Status: session.TodoStatusInProgress},
		}
		u.todoIsSpinning = true
		u.todoSpinner = spinner.New(spinner.WithSpinner(spinner.MiniDot))

		_, cmd := u.Update(pubsub.Event[session.Session]{
			Type: pubsub.UpdatedEvent,
			Payload: session.Session{
				ID: "test-session",
				Todos: []session.Todo{
					{Content: "Task 1", Status: session.TodoStatusCompleted},
					{Content: "Task 2", Status: session.TodoStatusInProgress},
				},
			},
		})
		require.Nil(t, cmd)
		require.True(t, u.todoIsSpinning)
	})

	t.Run("starts spinning when new in-progress todo appears", func(t *testing.T) {
		t.Parallel()

		u := newTodoSpinnerTestUI(t)
		u.session.Todos = []session.Todo{
			{Content: "Task 1", Status: session.TodoStatusPending},
		}
		u.todoSpinner = spinner.New(spinner.WithSpinner(spinner.MiniDot))
		require.False(t, u.todoIsSpinning)

		_, cmd := u.Update(pubsub.Event[session.Session]{
			Type: pubsub.UpdatedEvent,
			Payload: session.Session{
				ID: "test-session",
				Todos: []session.Todo{
					{Content: "Task 1", Status: session.TodoStatusInProgress},
					{Content: "Task 2", Status: session.TodoStatusPending},
				},
			},
		})
		require.NotNil(t, cmd)
		require.True(t, u.todoIsSpinning)
	})

	t.Run("no-ops for different session", func(t *testing.T) {
		t.Parallel()

		u := newTodoSpinnerTestUI(t)
		u.session.Todos = []session.Todo{
			{Content: "Task 1", Status: session.TodoStatusInProgress},
		}
		u.todoIsSpinning = true
		u.todoSpinner = spinner.New(spinner.WithSpinner(spinner.MiniDot))

		_, _ = u.Update(pubsub.Event[session.Session]{
			Type: pubsub.UpdatedEvent,
			Payload: session.Session{
				ID: "other-session",
				Todos: []session.Todo{
					{Content: "Task 1", Status: session.TodoStatusCompleted},
				},
			},
		})
		require.True(t, u.todoIsSpinning)
	})
}

func TestTodoSpinner_StopsOnTickWhenAllCompleted(t *testing.T) {
	t.Parallel()

	t.Run("stops spinning on tick when no in-progress todos", func(t *testing.T) {
		t.Parallel()

		u := newTodoSpinnerTestUI(t)
		u.todoIsSpinning = true
		u.todoSpinner = spinner.New(spinner.WithSpinner(spinner.MiniDot))

		u.session.Todos = []session.Todo{
			{Content: "Task 1", Status: session.TodoStatusCompleted},
			{Content: "Task 2", Status: session.TodoStatusCompleted},
		}

		tickMsg := spinner.TickMsg{}
		_, _ = u.Update(tickMsg)
		require.False(t, u.todoIsSpinning)
	})

	t.Run("keeps spinning on tick when in-progress todo exists", func(t *testing.T) {
		t.Parallel()

		u := newTodoSpinnerTestUI(t)
		u.todoIsSpinning = true
		u.todoSpinner = spinner.New(spinner.WithSpinner(spinner.MiniDot))

		u.session.Todos = []session.Todo{
			{Content: "Task 1", Status: session.TodoStatusCompleted},
			{Content: "Task 2", Status: session.TodoStatusInProgress},
		}

		tickMsg := spinner.TickMsg{}
		_, _ = u.Update(tickMsg)
		require.True(t, u.todoIsSpinning)
	})
}

// todoSpinnerTestWorkspace is a minimal workspace stub for todo spinner tests.
type todoSpinnerTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *todoSpinnerTestWorkspace) Config() *config.Config               { return w.cfg }
func (w *todoSpinnerTestWorkspace) AgentIsReady() bool                    { return false }
func (w *todoSpinnerTestWorkspace) AgentIsBusy() bool                     { return false }
func (w *todoSpinnerTestWorkspace) AgentQueuedPrompts(string) int         { return 0 }
func (w *todoSpinnerTestWorkspace) AgentQueuedPromptsList(string) []string { return nil }

func newTodoSpinnerTestUI(t *testing.T) *UI {
	t.Helper()

	ws := &todoSpinnerTestWorkspace{cfg: &config.Config{}}
	com := common.DefaultCommon(ws)

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
		state:    uiChat,
		chat:     NewChat(com),
		status:   NewStatus(com, nil),
		dialog:   dialog.NewOverlay(),
		textarea: ta,
		width:    140,
		height:   45,
	}
	u.session = &session.Session{
		ID:    "test-session",
		Todos: []session.Todo{},
	}
	return u
}
