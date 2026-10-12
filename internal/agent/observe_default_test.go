package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ledger"
	"github.com/stretchr/testify/require"
)

// presentLedger stands in for a configured ledger service. observesMemory only
// asks whether one exists, so an embedded interface with nothing behind it is
// the honest stub; giving it methods would claim more than the test needs.
type presentLedger struct{ ledger.Service }

// TestObservesMemoryDefaultsOn pins the default: recording is on unless the
// user turns it off. A config with no compaction block at all and a config with
// the option unset must agree, because the difference between them is whether
// the user has ever looked at the setting, not what they want.
func TestObservesMemoryDefaultsOn(t *testing.T) {
	t.Parallel()

	off := false
	on := true

	tests := []struct {
		name       string
		compaction *config.CompactionOptions
		hasLedger  bool
		subAgent   bool
		want       bool
	}{
		{
			name:      "no compaction block at all",
			hasLedger: true,
			want:      true,
		},
		{
			name:       "compaction block, option unset",
			compaction: &config.CompactionOptions{},
			hasLedger:  true,
			want:       true,
		},
		{
			name:       "explicitly on",
			compaction: &config.CompactionOptions{ObserveMemory: &on},
			hasLedger:  true,
			want:       true,
		},
		{
			name:       "explicitly off stays off",
			compaction: &config.CompactionOptions{ObserveMemory: &off},
			hasLedger:  true,
			want:       false,
		},
		{
			// The ledger is absent, not declined, so this is the build gate
			// being exercised rather than the config default.
			name:       "no ledger service, nowhere to record",
			compaction: &config.CompactionOptions{ObserveMemory: &on},
			want:       false,
		},
		{
			// A memory of a memory is the erosion the ledger exists to
			// prevent, so the sub-agent gate holds even against an explicit
			// opt-in.
			name:       "sub-agents never record",
			compaction: &config.CompactionOptions{ObserveMemory: &on},
			hasLedger:  true,
			subAgent:   true,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := config.NewTestStore(&config.Config{
				Options: &config.Options{Compaction: tt.compaction},
			})
			c := &coordinator{cfg: store}
			if tt.hasLedger {
				c.ledger = presentLedger{}
			}

			require.Equal(t, tt.want, c.observesMemory(tt.subAgent))
		})
	}
}
