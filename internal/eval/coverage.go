package eval

import (
	"fmt"
	"maps"
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
// headless) and can never be shared predicates.
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
	"checkpoints.written":          func(r *RunRecord) float64 { return float64(r.Checkpoints.Written) },
	"checkpoints.rendered":         func(r *RunRecord) float64 { return float64(r.Checkpoints.Rendered) },
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
	// read_files_rows is flag-invariant — filetracker populates it in
	// both arms. The -1 sentinel on pre-table artifacts fails min_
	// predicates closed rather than admitting a zero.
	"call_metrics.read_files_rows": func(r *RunRecord) float64 { return float64(callMetrics(r).ReadFilesRows) },
}

// armOnlyCoverageFields holds the flag-dependent call_metrics fields:
// unreachable or meaningless in the arm where the feature is off, so
// they can never join trajectory coverage (one arm would starve or
// mismeasure on every run). Arm-scoped coverage exists precisely to
// assert them — a treatment arm enabling project_index may demand the
// model actually reached for map, or bound the bytes it paid for
// results. map_calls is included although it also counts tool-not-
// found attempts in flag-off arms (unprompted reach is measurable
// there too); its sibling fields are what require the flag.
var armOnlyCoverageFields = map[string]func(*RunRecord) float64{
	"call_metrics.map_calls":                   func(r *RunRecord) float64 { return float64(callMetrics(r).MapCalls) },
	"call_metrics.map_calls_ok":                func(r *RunRecord) float64 { return float64(callMetrics(r).MapCallsOK) },
	"call_metrics.map_calls_index_unavailable": func(r *RunRecord) float64 { return float64(callMetrics(r).MapCallsIndexUnavailable) },
	"call_metrics.map_result_bytes":            func(r *RunRecord) float64 { return float64(callMetrics(r).MapResultBytes) },
	"call_metrics.question_calls":              func(r *RunRecord) float64 { return float64(callMetrics(r).QuestionCalls) },
	"call_metrics.question_calls_errored":      func(r *RunRecord) float64 { return float64(callMetrics(r).QuestionCallsErrored) },
	"call_metrics.wrong_pointer_events":        func(r *RunRecord) float64 { return float64(callMetrics(r).WrongPointerEvents) },
}

// armFields is the arm-coverage grammar: every trajectory-coverage
// field plus the flag-dependent set above. Built in init after the
// kinds loop so stub_stats.kinds.* is reachable from arm coverage too.
var armFields map[string]func(*RunRecord) float64

// flagGatedPrefixes name coverage fields whose counters only exist
// when a feature flag is on — stub_stats.* need notebook_stub_superseded,
// recalls.* need a registered recall tool. An unscoped min_ predicate
// over one of these starves the arm where the flag is off, so
// trajectory coverage rejects them; scope them per-arm instead.
var flagGatedPrefixes = []string{"stub_stats.", "recalls.", "checkpoints."}

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
	armFields = maps.Clone(coverageFields)
	maps.Copy(armFields, armOnlyCoverageFields)
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

// ParseArmCoverageKey validates an arm-scoped coverage key — the same
// grammar as ParseCoverageKey but the field set is wider: arm coverage
// may name flag-dependent fields because the arm itself fixes the
// flag's value, so "did the mechanism fire" is assertable there.
func ParseArmCoverageKey(key string) (op, field string, err error) {
	op, field, ok := strings.Cut(key, "_")
	if !ok || (op != "min" && op != "max") {
		return "", "", fmt.Errorf("expected min_<field> or max_<field>")
	}
	if _, ok := armFields[field]; !ok {
		return "", "", fmt.Errorf("unknown field %q (have: %s)", field, strings.Join(coverageFieldNamesFor(armFields), ", "))
	}
	return op, field, nil
}

// CoverageMet reports whether a run's record satisfies every predicate.
// Evaluated only on runs that produced a verdict — coverage unmet
// converts pass to inconclusive; fails stand regardless.
func CoverageMet(cov Coverage, rec *RunRecord) (bool, error) {
	met, _, err := coverageMet(cov, rec, coverageFields)
	return met, err
}

// ArmCoverageMet is CoverageMet over the arm grammar — the flag-
// dependent call_metrics fields are reachable here because the arm
// fixes the flag. Runs of other arms never see this arm's predicates;
// the caller passes the run's own arm's coverage block.
func ArmCoverageMet(cov Coverage, rec *RunRecord) (bool, error) {
	met, _, err := coverageMet(cov, rec, armFields)
	return met, err
}

// coverageMet returns the failing predicate key alongside the verdict
// so run records can say what starved them, not just which scope.
func coverageMet(cov Coverage, rec *RunRecord, fields map[string]func(*RunRecord) float64) (bool, string, error) {
	for key, want := range cov {
		op, field, ok := strings.Cut(key, "_")
		if !ok || (op != "min" && op != "max") {
			return false, "", fmt.Errorf("coverage %q: expected min_<field> or max_<field>", key)
		}
		fn, ok := fields[field]
		if !ok {
			return false, "", fmt.Errorf("coverage %q: unknown field %q", key, field)
		}
		// Absent analysis starves call_metrics predicates in BOTH
		// directions — max_* must not pass on a missing analysis.
		if strings.HasPrefix(field, "call_metrics.") && rec.CallMetrics == nil {
			return false, key, nil
		}
		got := fn(rec)
		switch op {
		case "min":
			if got < want {
				return false, key, nil
			}
		case "max":
			if got > want {
				return false, key, nil
			}
		}
	}
	return true, "", nil
}

func coverageFieldNames() []string {
	return coverageFieldNamesFor(coverageFields)
}

func coverageFieldNamesFor(fields map[string]func(*RunRecord) float64) []string {
	return sortedKeys(fields)
}
