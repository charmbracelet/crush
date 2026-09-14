package eval

import (
	"math/big"
	"math/rand/v2"
)

// FisherExactCollapse is the one-sided Fisher exact test the
// catastrophic tier runs: current arm with failsCur failures in nCur
// runs vs an accumulated baseline with failsBase failures in nBase
// runs. Returns the hypergeometric probability of the current arm
// drawing failsCur or more failures, given the margins — i.e., the
// p-value for "the current arm collapsed against baseline".
//
// Exact counts via math/big; the cells here are small (arm N ≤ ~20,
// baseline n ≤ few hundred) so precision never matters.
func FisherExactCollapse(failsCur, nCur, failsBase, nBase int) float64 {
	totalFails := failsCur + failsBase
	totalPass := (nCur - failsCur) + (nBase - failsBase)
	total := nCur + nBase

	// P(X = k) = C(totalFails, k) · C(totalPass, nCur−k) / C(total, nCur).
	denom := binom(total, nCur)
	var p big.Float
	p.SetInt64(0)
	for k := failsCur; k <= min(totalFails, nCur); k++ {
		num := new(big.Int).Mul(binom(totalFails, k), binom(totalPass, nCur-k))
		var term big.Float
		term.SetInt(num)
		term.Quo(&term, new(big.Float).SetInt(denom))
		p.Add(&p, &term)
	}
	f, _ := p.Float64()
	return f
}

// binom returns C(n, k) as a big.Int.
func binom(n, k int) *big.Int {
	if k < 0 || k > n {
		return big.NewInt(0)
	}
	return new(big.Int).Binomial(int64(n), int64(k))
}

// ArmPair is one trajectory's conclusive outcomes per arm, coded 1=pass
// 0=fail/timeout for the rate estimate.
type ArmPair struct {
	Control   []bool
	Treatment []bool
}

// PermutationP is the diffuse-tier corpus test. The corpus statistic is
// the mean of per-trajectory d_t = p̂_treat − p̂_ctrl. Each replicate
// independently re-splits ALL trajectories' pooled runs into arms of
// the observed sizes — runs are exchangeable under the null, no seeds
// needed — and recomputes the statistic. One-sided: returns the share
// of replicates with mean-d ≤ observed (detecting degradation).
func PermutationP(pairs []ArmPair, replicates int, rng *rand.Rand) float64 {
	if len(pairs) == 0 {
		return 1
	}
	observed := meanDiff(pairs)
	var extreme int
	for range replicates {
		if meanDiff(reshuffle(pairs, rng)) <= observed {
			extreme++
		}
	}
	// Add-one smoothing: the observed split is itself a legal
	// replicate, so the estimate never reports p=0.
	return float64(extreme+1) / float64(replicates+1)
}

// meanDiff computes the corpus statistic: mean over trajectories of
// p̂_treat − p̂_ctrl.
func meanDiff(pairs []ArmPair) float64 {
	var sum float64
	for _, p := range pairs {
		sum += rate(p.Treatment) - rate(p.Control)
	}
	return sum / float64(len(pairs))
}

// reshuffle produces one replicate: within each trajectory, pool both
// arms' runs and re-split into arms of the observed sizes.
func reshuffle(pairs []ArmPair, rng *rand.Rand) []ArmPair {
	out := make([]ArmPair, len(pairs))
	for i, p := range pairs {
		pool := make([]bool, 0, len(p.Control)+len(p.Treatment))
		pool = append(pool, p.Control...)
		pool = append(pool, p.Treatment...)
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		out[i].Control = pool[:len(p.Control)]
		out[i].Treatment = pool[len(p.Control):]
	}
	return out
}

func rate(s []bool) float64 {
	if len(s) == 0 {
		return 0
	}
	var n int
	for _, v := range s {
		if v {
			n++
		}
	}
	return float64(n) / float64(len(s))
}
