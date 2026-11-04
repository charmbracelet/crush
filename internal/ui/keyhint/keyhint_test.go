package keyhint

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/stretchr/testify/require"
)

func setDarwin(t *testing.T, value bool) {
	t.Helper()
	old := isDarwin
	isDarwin = value
	t.Cleanup(func() { isDarwin = old })
}

func TestLabel(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		setDarwin(t, true)

		tests := []struct {
			name string
			in   string
			want string
		}{
			{"single modifier", "ctrl+p", "⌃p"},
			{"multiple modifiers", "ctrl+shift+v", "⌃shift+v"},
			{"composite label", "y/Y/ctrl+c", "y/Y/⌃c"},
			{"placeholder", "ctrl+r+{i}", "⌃r+{i}"},
			{"bare modifier", "ctrl", "⌃"},
			{"unrelated key", "enter", "enter"},
			{"already a word label", "control+p", "control+p"},
			{"empty", "", ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				require.Equal(t, tt.want, Label(tt.in))
			})
		}
	})

	t.Run("other platforms", func(t *testing.T) {
		setDarwin(t, false)
		require.Equal(t, "ctrl+p", Label("ctrl+p"))
		require.Equal(t, "y/Y/ctrl+c", Label("y/Y/ctrl+c"))
	})
}

func TestLabelIsIdempotent(t *testing.T) {
	setDarwin(t, true)

	require.Equal(t, "⌃p", Label(Label("ctrl+p")),
		"labelling an already-labelled hint must not change it again")
}

func TestRelabelPreservesKeysAndState(t *testing.T) {
	setDarwin(t, true)

	b := key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "commands"),
	)
	got := relabel(b)

	require.Equal(t, []string{"ctrl+p"}, got.Keys())
	require.Equal(t, "⌃p", got.Help().Key)
	require.Equal(t, "commands", got.Help().Desc)
	require.True(t, got.Enabled())

	require.Equal(t, "ctrl+p", b.Help().Key, "the original binding must not be mutated")
}

func TestRelabelPreservesDisabledState(t *testing.T) {
	setDarwin(t, true)

	b := key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "commands"),
		key.WithDisabled(),
	)
	require.False(t, relabel(b).Enabled())
}

func TestRelabelWithoutHelp(t *testing.T) {
	setDarwin(t, true)

	b := key.NewBinding(key.WithKeys("ctrl+p"))
	require.Equal(t, b, relabel(b))
}

func TestRelabelAll(t *testing.T) {
	setDarwin(t, true)

	in := []key.Binding{
		key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "first")),
		key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "second")),
	}
	got := relabelAll(in)
	require.Len(t, got, 2)
	require.Equal(t, "⌃a", got[0].Help().Key)
	require.Equal(t, "⌃b", got[1].Help().Key)

	require.Nil(t, relabelAll(nil))
	require.Empty(t, relabelAll([]key.Binding{}))
}

type testKeyMap struct{}

func (testKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "commands")),
	}
}

func (testKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "commands")),
		},
		nil,
	}
}

func TestWrap(t *testing.T) {
	setDarwin(t, true)

	km := Wrap(testKeyMap{})

	short := km.ShortHelp()
	require.Len(t, short, 1)
	require.Equal(t, "⌃p", short[0].Help().Key)

	full := km.FullHelp()
	require.Len(t, full, 2)
	require.Equal(t, "⌃p", full[0][0].Help().Key)
	require.Nil(t, full[1], "nil groups must stay nil")
}

func TestWrapOtherPlatforms(t *testing.T) {
	setDarwin(t, false)

	km := Wrap(testKeyMap{})
	require.Equal(t, testKeyMap{}, km, "other platforms get the keymap back unchanged")
	require.Equal(t, "ctrl+p", km.ShortHelp()[0].Help().Key)
	require.Equal(t, "ctrl+p", km.FullHelp()[0][0].Help().Key)
}

type nilFullHelpKeyMap struct{ testKeyMap }

func (nilFullHelpKeyMap) FullHelp() [][]key.Binding { return nil }

func TestWrapNilFullHelp(t *testing.T) {
	setDarwin(t, true)

	require.Nil(t, Wrap(nilFullHelpKeyMap{}).FullHelp(),
		"a keymap without full help must stay nil")
}
