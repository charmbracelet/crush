package critic

import (
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

// Default critic configuration constants.
const (
	DefaultMaxIterations = 3
	DefaultThreshold     = 0.85
	DefaultCacheSize     = 32
	DefaultMaxDiffSize   = 32 * 1024        // 32KB
	DefaultMaxFileSize   = 10 * 1024 * 1024 // 10MB
	DefaultTimeout       = 10 * time.Second
	DefaultRetentionDays = 30
)

// CriticSkillConfig holds runtime configuration for the critic skill.
type CriticSkillConfig struct {
	// Enabled turns the critic on or off.
	Enabled bool
	// Model is the provider/model ID to use for the critic agent.
	// If empty, the small model from the agent config is used.
	Model string
	// MaxIterations is the maximum number of revision loops.
	MaxIterations int
	// AutoApprove bypasses the confidence threshold.
	AutoApprove bool
	// Threshold is the minimum confidence required for auto-approval.
	Threshold float64
	// CacheSize is the maximum number of cached critic reviews.
	CacheSize int
	// MaxDiffSize is the maximum bytes of diff to send to the critic.
	MaxDiffSize int
	// MaxFileSize is the maximum file size to snapshot.
	MaxFileSize int
	// Timeout is the maximum duration for each critic LLM call.
	Timeout time.Duration
	// RetentionDays is the number of days to retain critic reviews in the database.
	RetentionDays int
}

// NewCriticSkillConfig builds a runtime config from the global config.
func NewCriticSkillConfig(cfg *config.Config) CriticSkillConfig {
	if os.Getenv("CRUSH_CRITIC_GLOBAL_DISABLE") == "1" || os.Getenv("CRUSH_CRITIC_GLOBAL_DISABLE") == "true" {
		slog.Warn("Global critic disable is active via CRUSH_CRITIC_GLOBAL_DISABLE")
		return CriticSkillConfig{
			Enabled:       false,
			MaxIterations: DefaultMaxIterations,
			Threshold:     DefaultThreshold,
			CacheSize:     DefaultCacheSize,
			MaxDiffSize:   DefaultMaxDiffSize,
			MaxFileSize:   DefaultMaxFileSize,
			Timeout:       DefaultTimeout,
			RetentionDays: DefaultRetentionDays,
		}
	}

	if cfg == nil || cfg.Options == nil || cfg.Options.Critic == nil {
		return CriticSkillConfig{
			MaxIterations: DefaultMaxIterations,
			Threshold:     DefaultThreshold,
			CacheSize:     DefaultCacheSize,
			MaxDiffSize:   DefaultMaxDiffSize,
			MaxFileSize:   DefaultMaxFileSize,
			Timeout:       DefaultTimeout,
			RetentionDays: DefaultRetentionDays,
		}
	}

	cc := cfg.Options.Critic
	c := CriticSkillConfig{
		Enabled:       cc.IsEnabled(),
		Model:         cc.Model,
		MaxIterations: cc.MaxIterations,
		AutoApprove:   cc.AutoApprove,
		Threshold:     cc.Threshold,
		CacheSize:     cc.CacheSize,
		MaxDiffSize:   cc.MaxDiffSize,
		MaxFileSize:   cc.MaxFileSize,
		Timeout:       cc.Timeout,
		RetentionDays: cc.RetentionDays,
	}

	if c.MaxIterations <= 0 {
		c.MaxIterations = DefaultMaxIterations
	}
	if c.Threshold <= 0 {
		c.Threshold = DefaultThreshold
	}
	if c.CacheSize <= 0 {
		c.CacheSize = DefaultCacheSize
	}
	if c.MaxDiffSize <= 0 {
		c.MaxDiffSize = DefaultMaxDiffSize
	}
	if c.MaxFileSize <= 0 {
		c.MaxFileSize = DefaultMaxFileSize
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = DefaultRetentionDays
	}

	return c
}
