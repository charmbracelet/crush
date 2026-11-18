package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasTerminal(t *testing.T) {
	t.Parallel()

	const stdin, stdout uintptr = 0, 1
	tests := []struct {
		name      string
		terminals map[uintptr]bool
		want      bool
	}{
		{"both terminals", map[uintptr]bool{stdin: true, stdout: true}, true},
		{"piped stdin", map[uintptr]bool{stdout: true}, true},
		{"redirected stdout", map[uintptr]bool{stdin: true}, true},
		{"no terminal", map[uintptr]bool{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isTerminal := func(fd uintptr) bool { return tt.terminals[fd] }
			require.Equal(t, tt.want, hasTerminal(isTerminal, stdin, stdout))
		})
	}
}
