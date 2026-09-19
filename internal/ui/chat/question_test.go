package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestStyleAnswer(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{"yes", "User answered: yes", "Yes"},
		{"no", "User answered: no", "No"},
		{"skipped", "User skipped this question", "Skipped"},
		{"provided", "User provided: iphone", "iphone"},
		{"selected", `User selected: ["pi5"]`, "pi5"},
		{
			"selected and provided",
			"User selected: [\"pg\"]\nUser provided: with pgbouncer",
			"pg, with pgbouncer",
		},
		{
			"multiline free text",
			"User provided: Everseen\neverything",
			"Everseen everything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := styleAnswer(&sty, tt.answer)
			require.Equal(t, tt.want, ansi.Strip(got))
			for _, line := range strings.Split(got, ", ") {
				require.NotEqual(t, ansi.Strip(line), line,
					"answer segment rendered without styling")
			}
		})
	}
}
