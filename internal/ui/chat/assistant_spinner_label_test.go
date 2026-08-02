package chat

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// spinningAssistantItem builds an assistant item in the spinning state:
// not thinking, not finished, no content and no tool calls.
func spinningAssistantItem(t *testing.T) *AssistantMessageItem {
	t.Helper()
	sty := styles.CharmtonePantera()
	msg := &message.Message{ID: "spin-1", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	require.True(t, item.isSpinning(), "test fixture must be in the spinning state")
	return item
}

// TestRenderSpinningDoesNotRebuildLabelEveryFrame guards the render hot
// path. renderSpinning runs on every animation frame (20fps) and the
// item is deliberately uncached while spinning, so an unconditional
// anim.SetLabel re-renders the label and the ellipsis frames rune by
// rune through lipgloss on every frame. Measured at 104 allocs and
// 1669 B per SetLabel call, that is ~2000 allocs/second of pure garbage
// for the whole pre-first-token spinner window and for the entire
// duration of a retry countdown.
func TestRenderSpinningDoesNotRebuildLabelEveryFrame(t *testing.T) {
	// No t.Parallel: testing.AllocsPerRun panics in a parallel test.
	item := spinningAssistantItem(t)

	// Warm up: the first frame legitimately installs the label.
	item.renderSpinning()

	steady := testing.AllocsPerRun(200, func() {
		item.renderSpinning()
	})
	t.Logf("steady-state renderSpinning allocs/frame = %.0f", steady)

	// anim.Render itself allocates a handful; the label rebuild alone is
	// ~104. Anything near that means the label is being rebuilt per frame.
	require.Less(t, steady, float64(60),
		"renderSpinning rebuilds the spinner label on every frame (%.0f allocs/frame)", steady)
}

// TestRenderSpinningAppliesLabelChanges makes sure the caching above is
// not achieved by simply never updating the label: a retry countdown
// changes its text every second and each new value must reach anim.
func TestRenderSpinningAppliesLabelChanges(t *testing.T) {
	t.Parallel()

	item := spinningAssistantItem(t)

	item.renderSpinning()
	require.Equal(t, "Working", item.appliedLabel)

	item.SetWorkingLabel("Retrying in 5s")
	item.renderSpinning()
	require.Equal(t, "Retrying in 5s", item.appliedLabel)

	// A second, different countdown value must be pushed through too.
	item.SetWorkingLabel("Retrying in 4s")
	item.renderSpinning()
	require.Equal(t, "Retrying in 4s", item.appliedLabel)

	// Clearing restores the default label.
	item.SetWorkingLabel("")
	item.renderSpinning()
	require.Equal(t, "Working", item.appliedLabel)
}

func TestRenderSpinningHidesSuffixTimerWhenWorkingLabelIsSet(t *testing.T) {
	t.Parallel()

	item := spinningAssistantItem(t)
	item.SetWorkingLabel("Retrying in 5s")
	item.renderSpinning()
	require.Equal(t, "Retrying in 5s", item.appliedLabel)
}
