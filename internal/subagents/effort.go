package subagents

import (
	"charm.land/catwalk/pkg/catwalk"

	"github.com/charmbracelet/crush/internal/config"
)

// Effort level constants — these are the catwalk ReasoningLevels values and
// pass through directly to config.SelectedModel.ReasoningEffort.
const (
	EffortNone    = "none"
	EffortMinimal = "minimal"
	EffortLow     = "low"
	EffortMedium  = "medium"
	EffortHigh    = "high"
	EffortXHigh   = "xhigh"
	EffortMax     = "max"
)

// EffortIgnored reports whether a non-empty effort would be silently dropped
// because the model cannot reason. Callers use it to warn on misconfiguration;
// ApplyEffortToModel no-ops in the same case.
func EffortIgnored(effort string, catwalkModel catwalk.Model) bool {
	return effort != "" && !catwalkModel.CanReason
}

// ApplyEffortToModel applies the given effort level to a copy of selectedModel
// and returns the modified copy. The catwalkModel is used to determine whether
// the model supports reasoning.
//
// Rules:
//   - Empty effort is a no-op: the copy is returned unchanged.
//   - Models where CanReason is false are never modified.
//   - All other models: ReasoningEffort is set directly to the effort string.
//     The coordinator's shouldSetEffort check (slices.Contains(ReasoningLevels,
//     ReasoningEffort)) handles unsupported levels gracefully at dispatch time.
func ApplyEffortToModel(effort string, selectedModel config.SelectedModel, catwalkModel catwalk.Model) config.SelectedModel {
	if effort == "" || !catwalkModel.CanReason {
		return selectedModel
	}
	result := selectedModel
	result.ReasoningEffort = effort
	return result
}
