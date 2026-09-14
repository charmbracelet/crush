package eval

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleCorpus_QuarantineClean(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "eval", "corpus", "fix-nil-map-write"))
	if err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTrajectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{EvalDir: "../..", QuarantineRepeats: 2, WorkParent: t.TempDir()}
	t.Cleanup(r.Close)
	reason, err := r.Quarantine(context.Background(), tr, dir)
	if err != nil {
		t.Fatal(err)
	}
	// A rotted seed must fail CI, not just log — caveat: its declared
	// requires.tools only pre-flight on machines that have them.
	require.Empty(t, reason, "seeded trajectory quarantined: %s", reason)
}
