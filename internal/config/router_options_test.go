package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRouterOptions_DefaultsWhenNil(t *testing.T) {
	t.Parallel()

	var opts *RouterOptions
	require.InDelta(t, 0.7, opts.EffectiveConfidenceThreshold(), 0.0001)
	require.Equal(t, 1500*time.Millisecond, opts.EffectiveTimeout())
}

func TestRouterOptions_DefaultsWhenZero(t *testing.T) {
	t.Parallel()

	opts := &RouterOptions{}
	require.InDelta(t, 0.7, opts.EffectiveConfidenceThreshold(), 0.0001)
	require.Equal(t, 1500*time.Millisecond, opts.EffectiveTimeout())
}

func TestRouterOptions_ExplicitValues(t *testing.T) {
	t.Parallel()

	opts := &RouterOptions{ConfidenceThreshold: 0.9, TimeoutMS: 500}
	require.InDelta(t, 0.9, opts.EffectiveConfidenceThreshold(), 0.0001)
	require.Equal(t, 500*time.Millisecond, opts.EffectiveTimeout())
}

// TestRouterOptions_EffectiveMinModelConfidence_ComputesAutomaticFloor
// proves the unconfigured (zero-value) case computes 1/poolSize rather
// than a fixed number — so the floor stays "better than picking blindly"
// regardless of how many models the pool holds, without needing manual
// retuning every time the pool is resized.
func TestRouterOptions_EffectiveMinModelConfidence_ComputesAutomaticFloor(t *testing.T) {
	t.Parallel()

	var nilOpts *RouterOptions
	require.InDelta(t, 1.0/3, nilOpts.EffectiveMinModelConfidence(3), 0.0001)

	zeroOpts := &RouterOptions{}
	require.InDelta(t, 0.5, zeroOpts.EffectiveMinModelConfidence(2), 0.0001)
}

// TestRouterOptions_EffectiveMinModelConfidence_ExplicitValueWins proves
// an explicitly configured floor overrides the automatic 1/poolSize
// computation, so a backend with a known, non-random bias (e.g.
// consistently ~40% confident but never actually right) can be tuned by
// hand instead of only ever getting the generic floor.
func TestRouterOptions_EffectiveMinModelConfidence_ExplicitValueWins(t *testing.T) {
	t.Parallel()

	opts := &RouterOptions{MinModelConfidence: 0.9}
	require.InDelta(t, 0.9, opts.EffectiveMinModelConfidence(3), 0.0001)
}

// TestRouterOptions_EffectiveMinModelConfidence_EmptyPoolNeverApplies
// proves a pool size of 0 (or negative, defensively) returns a floor of
// 1 — impossible for any confidence value to clear — since there is no
// pool to pick a model from at all in that case.
func TestRouterOptions_EffectiveMinModelConfidence_EmptyPoolNeverApplies(t *testing.T) {
	t.Parallel()

	opts := &RouterOptions{}
	require.Equal(t, 1.0, opts.EffectiveMinModelConfidence(0))
}

func TestRouterOptions_ModelPoolDefaultsToEmpty(t *testing.T) {
	t.Parallel()

	opts := &RouterOptions{}
	require.Empty(t, opts.ModelPool)
}

func TestRouterOptions_ModelPoolRoundTripsThroughJSON(t *testing.T) {
	t.Parallel()

	opts := RouterOptions{ModelPool: []string{"anthropic/claude-opus-4", "anthropic/claude-haiku-4"}}
	data, err := json.Marshal(opts)
	require.NoError(t, err)

	var decoded RouterOptions
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, opts.ModelPool, decoded.ModelPool)
}
