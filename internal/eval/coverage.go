package eval

import (
	"fmt"
	"strings"
)

// coverageFields is the closed set of run-record field paths a coverage
// predicate may compare against. Keeping it a table (not reflection)
// is what makes the grammar closed: nothing else is reachable.
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
