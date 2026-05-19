package critic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeedbackCache_HitMiss(t *testing.T) {
	t.Parallel()
	fc, err := NewFeedbackCache(10)
	require.NoError(t, err)

	cp := Checkpoint{PrimaryDiff: "diff1", LSPDiagnostics: nil}
	_, hit := fc.Get(cp)
	require.False(t, hit)

	fb := &CriticFeedback{Verdict: "approve", Confidence: 0.9}
	fc.Put(cp, fb)

	got, hit := fc.Get(cp)
	require.True(t, hit)
	require.Equal(t, fb.Verdict, got.Verdict)
}

func TestFeedbackCache_DifferentKeys(t *testing.T) {
	t.Parallel()
	fc, err := NewFeedbackCache(10)
	require.NoError(t, err)

	cp1 := Checkpoint{PrimaryDiff: "diffA"}
	cp2 := Checkpoint{PrimaryDiff: "diffB"}

	fc.Put(cp1, &CriticFeedback{Verdict: "approve"})
	fc.Put(cp2, &CriticFeedback{Verdict: "halt"})

	got1, _ := fc.Get(cp1)
	require.Equal(t, "approve", got1.Verdict)

	got2, _ := fc.Get(cp2)
	require.Equal(t, "halt", got2.Verdict)
}

func TestFeedbackCache_Eviction(t *testing.T) {
	t.Parallel()
	fc, err := NewFeedbackCache(2)
	require.NoError(t, err)

	fc.Put(Checkpoint{PrimaryDiff: "a"}, &CriticFeedback{Verdict: "a"})
	fc.Put(Checkpoint{PrimaryDiff: "b"}, &CriticFeedback{Verdict: "b"})
	fc.Put(Checkpoint{PrimaryDiff: "c"}, &CriticFeedback{Verdict: "c"})

	_, hit := fc.Get(Checkpoint{PrimaryDiff: "a"})
	require.False(t, hit)
}

func TestFeedbackCache_Stats(t *testing.T) {
	t.Parallel()
	fc, err := NewFeedbackCache(2)
	require.NoError(t, err)

	cp1 := Checkpoint{PrimaryDiff: "a"}
	cp2 := Checkpoint{PrimaryDiff: "b"}

	fc.Put(cp1, &CriticFeedback{Verdict: "approve"})
	_, ok := fc.Get(cp1)
	require.True(t, ok)

	_, ok = fc.Get(cp2)
	require.False(t, ok)

	hits, misses := fc.Stats()
	require.Equal(t, 1, hits)
	require.Equal(t, 1, misses)
}
