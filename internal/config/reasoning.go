package config

import (
	"log/slog"
	"slices"
	"strings"
)

// ompEffortOrder ranks the OMP-style reasoning suffixes accepted on
// `crush run` model flags from least to most reasoning effort.
var ompEffortOrder = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

// SplitModelEffort separates an OMP-style reasoning-effort suffix from a
// model string such as "anthropic/claude-opus-5:xhigh". It returns the model
// string without the suffix and the requested effort. When the trailing
// segment after the last colon is not a known effort level (e.g. openrouter
// ":free" or ":exacto" model IDs), the model string is returned unchanged
// with an empty effort.
func SplitModelEffort(model string) (clean string, effort string) {
	if model == "" {
		return model, ""
	}
	idx := strings.LastIndex(model, ":")
	if idx < 0 {
		return model, ""
	}
	suffix := model[idx+1:]
	if !slices.Contains(ompEffortOrder, suffix) {
		return model, ""
	}
	return model[:idx], suffix
}

// ResolveReasoningEffort maps an OMP reasoning-effort suffix to the closest
// reasoning level the model supports. "off" maps to "none" when the model
// supports it (the standard way to disable reasoning); otherwise, when the
// model has no disable level, it returns "" so the caller leaves the effort
// unset. Unknown or empty efforts also return "".
func ResolveReasoningEffort(effort string, levels []string) string {
	if effort == "" || len(levels) == 0 {
		return ""
	}

	// Direct hit, including "off" aliasing "none".
	for _, lv := range levels {
		if lv == effort || (effort == "off" && lv == "none") {
			return lv
		}
	}

	// "off" with no disable level cannot be honored.
	if effort == "off" {
		return ""
	}

	want := -1
	for i, lv := range ompEffortOrder {
		if lv == effort {
			want = i
			break
		}
	}
	if want < 0 {
		return ""
	}

	// Pick the supported level closest to the requested effort on the
	// OMP ordering. Unknown levels (e.g. "none") are skipped.
	best := ""
	bestDist := int(^uint(0) >> 1)
	for _, lv := range levels {
		idx := -1
		for i, o := range ompEffortOrder {
			if o == lv {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		d := idx - want
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = lv
		}
	}
	return best
}

// ResolveModelReasoningEffort maps an OMP reasoning-effort suffix to the
// closest supported level for a specific model within a provider config. It
// logs a warning when the requested effort cannot be honored so silent
// fallback to the model default is visible to the user.
func ResolveModelReasoningEffort(p ProviderConfig, modelID, effort string) string {
	if effort == "" {
		return ""
	}
	for _, m := range p.Models {
		if m.ID != modelID {
			continue
		}
		resolved := ResolveReasoningEffort(effort, m.ReasoningLevels)
		if resolved == "" {
			slog.Warn("Reasoning effort not supported by model, using default",
				"model", modelID, "effort", effort, "levels", m.ReasoningLevels)
		}
		return resolved
	}
	return ""
}
