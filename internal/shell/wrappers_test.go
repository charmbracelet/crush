package shell

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeelWrapper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []string
		want    []string
		wrapped bool
	}{
		{"not a wrapper", []string{"ls", "-la"}, nil, false},
		{"nohup", []string{"nohup", "ls", "-la"}, []string{"ls", "-la"}, true},
		{"timeout skips duration", []string{"timeout", "5", "ls"}, []string{"ls"}, true},
		{"timeout with value flag", []string{"timeout", "-s", "TERM", "5", "ls"}, []string{"ls"}, true},
		{"timeout with equals flag", []string{"timeout", "--signal=TERM", "5", "ls"}, []string{"ls"}, true},
		{"nice with adjustment", []string{"nice", "-n", "10", "ls"}, []string{"ls"}, true},
		// Assignments are not skipped: the assignment stays at the head of
		// the inner argv, where it matches no entry and so fails closed.
		{"env with assignment", []string{"env", "FOO=bar", "ls"}, []string{"FOO=bar", "ls"}, true},
		{"env with multiple assignments", []string{"env", "A=1", "B=2", "ls", "-l"}, []string{"A=1", "B=2", "ls", "-l"}, true},
		{"env alone is not wrapping", []string{"env"}, nil, false},
		{"env with only flags is not wrapping", []string{"env", "-i"}, nil, false},
		{"double dash", []string{"nohup", "--", "ls"}, []string{"ls"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := peelWrapper(tt.input)
			require.Equal(t, tt.wrapped, ok, "peelWrapper(%v) wrapped", tt.input)
			if tt.wrapped {
				require.Equal(t, tt.want, got, "peelWrapper(%v)", tt.input)
			}
		})
	}
}

// ResolveArgv is what both the safe list and the deny list consult, so the
// two cannot disagree about which command an argv actually runs.
func TestResolveArgv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"plain command resolves to itself", []string{"ls", "-la"}, []string{"ls", "-la"}},
		{"single wrapper", []string{"nice", "curl", "x"}, []string{"curl", "x"}},
		{"nested wrappers", []string{"nohup", "nice", "curl"}, []string{"curl"}},
		{"wrapper with operand", []string{"timeout", "5", "sudo", "rm"}, []string{"sudo", "rm"}},
		{"wrapper name folds case", []string{"NICE", "curl"}, []string{"curl"}},
		{"bare wrapper resolves to itself", []string{"env"}, []string{"env"}},
		{"empty argv", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ResolveArgv(tt.input))
		})
	}
}
