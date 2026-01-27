package toolchain

// Config holds configuration options for toolchain summarization.
type Config struct {
	// Enabled controls whether toolchain summarization is active.
	Enabled bool `json:"enabled" jsonschema:"description=Enable toolchain summarization,default=true"`

	// MinCalls is the minimum number of tool calls required before summarization occurs.
	// Chains with fewer calls will not be summarized.
	MinCalls int `json:"min_calls,omitempty" jsonschema:"description=Minimum tool calls before summarization,default=3,minimum=1"`

	// MaxOutputLength is the maximum length of tool output to include in summaries.
	// Outputs longer than this will be truncated.
	MaxOutputLength int `json:"max_output_length,omitempty" jsonschema:"description=Maximum tool output length in summary,default=500"`

	// CollapseByDefault determines whether summaries start collapsed.
	CollapseByDefault bool `json:"collapse_by_default,omitempty" jsonschema:"description=Start summaries in collapsed state,default=true"`

	// IncludeTimings includes execution duration information in summaries.
	IncludeTimings bool `json:"include_timings,omitempty" jsonschema:"description=Include timing information in summaries,default=true"`

	// GroupByTool groups consecutive calls to the same tool in summaries.
	GroupByTool bool `json:"group_by_tool,omitempty" jsonschema:"description=Group consecutive calls to same tool,default=true"`
}

// DefaultConfig returns the default configuration for toolchain summarization.
func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		MinCalls:          3,
		MaxOutputLength:   500,
		CollapseByDefault: true,
		IncludeTimings:    true,
		GroupByTool:       true,
	}
}

// ShouldSummarize returns true if the given chain should be summarized
// based on the configuration settings.
func (c *Config) ShouldSummarize(chain *Chain) bool {
	if !c.Enabled {
		return false
	}
	if chain == nil || chain.IsEmpty() {
		return false
	}
	return chain.Len() >= c.MinCalls
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if c.MinCalls < 1 {
		c.MinCalls = 1
	}
	if c.MaxOutputLength < 0 {
		c.MaxOutputLength = 0
	}
	return nil
}

// Merge combines this config with another, with the other config taking precedence
// for any non-zero values.
func (c *Config) Merge(other Config) Config {
	result := *c

	// other.Enabled always takes precedence since it's a bool
	result.Enabled = other.Enabled

	if other.MinCalls > 0 {
		result.MinCalls = other.MinCalls
	}
	if other.MaxOutputLength > 0 {
		result.MaxOutputLength = other.MaxOutputLength
	}

	// Bools take precedence from other
	result.CollapseByDefault = other.CollapseByDefault
	result.IncludeTimings = other.IncludeTimings
	result.GroupByTool = other.GroupByTool

	return result
}
