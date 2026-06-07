package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
)

func TestModelMaxOutputTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    Model
		want int64
	}{
		{
			name: "ModelCfg override wins over catwalk default",
			m: Model{
				CatwalkCfg: catwalk.Model{DefaultMaxTokens: 4096},
				ModelCfg:   config.SelectedModel{MaxTokens: 32000},
			},
			want: 32000,
		},
		{
			name: "falls back to catwalk default when ModelCfg is zero",
			m: Model{
				CatwalkCfg: catwalk.Model{DefaultMaxTokens: 16384},
			},
			want: 16384,
		},
		{
			name: "returns zero when neither is configured",
			m:    Model{},
			want: 0,
		},
		{
			name: "explicit zero in ModelCfg does not override catwalk default",
			m: Model{
				CatwalkCfg: catwalk.Model{DefaultMaxTokens: 8192},
				ModelCfg:   config.SelectedModel{MaxTokens: 0},
			},
			want: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.m.MaxOutputTokens(); got != tt.want {
				t.Fatalf("MaxOutputTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}
