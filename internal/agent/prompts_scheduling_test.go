package agent

import (
	"strings"
	"testing"
	"text/template"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/stretchr/testify/require"
)

// renderCoderTemplate executes the embedded coder template directly with the
// given data — no ConfigStore, no filesystem discovery — so the test is
// hermetic and fast.
func renderCoderTemplate(t *testing.T, dat prompt.PromptDat) string {
	t.Helper()
	tpl, err := template.New("coder").Parse(string(coderPromptTmpl))
	require.NoError(t, err)
	var b strings.Builder
	require.NoError(t, tpl.Execute(&b, dat))
	return b.String()
}

// Scheduling section gate
//
// The <scheduling> section points the model at CronCreate / CronList /
// CronDelete. Guidance follows the tool, so it is gated the same way the
// A2UI section is: the coordinator turns it on because it registers the
// cron tools, and callers that do not — the recorded agent tests among
// them — leave it off.
//
// That gate is also what keeps the VCR cassettes byte-stable. The system
// prompt is part of every recorded request body, so an ungated section
// invalidates all 13 cassettes at once and the whole agent package goes
// red with "requested interaction not found".
//
// @joestump 08/23/2026 - Added with the gate, after the ungated version
// of this section broke every cassette on PR #274.

func TestCoderPromptSchedulingGate(t *testing.T) {
	t.Parallel()

	off := renderCoderTemplate(t, prompt.PromptDat{})
	require.NotContains(t, off, "<scheduling>")
	require.NotContains(t, off, "CronCreate")

	on := renderCoderTemplate(t, prompt.PromptDat{Scheduling: true})
	require.Contains(t, on, "<scheduling>")
	require.Contains(t, on, "CronCreate / CronList / CronDelete")
}

// With the section off, the rendered prompt must be byte-identical to what
// it was before the section existed — a stray blank line left behind by the
// {{if}} is still a cassette-breaking diff, and it is invisible in review.
//
// Newlines are normalized first: the template is checked out CRLF on
// Windows, so an \n-only assertion fails there for a reason that has
// nothing to do with the gate.
func TestCoderPromptSchedulingOffLeavesNoWhitespace(t *testing.T) {
	t.Parallel()

	off := strings.ReplaceAll(renderCoderTemplate(t, prompt.PromptDat{}), "\r\n", "\n")
	require.Contains(t, off, "</bash_commands>\n</tool_usage>",
		"the disabled scheduling gate left whitespace between the sections")
}
