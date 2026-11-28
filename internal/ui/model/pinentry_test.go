package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/stretchr/testify/require"
)

func TestHandlePinentryEvent(t *testing.T) {
	t.Parallel()

	newUI := func() *UI {
		com := &common.Common{
			Workspace: &testWorkspace{},
			Styles:    &styles.Styles{},
		}
		return &UI{
			com:    com,
			status: NewStatus(com, nil),
		}
	}

	t.Run("tracks terminal handover state without a program", func(t *testing.T) {
		t.Parallel()

		ui := newUI()
		ui.handlePinentryEvent(pinentry.Event{Active: true, CtrlLRedraw: true})
		require.True(t, ui.pinentryActive)
		require.True(t, ui.pinentryCtrlL)

		ui.handlePinentryEvent(pinentry.Event{})
		require.False(t, ui.pinentryActive)
	})

	t.Run("shows and clears the security key touch hint", func(t *testing.T) {
		t.Parallel()

		ui := newUI()
		ui.handlePinentryEvent(pinentry.Event{TouchPending: true})
		require.True(t, ui.pinentryTouchPending)
		require.Equal(t, pinentryTouchMessage, ui.status.msg.Msg)

		ui.handlePinentryEvent(pinentry.Event{})
		require.False(t, ui.pinentryTouchPending)
		require.True(t, ui.status.msg.IsEmpty())
	})

	t.Run("repeated events are idempotent", func(t *testing.T) {
		t.Parallel()

		ui := newUI()
		ui.handlePinentryEvent(pinentry.Event{TouchPending: true})
		ui.handlePinentryEvent(pinentry.Event{TouchPending: true})
		require.True(t, ui.pinentryTouchPending)
		require.Equal(t, pinentryTouchMessage, ui.status.msg.Msg)
	})
}

// TestPinentryTouchClearKeepsNewerMessage checks that the touch landing
// does not wipe a status message that replaced the touch hint.
func TestPinentryTouchClearKeepsNewerMessage(t *testing.T) {
	t.Parallel()

	com := &common.Common{Workspace: &testWorkspace{}, Styles: &styles.Styles{}}
	ui := &UI{com: com, status: NewStatus(com, nil)}
	ui.handlePinentryEvent(pinentry.Event{TouchPending: true})
	ui.status.Show(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "newer"})
	ui.handlePinentryEvent(pinentry.Event{})
	require.Equal(t, "newer", ui.status.msg.Msg)
}
