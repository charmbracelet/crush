package toolcoach

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewToolcoachConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil config returns defaults", func(t *testing.T) {
		t.Parallel()
		cfg := NewToolcoachConfig(nil)
		require.False(t, cfg.Enabled)
		require.Equal(t, DefaultMaxPatternsPerTurn, cfg.MaxPatternsPerTurn)
	})

	t.Run("empty options returns defaults", func(t *testing.T) {
		t.Parallel()
		cfg := NewToolcoachConfig(&config.Config{})
		require.False(t, cfg.Enabled)
		require.Equal(t, DefaultMaxPatternsPerTurn, cfg.MaxPatternsPerTurn)
	})

	t.Run("auto enable when section present", func(t *testing.T) {
		t.Parallel()
		c := &config.Config{
			Options: &config.Options{
				Toolcoach: &config.ToolcoachConfig{},
			},
		}
		cfg := NewToolcoachConfig(c)
		require.True(t, cfg.Enabled)
		require.Equal(t, DefaultMaxPatternsPerTurn, cfg.MaxPatternsPerTurn)
	})

	t.Run("explicit disable", func(t *testing.T) {
		t.Parallel()
		f := false
		c := &config.Config{
			Options: &config.Options{
				Toolcoach: &config.ToolcoachConfig{Enabled: &f},
			},
		}
		cfg := NewToolcoachConfig(c)
		require.False(t, cfg.Enabled)
	})

	t.Run("custom max patterns", func(t *testing.T) {
		t.Parallel()
		c := &config.Config{
			Options: &config.Options{
				Toolcoach: &config.ToolcoachConfig{
					MaxPatternsPerTurn: 5,
				},
			},
		}
		cfg := NewToolcoachConfig(c)
		require.True(t, cfg.Enabled)
		require.Equal(t, 5, cfg.MaxPatternsPerTurn)
	})

	t.Run("zero max patterns falls back to default", func(t *testing.T) {
		t.Parallel()
		c := &config.Config{
			Options: &config.Options{
				Toolcoach: &config.ToolcoachConfig{
					MaxPatternsPerTurn: 0,
				},
			},
		}
		cfg := NewToolcoachConfig(c)
		require.Equal(t, DefaultMaxPatternsPerTurn, cfg.MaxPatternsPerTurn)
	})
}
