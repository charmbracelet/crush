package eval

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// coverageFields is the closed set of run-record field paths a coverage
// predicate may compare against. Keeping it a table (not reflection)
// is what makes the grammar closed: nothing else is reachable.
//
// Warning for corpus authors: all stub_stats.* fields are flag-gated,
// not flag-invariant — stub flagging and promotion are disabled in the
// control arm, so they are structurally 0 there. That includes
// stub_stats.boundary_advances and every stub_stats.kinds.<kind>: the
// counters only exist once a run has promoted a stub, so they cannot
// serve as flag-agnostic "work happened" predicates — they are safe
// only on arms where stubbing is enabled. Predicates over steps and
// tokens.* are safe.
//
// call_metrics.* registers the flag-invariant subset only: map_*,
// question_*, and wrong_pointer_events are absent-by-construction in
// one arm (map isn't registered in control; question isn't registered
// headless) and can never be predicates.
var coverageFields = map[string]func(*RunRecord) float64{
	"steps":                        func(r *RunRecord) float64 { return float64(r.Steps) },
	"tokens.input":                 func(r *RunRecord) float64 { return float64(r.Tokens.Input) },
	"tokens.output":                func(r *RunRecord) float64 { return float64(r.Tokens.Output) },
	"tokens.cache_read":            func(r *RunRecord) float64 { return float64(r.Tokens.CacheRead) },
	"tokens.cache_write":           func(r *RunRecord) float64 { return float64(r.Tokens.CacheWrite) },
	"stub_stats.invalidations":     func(r *RunRecord) float64 { return float64(r.StubStats.Invalidations) },
	"stub_stats.results":           func(r *RunRecord) float64 { return float64(r.StubStats.Results) },
	"stub_stats.saved_bytes":       func(r *RunRecord) float64 { return float64(r.StubStats.SavedBytes) },
	"stub_stats.boundary_advances": func(r *RunRecord) float64 { return float64(r.StubStats.BoundaryAdvances) },
	"recalls.result":               func(r *RunRecord) float64 { return float64(r.Recalls.Result) },
	"recalls.entry":                func(r *RunRecord) float64 { return float64(r.Recalls.Entry) },
	"recalls.empty":                func(r *RunRecord) float64 { return float64(r.Recalls.Empty) },
	"recalls.cross":                func(r *RunRecord) float64 { return float64(r.Recalls.Cross) },
	// Flag-invariant call_metrics subset — see the comment above.
	"call_metrics.requests":          func(r *RunRecord) float64 { return float64(callMetrics(r).Requests) },
	"call_metrics.calls":             func(r *RunRecord) float64 { return float64(callMetrics(r).Calls) },
	"call_metrics.first_write_index": func(r *RunRecord) float64 { return float64(callMetrics(r).FirstWriteIndex) },
	"call_metrics.first_write_attempt_index": func(r *RunRecord) float64 {
		return float64(callMetrics(r).FirstWriteAttemptIndex)
	},
	"call_metrics.requests_to_first_edit": func(r *RunRecord) float64 { return float64(callMetrics(r).RequestsToFirstEdit) },
	"call_metrics.discovery_calls_before_write": func(r *RunRecord) float64 {
		return float64(callMetrics(r).DiscoveryCallsBeforeWrite)
	},
	"call_metrics.files_viewed":             func(r *RunRecord) float64 { return float64(callMetrics(r).FilesViewed) },
	"call_metrics.edit_failures":            func(r *RunRecord) float64 { return float64(callMetrics(r).EditFailures) },
	"call_metrics.edit_failures_hook":       func(r *RunRecord) float64 { return float64(callMetrics(r).EditFailuresHook) },
	"call_metrics.edit_failures_not_found":  func(r *RunRecord) float64 { return float64(callMetrics(r).EditFailuresNotFound) },
	"call_metrics.edit_failures_cancelled":  func(r *RunRecord) float64 { return float64(callMetrics(r).EditFailuresCancelled) },
	"call_metrics.edit_failures_permission": func(r *RunRecord) float64 { return float64(callMetrics(r).EditFailuresPermission) },
	"call_metrics.edit_failures_other":      func(r *RunRecord) float64 { return float64(callMetrics(r).EditFailuresOther) },
	"call_metrics.rereads":                  func(r *RunRecord) float64 { return float64(callMetrics(r).Rereads) },
	"call_metrics.rereads_same_turn":        func(r *RunRecord) float64 { return float64(callMetrics(r).RereadsSameTurn) },
	"call_metrics.rereads_cross_turn":       func(r *RunRecord) float64 { return float64(callMetrics(r).RereadsCrossTurn) },
	"call_metrics.canceled_calls":           func(r *RunRecord) float64 { return float64(callMetrics(r).CanceledCalls) },
	"call_metrics.interrupted_calls":        func(r *RunRecord) float64 { return float64(callMetrics(r).InterruptedCalls) },
	"call_metrics.truncated_calls":          func(r *RunRecord) float64 { return float64(callMetrics(r).TruncatedCalls) },
	"call_metrics.view_directory_errors":    func(r *RunRecord) float64 { return float64(callMetrics(r).ViewDirectoryErrors) },
}

// callMetrics dereferences the optional analysis sub-object. CoverageMet
// short-circuits nil CallMetrics before reaching field funcs, so this
// only runs when analysis is present.
func callMetrics(r *RunRecord) CallMetrics {
	if r.CallMetrics == nil {
		return CallMetrics{}
	}
	return *r.CallMetrics
}

func init() {
	// Per-kind stub counts: stub_stats.kinds.<kind> for every
	// message.StubKind, keyed by the kind's telemetry label — the
	// empty-string superseded kind spells "superseded", so
	// min_stub_stats.kinds.superseded is a real predicate.
	for _, kind := range message.StubKinds() {
		name := kind.String()
		coverageFields["stub_stats.kinds."+name] = func(r *RunRecord) float64 {
			return float64(r.StubStats.Kinds[name])
		}
	}
}

// ParseCoverageKey validates a coverage predicate key at load time:
// "<min|max>_<field-path>" where field-path is in coverageFields.
// Field paths use underscores themselves, so the operator is split on
// the FIRST underscore only.
func ParseCoverageKey(key string) (op, field string, err error) {
	op, field, ok := strings.Cut(key, "_")
	if !ok || (op != "min" && op != "max") {
		return "", "", fmt.Errorf("expected min_<field> or max_<field>")
	}
	if _, ok := coverageFields[field]; !ok {
		return "", "", fmt.Errorf("unknown field %q (have: %s)", field, strings.Join(coverageFieldNames(), ", "))
	}
	return op, field, nil
}

// CoverageMet reports whether a run's record satisfies every predicate.
// Evaluated only on runs that produced a verdict — coverage unmet
// converts pass to inconclusive; fails stand regardless.
func CoverageMet(cov Coverage, rec *RunRecord) (bool, error) {
	for key, want := range cov {
		op, field, err := ParseCoverageKey(key)
		if err != nil {
			return false, err
		}
		// Absent analysis starves call_metrics predicates in BOTH
		// directions — max_* must not pass on a missing analysis.
		if strings.HasPrefix(field, "call_metrics.") && rec.CallMetrics == nil {
			return false, nil
		}
		got := coverageFields[field](rec)
		switch op {
		case "min":
			if got < want {
				return false, nil
			}
		case "max":
			if got > want {
				return false, nil
			}
		}
	}
	return true, nil
}

func coverageFieldNames() []string {
	return sortedKeys(coverageFields)
}
