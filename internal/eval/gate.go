package eval

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"strings"
)

// Report is the experiment's gate verdict. Three multiple-comparison
// families: catastrophic (Bonferroni over the full stable band),
// diffuse (single corpus-level test), and the excluded-class
// differential (per-trajectory, its own correction).
type Report struct {
	// Catastrophic lists trajectories whose treatment arm collapsed
	// against their accumulated baseline at corrected significance.
	Catastrophic []string
	// CatastrophicEligible marks which stable-band trajectories had a
	// baseline deep enough that a 0/N could fire — the rest are
	// characterization, not protection.
	CatastrophicEligible map[string]bool
	// DiffuseP is the corpus-level permutation p-value over mid +
	// uncharacterized bands.
	DiffuseP float64
	// ExcludedDifferential lists trajectories whose treatment arm
	// produced significantly more inconclusive/error outcomes than
	// control — mechanism flags aren't orthogonal to exclusion by
	// construction, so a differential is a first-class alarm.
	ExcludedDifferential []string
	// Starved trajectories exhausted their attempts cap on
	// inconclusive; Saturated exhausted on error. Alarm labels on the
	// summary, not persisted trajectory states.
	Starved   []string
	Saturated []string
	// Coincident lists trajectories where treatment AND the current
	// control arm both collapsed against baseline — suspect
	// trajectory rot or model drift, not the change under test.
	Coincident []string
	// Smoke lists stable-band trajectories showing the strict 0/N
	// collapse pattern — used by the smoke tier, which must work
	// where baselines are thin. In a normal experiment report it is
	// still a gate: an ineligible trajectory collapsing to 0/N is the
	// detector working, not a false alarm (p̂≈0.9 → P(0/3|null)≈1e-3).
	Smoke []string
	// Skipped lists trajectories the experiment didn't run —
	// requires pre-flight rejects ("id: tool:go os:linux") or
	// bands absent from runs_per_trajectory. Environment rot and
	// config gaps made explicit instead of error outcomes or
	// silence.
	Skipped []string
	// NoopFlags lists trajectories whose arms resolved to identical
	// flag projections — the flag under test did nothing and the
	// pairing is a guaranteed null. Fails closed.
	NoopFlags []string
}

// Fired reports whether any alarm tripped — including the diffuse
// tier's corpus-level p against alpha.
func (r Report) Fired(alpha float64) bool {
	return r.DiffuseP < alpha ||
		len(r.Catastrophic) > 0 || len(r.ExcludedDifferential) > 0 ||
		len(r.Starved) > 0 || len(r.Saturated) > 0 || len(r.Smoke) > 0 ||
		len(r.Coincident) > 0 ||
		len(r.Skipped) > 0 || // Corpus shrinkage is an alarm.
		len(r.NoopFlags) > 0
}

// Summary renders the report for the CLI.
func (r Report) Summary(alpha float64) string {
	var b strings.Builder
	fire := func(name string, items []string) {
		if len(items) > 0 {
			fmt.Fprintf(&b, "  ALARM %s: %s\n", name, strings.Join(items, ", "))
		}
	}
	fire("catastrophic", r.Catastrophic)
	fire("excluded-differential", r.ExcludedDifferential)
	fire("coverage-starved", r.Starved)
	fire("error-saturated", r.Saturated)
	fire("smoke", r.Smoke)
	fire("coincident-collapse", r.Coincident)
	fire("noop-flag", r.NoopFlags)
	if len(r.Skipped) > 0 {
		fmt.Fprintf(&b, "  SKIP: %s\n", strings.Join(r.Skipped, ", "))
	}
	eligible := 0
	for _, ok := range r.CatastrophicEligible {
		if ok {
			eligible++
		}
	}
	fmt.Fprintf(&b, "  catastrophic coverage: %d/%d stable trajectories eligible\n", eligible, len(r.CatastrophicEligible))
	fmt.Fprintf(&b, "  diffuse p = %.4g (alpha %.3g)\n", r.DiffuseP, alpha)
	if !r.Fired(alpha) {
		b.WriteString("  verdict: PASS\n")
	} else {
		b.WriteString("  verdict: FAIL\n")
	}
	return b.String()
}

// Evaluate runs the gate over one experiment's records. bands is the
// frozen experiment-start snapshot; baselineKey is the experiment's
// baseline condition (control arm's effective-config hash); alpha is
// the per-family significance level before correction.
func Evaluate(exp *Experiment, bands *Bands, baselineKey string, records []RunRecord, corpusIDs map[string]bool, alpha float64, replicates int, rng *rand.Rand) Report {
	rep := Report{CatastrophicEligible: map[string]bool{}, DiffuseP: 1}

	byTraj := map[string][]RunRecord{}
	for _, r := range records {
		byTraj[r.TrajectoryID] = append(byTraj[r.TrajectoryID], r)
	}

	stable := stableBandSize(bands, corpusIDs)
	if stable == 0 {
		stable = 1
	}
	corrAlpha := alpha / float64(stable) // Bonferroni over the FULL stable band.

	var diffusePairs []ArmPair
	var exclTraj []string
	var exclP []float64

	for id, recs := range byTraj {
		ctrl := conclusiveByArm(recs, ArmControl)
		treat := conclusiveByArm(recs, ArmTreatment)

		// No-op detection: if both arms' latest resolved projections
		// are identical, the flag under test did nothing — the null
		// is guaranteed and the verdict is meaningless. Arms whose
		// runs never reported telemetry (crashed/fake drivers) carry
		// no projection and are skipped — the alarm is telemetry-
		// dependent by design.
		var ctrlRes, treatRes map[string]any
		for _, r := range recs {
			// Records append chronologically — last non-empty wins.
			if len(r.ResolvedOptions) > 0 {
				if r.Arm == ArmControl {
					ctrlRes = r.ResolvedOptions
				}
				if r.Arm == ArmTreatment {
					treatRes = r.ResolvedOptions
				}
			}
		}
		if ctrlRes != nil && treatRes != nil && resolvedEqual(ctrlRes, treatRes) {
			rep.NoopFlags = append(rep.NoopFlags, id)
		}

		switch bands.Band(id) {
		case BandStable:
			// Baselines are keyed on the resolved model observed in
			// the runs, not the experiment's spelling — an alias or
			// normalization difference must not silently darken the
			// catastrophic tier. Warn on divergence.
			model := exp.Model
			for _, rec := range recs {
				if rec.Env.ModelResolved != "" {
					model = rec.Env.ModelResolved
					break
				}
			}
			if model != exp.Model {
				slog.Warn("Resolved model differs from experiment pin; baselines keyed on resolved",
					"trajectory", id, "resolved", model, "pin", exp.Model)
			}
			base := bands.Baseline(id, model, baselineKey)
			baseFails := base.N - base.Passes
			// Coincidence detector: a control arm that collapses
			// against the same baseline signals trajectory rot or
			// model drift — alarming regardless of the treatment
			// outcome, and it disqualifies the catastrophic verdict
			// (the collapse isn't attributable to the flag). Checked
			// before the treatment-emptiness bail: an all-excluded
			// treatment arm doesn't excuse a rotted control.
			ctrlFails := 0
			for _, ok := range ctrl {
				if !ok {
					ctrlFails++
				}
			}
			ctrlCollapsed := len(ctrl) > 0 && base.N > 0 &&
				FisherExactCollapse(ctrlFails, len(ctrl), baseFails, base.N) < corrAlpha
			if ctrlCollapsed {
				rep.Coincident = append(rep.Coincident, fmt.Sprintf("%s (control %d/%d vs baseline %d/%d)", id, len(ctrl)-ctrlFails, len(ctrl), base.Passes, base.N))
			}
			n := len(treat)
			if n == 0 {
				continue
			}
			// Eligibility is computed, not a fixed n: the trajectory
			// qualifies when a 0/N result would reach corrected
			// significance against its baseline.
			pMin := FisherExactCollapse(n, n, baseFails, base.N)
			eligible := pMin < corrAlpha
			rep.CatastrophicEligible[id] = eligible
			if eligible {
				fails := 0
				for _, ok := range treat {
					if !ok {
						fails++
					}
				}
				if p := FisherExactCollapse(fails, n, baseFails, base.N); p < corrAlpha && !ctrlCollapsed {
					rep.Catastrophic = append(rep.Catastrophic, fmt.Sprintf("%s (%d/%d vs baseline %d/%d, p=%.2g)", id, n-fails, n, base.Passes, base.N, p))
				}
			}
			// Smoke pattern: strict 0/N, no baseline required.
			if n > 0 && allFailed(treat) {
				rep.Smoke = append(rep.Smoke, id)
			}

		case BandMid, BandUncharacterized:
			if len(ctrl) > 0 && len(treat) > 0 {
				diffusePairs = append(diffusePairs, ArmPair{Control: ctrl, Treatment: treat})
			}
		}

		// Excluded-class differential, per trajectory: Fisher on
		// excluded counts between arms.
		et, ec, tt, tc := excludedCounts(recs)
		if tt+tc > 0 {
			exclTraj = append(exclTraj, id)
			exclP = append(exclP, FisherExactCollapse(et, tt, ec, tc))
		}
	}

	if len(diffusePairs) > 0 {
		rep.DiffuseP = PermutationP(diffusePairs, replicates, rng)
	}

	// Third family: Bonferroni over the trajectories actually tested.
	if len(exclTraj) > 0 {
		corr := alpha / float64(len(exclTraj))
		for i, p := range exclP {
			if p < corr {
				rep.ExcludedDifferential = append(rep.ExcludedDifferential, exclTraj[i])
			}
		}
	}

	sort.Strings(rep.Catastrophic)
	sort.Strings(rep.ExcludedDifferential)
	sort.Strings(rep.Smoke)
	return rep
}

// stableBandSize counts the live corpus's stable band for the
// Bonferroni denominator — correcting across only eligible
// trajectories would be
// circular, since eligibility is defined by reaching that alpha.
// stableBandSize counts stable-band members of the live corpus —
// entries for trajectories removed from the corpus must not inflate
// the Bonferroni denominator.
func stableBandSize(bands *Bands, corpusIDs map[string]bool) int {
	n := 0
	for id, e := range bands.Entries {
		if e.Band == BandStable && corpusIDs[id] {
			n++
		}
	}
	return n
}

// conclusiveByArm returns pass/fail bits (timeout counts as fail — a
// real failure mode) for one arm's conclusive runs.
func conclusiveByArm(recs []RunRecord, arm string) []bool {
	var out []bool
	for _, r := range recs {
		if r.Arm != arm || !r.Outcome.Conclusive() {
			continue
		}
		out = append(out, r.Outcome == OutcomePass)
	}
	return out
}

// excludedCounts returns (treatExcluded, ctrlExcluded, treatAttempts,
// ctrlAttempts) over inconclusive+error vs all attempts.
func excludedCounts(recs []RunRecord) (et, ec, tt, tc int) {
	for _, r := range recs {
		excluded := r.Outcome == OutcomeInconclusive || r.Outcome == OutcomeError
		switch r.Arm {
		case ArmTreatment:
			tt++
			if excluded {
				et++
			}
		case ArmControl:
			tc++
			if excluded {
				ec++
			}
		}
	}
	return
}

func allFailed(bits []bool) bool {
	for _, b := range bits {
		if b {
			return false
		}
	}
	return len(bits) > 0
}
