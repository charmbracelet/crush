package dialog

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// stubDialog is a minimal Dialog for testing Overlay behavior.
type stubDialog struct {
	id       string
	received []tea.Msg
}

func (s *stubDialog) ID() string { return s.id }
func (s *stubDialog) HandleMsg(msg tea.Msg) Action {
	s.received = append(s.received, msg)
	return nil
}
func (s *stubDialog) Draw(_ uv.Screen, _ uv.Rectangle) *tea.Cursor { return nil }

func keyMsg(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// TestOverlay_GracePeriodSwallowsKeys verifies that all keystrokes
// arriving within the grace period after OpenDialogWithGrace are absorbed
// and never forwarded to the dialog.
func TestOverlay_GracePeriodSwallowsKeys(t *testing.T) {
	t.Parallel()

	d := &stubDialog{id: "test"}
	o := NewOverlay()
	o.OpenDialogWithGrace(d)

	for _, r := range []rune{'a', 's', 'd', 'x', 'z'} {
		o.Update(keyMsg(r))
	}
	require.Empty(t, d.received, "no keys should reach the dialog during grace period")
}

// TestOverlay_GracePeriodArmsAfterQuiet verifies that once input has been
// quiet for graceQuietPeriod, subsequent keys are forwarded normally.
func TestOverlay_GracePeriodArmsAfterQuiet(t *testing.T) {
	t.Parallel()

	d := &stubDialog{id: "test"}
	o := NewOverlay()
	o.OpenDialogWithGrace(d)

	// Backdate so both deadlines have elapsed.
	o.graceOpenedAt = time.Now().Add(-graceMaxDelay - time.Millisecond)
	o.graceLastInputAt = time.Now().Add(-graceQuietPeriod - time.Millisecond)

	o.Update(keyMsg('a'))
	require.Len(t, d.received, 1, "key after grace period should reach the dialog")
}

// TestOverlay_GracePeriodBurstExtendsQuietWindow verifies that a sustained
// burst of keystrokes keeps resetting the quiet timer, but the fixed
// ceiling (graceMaxDelay) eventually arms the dialog regardless.
func TestOverlay_GracePeriodBurstExtendsQuietWindow(t *testing.T) {
	t.Parallel()

	d := &stubDialog{id: "test"}
	o := NewOverlay()
	o.OpenDialogWithGrace(d)

	// Simulate keys arriving every 40ms (within graceQuietPeriod).
	// Each one resets the quiet timer, but we stay under graceMaxDelay.
	for range 5 {
		o.graceLastInputAt = time.Now().Add(-40 * time.Millisecond)
		o.Update(keyMsg('a'))
	}
	require.Empty(t, d.received, "burst within max delay should be absorbed")

	// Now exceed the max delay ceiling. Even though lastInputAt is recent,
	// the fixed deadline forces arming.
	o.graceOpenedAt = time.Now().Add(-graceMaxDelay - time.Millisecond)
	o.graceLastInputAt = time.Now() // just typed, but max delay wins
	o.Update(keyMsg('b'))
	require.Len(t, d.received, 1, "key after max delay should reach dialog even during burst")
}

// TestOverlay_OpenDialogWithoutGraceHasNoGuard verifies that dialogs
// opened via OpenDialog (without grace) receive keys immediately.
func TestOverlay_OpenDialogWithoutGraceHasNoGuard(t *testing.T) {
	t.Parallel()

	d := &stubDialog{id: "test"}
	o := NewOverlay()
	o.OpenDialog(d)

	o.Update(keyMsg('a'))
	require.Len(t, d.received, 1, "dialog without grace should receive keys immediately")
}

// TestOverlay_GraceClearedOnClose verifies that closing the front dialog
// clears grace state so a subsequently opened dialog without grace is
// not affected.
func TestOverlay_GraceClearedOnClose(t *testing.T) {
	t.Parallel()

	d1 := &stubDialog{id: "first"}
	d2 := &stubDialog{id: "second"}
	o := NewOverlay()
	o.OpenDialogWithGrace(d1)

	// Close the grace dialog and open a normal one.
	o.CloseFrontDialog()
	o.OpenDialog(d2)

	o.Update(keyMsg('a'))
	require.Len(t, d2.received, 1, "new dialog after grace close should receive keys immediately")
}

// TestOverlay_NonKeyMessagesPassDuringGrace verifies that non-keypress
// messages (e.g. mouse, tick) are forwarded even during the grace period.
func TestOverlay_NonKeyMessagesPassDuringGrace(t *testing.T) {
	t.Parallel()

	d := &stubDialog{id: "test"}
	o := NewOverlay()
	o.OpenDialogWithGrace(d)

	o.Update(tea.MouseWheelMsg{})
	require.Len(t, d.received, 1, "non-key messages should pass through during grace")
}
