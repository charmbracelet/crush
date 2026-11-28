package model

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// newTodoSpinnerTestUI builds a chat UI whose only animation is the
// in-progress todo in the pills. Its clock reads the returned time.
func newTodoSpinnerTestUI(t *testing.T) (*UI, *time.Time) {
	t.Helper()
	u := newFrameTestUI(t)
	u.com.Workspace = &testWorkspace{cfg: &config.Config{Options: &config.Options{}}}
	u.session = &session.Session{
		ID: "s1",
		Todos: []session.Todo{
			{Content: "first", Status: session.TodoStatusCompleted},
			{Content: "second", ActiveForm: "Doing second", Status: session.TodoStatusInProgress},
		},
	}
	u.todoIsSpinning = true
	now := time.Unix(1000, 0)
	u.animClock.now = func() time.Time { return now }
	u.updateLayoutAndSize()
	require.Positive(t, u.layout.pills.Dx(), "the pills must be laid out")
	return u, &now
}

func TestTodoSpinner_ArmsTheSharedClock(t *testing.T) {
	t.Parallel()
	u, _ := newTodoSpinnerTestUI(t)
	require.False(t, u.chat.hasVisibleAnimation())

	_, cmd := u.Update(neutralMsg{})
	require.NotNil(t, cmd)
	require.True(t, u.animClock.running, "a spinning todo pill must arm the clock")
}

func TestTodoSpinner_KeepsSpinningBehindADialog(t *testing.T) {
	t.Parallel()
	u, _ := newTodoSpinnerTestUI(t)
	u.dialog.OpenDialog(dialog.NewQuit(u.com, 0))
	require.True(t, u.dialog.HasDialogs())
	require.True(t, u.todoSpinnerVisible(), "the pills stay visible behind a dialog")

	_, cmd := u.Update(neutralMsg{})
	require.NotNil(t, cmd)
	require.True(t, u.animClock.running, "a todo pill behind a dialog must keep the clock running")
}

func TestTodoSpinner_HiddenPillsDoNotArmTheClock(t *testing.T) {
	t.Parallel()
	u, _ := newTodoSpinnerTestUI(t)
	u.todoIsSpinning = false
	_, _ = u.Update(neutralMsg{})
	require.False(t, u.animClock.running, "a still todo pill needs no clock")

	u.todoIsSpinning = true
	u.activeInline = dialog.NewQuestionForm(u.com.Styles, question.Request{})
	require.False(t, u.todoSpinnerVisible(), "an inline editor hides the pills")
}

func TestTodoSpinner_TickRedrawsOnlyWhenTheGlyphChanges(t *testing.T) {
	t.Parallel()
	u, now := newTodoSpinnerTestUI(t)
	_, _ = u.Update(neutralMsg{})
	first := u.View()
	require.True(t, strings.Contains(ansi.Strip(first.Content), "Doing second"))

	// Within the same glyph period the tick reuses the frame.
	_, _ = u.Update(animTickMsg{gen: u.animClock.gen})
	u.View()
	require.Equal(t, 1, u.frames.hits, "a tick that moved nothing must hit the cache")
	require.True(t, u.animClock.running, "the tick must re-arm the clock")

	// Once the glyph changes the tick must render a new frame.
	*now = now.Add(2 * anim.FrameInterval())
	_, _ = u.Update(animTickMsg{gen: u.animClock.gen})
	second := u.View()
	require.Equal(t, 1, u.frames.hits, "a moved todo glyph must not be served from cache")
	require.NotEqual(t, first.Content, second.Content)
	require.Equal(t, spinnerFrameIndex(*now), u.lastTodoSpinnerFrame)
}
