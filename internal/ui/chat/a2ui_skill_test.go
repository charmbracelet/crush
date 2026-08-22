package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/joestump-agent/a2tea"
	"github.com/stretchr/testify/require"
)

// TestA2UISkillExampleRenders guards the builtin a2ui skill's documented
// wire-format example against drift: every real <a2ui-json> payload embedded
// in SKILL.md must render through the pinned a2tea as actual content, not an
// "[a2tea: ...]" placeholder. This caught the Button using a nonexistent
// "label" field (a2ui Buttons take a child Text ID); if the doc regresses to
// a payload the renderer can't draw, this fails instead of shipping a broken
// reference to the model.
func TestA2UISkillExampleRenders(t *testing.T) {
	t.Parallel()

	var instructions string
	for _, s := range skills.DiscoverBuiltin() {
		if s.Name == "a2ui" {
			instructions = s.Instructions
			break
		}
	}
	require.NotEmpty(t, instructions, "a2ui builtin skill not found")

	// SKILL.md mentions <a2ui-json>{...}</a2ui-json> inline as a placeholder as
	// well as carrying the real example; render only the payloads that
	// actually describe a surface (an updateComponents message).
	blocks := extractA2UIBlocks(instructions)
	var rendered int
	var all strings.Builder
	for _, block := range blocks {
		if !strings.Contains(block, "updateComponents") {
			continue
		}
		parts, err := a2tea.Scan(block)
		require.NoError(t, err, "documented example must parse as A2UI")
		for _, p := range parts {
			if len(p.Messages) == 0 {
				continue
			}
			m, err := a2tea.Render(p.Messages)
			require.NoError(t, err, "documented example must render")
			all.WriteString(m.View().Content)
			rendered++
		}
	}
	require.Positive(t, rendered, "SKILL.md must contain a renderable <a2ui-json> example")

	out := all.String()
	require.NotContains(t, out, "[a2tea:",
		"documented example rendered a placeholder (broken wire format): %q", out)
	// The example's button label and body text must survive to the surface —
	// the button label in particular only renders if it uses a child Text ID.
	require.Contains(t, out, "Acknowledge")
	require.Contains(t, out, "Build passed")
}

// extractA2UIBlocks returns every <a2ui-json>...</a2ui-json> span (tags
// included) found in s, in order.
func extractA2UIBlocks(s string) []string {
	const open, close = "<a2ui-json>", "</a2ui-json>"
	var out []string
	for {
		i := strings.Index(s, open)
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], close)
		if j < 0 {
			break
		}
		end := i + j + len(close)
		out = append(out, s[i:end])
		s = s[end:]
	}
	return out
}
