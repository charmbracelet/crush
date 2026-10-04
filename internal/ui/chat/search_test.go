package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestGrepToolMessageItem_PendingShowsSearchParameters guards that a running
// grep renders its search parameters next to the spinner instead of hiding
// them behind a bare "Grep" label.
func TestGrepToolMessageItem_PendingShowsSearchParameters(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GrepToolRenderContext{}
	out := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{
			ID:       "grep1",
			Name:     "grep",
			Input:    `{"pattern":"A9041","path":".","include":"*.go","literal_text":true}`,
			Finished: false,
		},
		Status: ToolStatusRunning,
	}))

	require.Contains(t, out, "Grep")
	require.Contains(t, out, "A9041")
	require.Contains(t, out, "path=.")
	require.Contains(t, out, "include=*.go")
	require.Contains(t, out, "literal=true")
	require.NotContains(t, out, "Invalid parameters")
}

// TestGrepToolMessageItem_PendingPartialInputFallsBackToSpinner guards that
// while the tool call input is still streaming and no field usable for the
// header has completed yet, the pending state falls back to the plain
// spinner instead of flashing an "Invalid parameters" error.
func TestGrepToolMessageItem_PendingPartialInputFallsBackToSpinner(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GrepToolRenderContext{}
	streamingPrefixes := []string{"", "{", `{"pattern"`, `{"pattern":`, `{"pattern":"fo`}

	for _, compact := range []bool{false, true} {
		for _, input := range streamingPrefixes {
			a := anim.New(anim.Settings{ID: "grep-anim", Label: "Working"})
			out := ctx.RenderTool(&sty, 120, &ToolRenderOpts{
				ToolCall: message.ToolCall{
					ID:       "grep2",
					Name:     "grep",
					Input:    input,
					Finished: false,
				},
				Status:  ToolStatusRunning,
				Anim:    a,
				Compact: compact,
			})
			require.Equal(t, pendingTool(&sty, "Grep", a, compact), out,
				"pending grep with partial input %q must render the plain spinner (compact=%v)", input, compact)
		}
	}
}

// TestGrepToolMessageItem_PendingPartialInputShowsCompleteFields guards that
// fields whose JSON value already closed are rendered while the rest of the
// input is still streaming, so the search context appears progressively.
func TestGrepToolMessageItem_PendingPartialInputShowsCompleteFields(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GrepToolRenderContext{}

	tests := []struct {
		name     string
		input    string
		contains []string
		missing  []string
	}{
		{
			name:     "pattern closed, object not",
			input:    `{"pattern":"fo"`,
			contains: []string{"fo"},
			missing:  []string{"path=", "include=", "literal="},
		},
		{
			name:     "path closed, object not",
			input:    `{"pattern":"foo","path":"."`,
			contains: []string{"foo", "path=."},
			missing:  []string{"include=", "literal="},
		},
		{
			name:     "all fields closed, closing brace missing",
			input:    `{"pattern":"foo","path":".","include":"*.go","literal_text":true`,
			contains: []string{"foo", "path=.", "include=*.go", "literal=true"},
			missing:  []string{},
		},
		{
			name:     "wrongly typed pattern shows no header at all",
			input:    `{"pattern":123,"path":"."`,
			contains: []string{"Grep"},
			missing:  []string{"123", "path="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
				ToolCall: message.ToolCall{
					ID:       "grep5",
					Name:     "grep",
					Input:    tt.input,
					Finished: false,
				},
				Status: ToolStatusRunning,
			}))

			for _, want := range tt.contains {
				require.Contains(t, out, want)
			}
			for _, notWant := range tt.missing {
				require.NotContains(t, out, notWant)
			}
			require.NotContains(t, out, "Invalid parameters")
		})
	}
}

// TestGrepToolMessageItem_PendingHeaderMatchesCompletedHeader guards that the
// pending state renders the very same header as the completed state, so
// nested (compact) grep calls keep their styling while loading.
func TestGrepToolMessageItem_PendingHeaderMatchesCompletedHeader(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GrepToolRenderContext{}
	tc := message.ToolCall{
		ID:    "grep3",
		Name:  "grep",
		Input: `{"pattern":"foo","path":"."}`,
	}
	opts := func(finished bool) *ToolRenderOpts {
		tc := tc
		tc.Finished = finished
		return &ToolRenderOpts{
			ToolCall: tc,
			Status:   ToolStatusRunning,
			Compact:  true,
		}
	}

	a := anim.New(anim.Settings{ID: "grep-anim", Label: "Working"})
	pendingOpts := opts(false)
	pendingOpts.Anim = a
	pending := ctx.RenderTool(&sty, 120, pendingOpts)
	completed := ctx.RenderTool(&sty, 120, opts(true))

	require.Equal(t, completed+" "+a.Render(), pending,
		"pending compact grep must reuse the completed (nested-styled) header plus the spinner")
}

// TestGrepToolMessageItem_InvalidInputStillErrorsWhenFinished guards that the
// fallback only applies to pending calls: once a call is finished, invalid
// JSON still renders the "Invalid parameters" error.
func TestGrepToolMessageItem_InvalidInputStillErrorsWhenFinished(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &GrepToolRenderContext{}
	out := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: message.ToolCall{
			ID:       "grep4",
			Name:     "grep",
			Input:    `{"pattern":"fo`,
			Finished: true,
		},
		Status: ToolStatusError,
	}))

	require.Contains(t, out, "Invalid parameters")
}
