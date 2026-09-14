package eval

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/config"
)

// validateFlagSet returns problems for keys absent from
// config.Options. Without this, a typo'd flag is silently ignored by
// the child's json.Unmarshal, producing an experiment whose arms'
// effective configs are identical: a guaranteed-null gate that
// reports PASS on a broken experiment.
func validateFlagSet(keys []string, where string) []string {
	valid := config.OptionKeys()
	var problems []string
	for _, k := range keys {
		if !valid[k] {
			problems = append(problems, fmt.Sprintf("%s %q is not a config.Options key — the flag would silently no-op", where, k))
		}
	}
	return problems
}

// flagNames is the deterministic env list ordering for
// CRUSH_EVAL_FLAGS.
func flagNames(m *FlagsManifest) []string {
	return sortedKeys(m.Defaults)
}

// resolvedEqual reports whether two resolved-options projections are
// identical — the no-op-flag detector: arm intents differed but the
// flag resolved to the same effective value in both.
func resolvedEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || fmt.Sprintf("%v", va) != fmt.Sprintf("%v", vb) {
			return false
		}
	}
	return true
}
