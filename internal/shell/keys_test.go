package shell

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestParseKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    uv.KeyPressEvent
		wantErr bool
	}{
		{name: "enter", want: uv.KeyPressEvent{Code: uv.KeyEnter}},
		{name: "return", want: uv.KeyPressEvent{Code: uv.KeyEnter}},
		{name: "tab", want: uv.KeyPressEvent{Code: uv.KeyTab}},
		{name: "esc", want: uv.KeyPressEvent{Code: uv.KeyEscape}},
		{name: "escape", want: uv.KeyPressEvent{Code: uv.KeyEscape}},
		{name: "backspace", want: uv.KeyPressEvent{Code: uv.KeyBackspace}},
		{name: "delete", want: uv.KeyPressEvent{Code: uv.KeyDelete}},
		{name: "insert", want: uv.KeyPressEvent{Code: uv.KeyInsert}},
		{name: "up", want: uv.KeyPressEvent{Code: uv.KeyUp}},
		{name: "down", want: uv.KeyPressEvent{Code: uv.KeyDown}},
		{name: "left", want: uv.KeyPressEvent{Code: uv.KeyLeft}},
		{name: "right", want: uv.KeyPressEvent{Code: uv.KeyRight}},
		{name: "home", want: uv.KeyPressEvent{Code: uv.KeyHome}},
		{name: "end", want: uv.KeyPressEvent{Code: uv.KeyEnd}},
		{name: "pageup", want: uv.KeyPressEvent{Code: uv.KeyPgUp}},
		{name: "pagedown", want: uv.KeyPressEvent{Code: uv.KeyPgDown}},
		{name: "space", want: uv.KeyPressEvent{Code: ' ', Text: " "}},
		{name: "f1", want: uv.KeyPressEvent{Code: uv.KeyF1}},
		{name: "f12", want: uv.KeyPressEvent{Code: uv.KeyF12}},

		{name: "ctrl+c", want: uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl}},
		{name: "ctrl+z", want: uv.KeyPressEvent{Code: 'z', Mod: uv.ModCtrl}},
		{name: "ctrl+space", want: uv.KeyPressEvent{Code: ' ', Text: "", Mod: uv.ModCtrl}},
		{name: "alt+enter", want: uv.KeyPressEvent{Code: uv.KeyEnter, Mod: uv.ModAlt}},
		{name: "shift+tab", want: uv.KeyPressEvent{Code: uv.KeyTab, Mod: uv.ModShift}},

		// Single printable characters keep their text so shifted and
		// composed characters are sent as the symbol itself.
		{name: "a", want: uv.KeyPressEvent{Code: 'a', Text: "a"}},
		{name: "A", want: uv.KeyPressEvent{Code: 'A', Text: "A"}},
		{name: "!", want: uv.KeyPressEvent{Code: '!', Text: "!"}},
		{name: "?", want: uv.KeyPressEvent{Code: '?', Text: "?"}},

		{name: "", wantErr: true},
		{name: "ctrl", wantErr: true},
		{name: "shift", wantErr: true},
		{name: "ctrl+", wantErr: true},
		{name: "blahblah", wantErr: true},
		{name: "banana+enter", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseKey(tc.name)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestSendKey drives a real session with semantic keys and verifies the
// child reads the encoded keystrokes.
func TestInteractiveSessionSendKey(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, `read line; echo "keys:$line"`)

	require.NoError(t, session.SendKey(mustKey(t, "h")))
	require.NoError(t, session.SendKey(mustKey(t, "i")))
	require.NoError(t, session.SendKey(mustKey(t, "enter")))

	waitForExit(t, session)
	require.Contains(t, session.CaptureText(), "keys:hi")
}

func mustKey(t *testing.T, name string) uv.KeyPressEvent {
	t.Helper()
	key, err := ParseKey(name)
	require.NoError(t, err)
	return key
}
