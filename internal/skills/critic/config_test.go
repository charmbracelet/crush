package critic

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewCriticSkillConfig_NilCritic(t *testing.T) {
	t.Parallel()
	cfg := NewCriticSkillConfig(&config.Config{
		Options: &config.Options{},
	})
	require.False(t, cfg.Enabled)
	require.Equal(t, DefaultMaxIterations, cfg.MaxIterations)
}

func TestNewCriticSkillConfig_Overrides(t *testing.T) {
	t.Parallel()
	enabled := true
	globalCfg := &config.Config{
		Options: &config.Options{
			Critic: &config.CriticConfig{
				Enabled:       &enabled,
				Model:         "gpt-4o-mini",
				MaxIterations: 5,
				AutoApprove:   true,
				Threshold:     0.9,
				CacheSize:     64,
				MaxDiffSize:   1024,
				MaxFileSize:   2048,
				Timeout:       5,
				RetentionDays: 7,
			},
		},
	}
	cfg := NewCriticSkillConfig(globalCfg)
	require.True(t, cfg.Enabled)
	require.Equal(t, "gpt-4o-mini", cfg.Model)
	require.Equal(t, 5, cfg.MaxIterations)
	require.True(t, cfg.AutoApprove)
	require.InDelta(t, 0.9, cfg.Threshold, 0.001)
	require.Equal(t, 64, cfg.CacheSize)
	require.Equal(t, 1024, cfg.MaxDiffSize)
	require.Equal(t, 2048, cfg.MaxFileSize)
	require.Equal(t, 5, int(cfg.Timeout))
	require.Equal(t, 7, cfg.RetentionDays)
}

func TestNewCriticSkillConfig_DefaultsForZeroValues(t *testing.T) {
	t.Parallel()
	enabled := true
	globalCfg := &config.Config{
		Options: &config.Options{
			Critic: &config.CriticConfig{
				Enabled:       &enabled,
				MaxIterations: 0,
				Threshold:     0,
				CacheSize:     0,
				MaxDiffSize:   0,
				MaxFileSize:   0,
				Timeout:       0,
				RetentionDays: 0,
			},
		},
	}
	cfg := NewCriticSkillConfig(globalCfg)
	require.True(t, cfg.Enabled)
	require.Equal(t, 3, cfg.MaxIterations)
	require.InDelta(t, 0.85, cfg.Threshold, 0.001)
	require.Equal(t, 32, cfg.CacheSize)
	require.Equal(t, DefaultMaxDiffSize, cfg.MaxDiffSize)
	require.Equal(t, DefaultMaxFileSize, cfg.MaxFileSize)
	require.Equal(t, DefaultTimeout, cfg.Timeout)
	require.Equal(t, DefaultRetentionDays, cfg.RetentionDays)
}

func TestNewCriticSkillConfig_GlobalDisable(t *testing.T) {
	t.Setenv("CRUSH_CRITIC_GLOBAL_DISABLE", "1")
	enabled := true
	globalCfg := &config.Config{
		Options: &config.Options{
			Critic: &config.CriticConfig{
				Enabled: &enabled,
			},
		},
	}
	cfg := NewCriticSkillConfig(globalCfg)
	require.False(t, cfg.Enabled)
}

func TestNewCriticSkillConfig_AutoEnable(t *testing.T) {
	t.Parallel()
	// When the critic section exists but enabled is omitted, it auto-enables.
	globalCfg := &config.Config{
		Options: &config.Options{
			Critic: &config.CriticConfig{},
		},
	}
	cfg := NewCriticSkillConfig(globalCfg)
	require.True(t, cfg.Enabled)
}

func TestNewCriticSkillConfig_ExplicitDisable(t *testing.T) {
	t.Parallel()
	disabled := false
	globalCfg := &config.Config{
		Options: &config.Options{
			Critic: &config.CriticConfig{
				Enabled: &disabled,
			},
		},
	}
	cfg := NewCriticSkillConfig(globalCfg)
	require.False(t, cfg.Enabled)
}
