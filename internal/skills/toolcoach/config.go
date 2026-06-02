package toolcoach

import "github.com/charmbracelet/crush/internal/config"

const (
	// DefaultMaxPatternsPerTurn is the maximum number of coaching tips that
	// can fire in a single agent turn. This prevents the coach from becoming
	// noisy or overwhelming.
	DefaultMaxPatternsPerTurn = 3

	// DefaultHistorySize is the number of recent tool calls to retain for
	// sequence-based pattern detection.
	DefaultHistorySize = 20

	// DefaultEffectivenessLookback is how far back to look when loading
	// historical pattern effectiveness for adaptive severity.
	DefaultEffectivenessLookbackDays = 30

	// DefaultAutoRetrySessions is the number of unique sessions after which
	// the coaching intensity auto-switches from tutor to balanced.
	DefaultAutoRetrySessions = 3
)

// CoachingIntensity controls how aggressively the coach fires tips.
type CoachingIntensity string

const (
	// CoachingTutor fires all hints with verbose explanations.
	CoachingTutor CoachingIntensity = "tutor"
	// CoachingBalanced suppresses hint-level tips after the user has seen
	// enough sessions (experienced user mode).
	CoachingBalanced CoachingIntensity = "balanced"
	// CoachingMinimal fires only critical-level tips.
	CoachingMinimal CoachingIntensity = "minimal"
)

// ToolcoachConfig holds runtime configuration for the tool pattern coach.
type ToolcoachConfig struct {
	Enabled                   bool
	MaxPatternsPerTurn        int
	EnabledPatterns           []string
	AdaptiveSeverity          bool
	EffectivenessLookbackDays int
	Intensity                 CoachingIntensity
	AutoRetry                 bool
	AutoRetrySessions         int // sessions before auto-switching tutor→balanced

	// enabledSet is a pre-computed lookup of EnabledPatterns for O(1)
	// membership tests. It is built in NewToolcoachConfig and never modified.
	enabledSet map[string]struct{}
}

// NewToolcoachConfig builds runtime config from the global config.
func NewToolcoachConfig(cfg *config.Config) ToolcoachConfig {
	if cfg == nil || cfg.Options == nil || cfg.Options.Toolcoach == nil {
		return ToolcoachConfig{
			MaxPatternsPerTurn: DefaultMaxPatternsPerTurn,
		}
	}

	tc := cfg.Options.Toolcoach
	c := ToolcoachConfig{
		Enabled:                   tc.IsEnabled(),
		MaxPatternsPerTurn:        tc.MaxPatternsPerTurn,
		EnabledPatterns:           tc.EnabledPatterns,
		EffectivenessLookbackDays: tc.EffectivenessLookbackDays,
		AutoRetrySessions:         tc.AutoRetrySessions,
	}
	if tc.AdaptiveSeverity != nil {
		c.AdaptiveSeverity = *tc.AdaptiveSeverity
	}
	if tc.AutoRetry != nil {
		c.AutoRetry = *tc.AutoRetry
	}
	c.Intensity = CoachingIntensity(tc.Intensity)
	if c.Intensity == "" {
		c.Intensity = CoachingTutor
	}
	if c.MaxPatternsPerTurn <= 0 {
		c.MaxPatternsPerTurn = DefaultMaxPatternsPerTurn
	}
	if c.EffectivenessLookbackDays <= 0 {
		c.EffectivenessLookbackDays = DefaultEffectivenessLookbackDays
	}
	if c.AutoRetrySessions <= 0 {
		c.AutoRetrySessions = DefaultAutoRetrySessions
	}
	if len(c.EnabledPatterns) > 0 {
		c.enabledSet = make(map[string]struct{}, len(c.EnabledPatterns))
		for _, id := range c.EnabledPatterns {
			c.enabledSet[id] = struct{}{}
		}
	}
	return c
}
