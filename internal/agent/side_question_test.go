package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// stubLanguageModel is a non-nil fantasy.LanguageModel marker for unit
// tests that only need to distinguish present vs absent models.
type stubLanguageModel struct {
	fantasy.LanguageModel
}

func TestSideQuestionAttemptsOrder(t *testing.T) {
	t.Parallel()

	small := Model{
		Model: stubLanguageModel{},
		CatwalkCfg: catwalk.Model{
			ContextWindow: 1000,
		},
	}
	large := Model{
		Model: stubLanguageModel{},
		CatwalkCfg: catwalk.Model{
			ContextWindow: 100_000,
		},
	}

	t.Run("fits small uses small then large", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(small, large, 100, 100)
		require.Len(t, got, 2)
		require.Equal(t, small, got[0])
		require.Equal(t, large, got[1])
	})

	t.Run("over 80 percent uses large only", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(small, large, 700, 200)
		require.Len(t, got, 1)
		require.Equal(t, large, got[0])
	})

	t.Run("exactly 80 percent still prefers small", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(small, large, 700, 100)
		require.Len(t, got, 2)
		require.Equal(t, small, got[0])
	})

	t.Run("nil small falls back to large", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(Model{}, large, 0, 0)
		require.Len(t, got, 1)
		require.Equal(t, large, got[0])
	})

	t.Run("both nil returns empty", func(t *testing.T) {
		t.Parallel()
		got := sideQuestionAttempts(Model{}, Model{}, 0, 0)
		require.Empty(t, got)
	})
}
