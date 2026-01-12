package config

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTUIOptions(t *testing.T) {
	t.Run("quit_default_yes", func(t *testing.T) {
		t.Run("defaults to false", func(t *testing.T) {
			cfg, err := LoadReader(strings.NewReader("{}"))
			require.NoError(t, err)
			require.False(t, cfg.TUIOptions().IsQuitDefaultYes())
		})

		t.Run("can be set to true", func(t *testing.T) {
			cfg, err := LoadReader(strings.NewReader(`{"options": {"tui": {"quit_default_yes": true}}}`))
			require.NoError(t, err)
			require.True(t, cfg.TUIOptions().IsQuitDefaultYes())
		})

		t.Run("merging works", func(t *testing.T) {
			data1 := strings.NewReader(`{"options": {"tui": {"quit_default_yes": false}}}`)
			data2 := strings.NewReader(`{"options": {"tui": {"quit_default_yes": true}}}`)

			merged, err := Merge([]io.Reader{data1, data2})
			require.NoError(t, err)

			cfg, err := LoadReader(merged)
			require.NoError(t, err)
			require.True(t, cfg.TUIOptions().IsQuitDefaultYes())
		})
	})

	t.Run("confirm_quit", func(t *testing.T) {
		t.Run("defaults to true", func(t *testing.T) {
			cfg, err := LoadReader(strings.NewReader("{}"))
			require.NoError(t, err)
			require.True(t, cfg.TUIOptions().ShouldConfirmQuit())
		})

		t.Run("can be set to false", func(t *testing.T) {
			cfg, err := LoadReader(strings.NewReader(`{"options": {"tui": {"confirm_quit": false}}}`))
			require.NoError(t, err)
			require.False(t, cfg.TUIOptions().ShouldConfirmQuit())
		})

		t.Run("merging works", func(t *testing.T) {
			data1 := strings.NewReader(`{"options": {"tui": {"confirm_quit": true}}}`)
			data2 := strings.NewReader(`{"options": {"tui": {"confirm_quit": false}}}`)

			merged, err := Merge([]io.Reader{data1, data2})
			require.NoError(t, err)

			cfg, err := LoadReader(merged)
			require.NoError(t, err)
			require.False(t, cfg.TUIOptions().ShouldConfirmQuit())
		})
	})
}
