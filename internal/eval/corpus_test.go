package eval

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorpus_QuarantineClean(t *testing.T) {
	evalDir, err := filepath.Abs(filepath.Join("..", "..", "eval"))
	require.NoError(t, err)
	corpus, err := LoadCorpus(evalDir)
	require.NoError(t, err)
	require.NotEmpty(t, corpus)

	for id, tr := range corpus {
		tr := tr
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			// Quarantine doesn't consult requires — a host missing a
			// declared tool would report every state as broken rather
			// than judging the check, so skip instead of failing.
			if missing := CheckRequires(tr); len(missing) > 0 {
				t.Skipf("missing requirements: %v", missing)
			}
			r := &Runner{EvalDir: evalDir, QuarantineRepeats: 2, WorkParent: t.TempDir()}
			t.Cleanup(r.Close)
			dir := filepath.Join(evalDir, "corpus", id)
			reason, err := r.Quarantine(context.Background(), tr, dir)
			require.NoError(t, err)
			// A rotted seed must fail CI, not just log.
			require.Empty(t, reason, "trajectory %s quarantined: %s", id, reason)
		})
	}
}
