package subagents

import (
	"slices"

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

// EffortIgnored reports whether a non-empty effort would be dropped because
// the model cannot reason or does not list it among its ReasoningLevels.
// Callers use it to warn on misconfiguration; ApplyEffortToModel no-ops in
// the same case.
func EffortIgnored(effort string, catwalkModel catwalk.Model) bool {
	return effort != "" &&
		(!catwalkModel.CanReason || !slices.Contains(catwalkModel.ReasoningLevels, effort))
}

// ApplyEffortToModel applies the given effort level to a copy of selectedModel
// and returns the modified copy. The catwalkModel is used to determine whether
// the model supports reasoning.
//
// Rules:
//   - Empty effort is a no-op: the copy is returned unchanged.
//   - Ignored efforts (see EffortIgnored) keep the user's ReasoningEffort;
//     overwriting it would make the call-time fallback pick the catwalk
//     default instead of either the request or the user's setting.
//   - Otherwise ReasoningEffort is set directly to the effort string.
func ApplyEffortToModel(effort string, selectedModel config.SelectedModel, catwalkModel catwalk.Model) config.SelectedModel {
	if effort == "" || EffortIgnored(effort, catwalkModel) {
		return selectedModel
	}
	result := selectedModel
	result.ReasoningEffort = effort
	return result
}
