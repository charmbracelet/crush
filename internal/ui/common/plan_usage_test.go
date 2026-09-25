package common

import (
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

const openaiID = string(catwalk.InferenceProviderOpenAI)

// recordPlan installs a snapshot for the duration of a test.
func recordPlan(t *testing.T, u openai.Usage) {
	t.Helper()
	openai.RecordUsage(u)
	t.Cleanup(func() { openai.RecordUsage(openai.Usage{}) })
}

func spentPlan() openai.Usage {
	return openai.Usage{
		Windows: []openai.Window{
			{UsedPercent: 100, Length: 5 * time.Hour},
			{UsedPercent: 64, Length: 7 * 24 * time.Hour},
		},
	}
}

// Usage belongs to whichever provider is actually selected. Signing in to a
// ChatGPT plan and then working against another provider must not leave the
// plan's figures on screen, where they would look current.
func TestPlanUsageOnlyForTheSelectedProvider(t *testing.T) {
	recordPlan(t, spentPlan())

	_, ok := PlanUsageFor(openaiID)
	require.True(t, ok, "the selected provider's plan is reported")

	for _, other := range []string{
		string(catwalk.InferenceProviderAnthropic),
		"hyper",
		"",
	} {
		_, ok := PlanUsageFor(other)
		require.False(t, ok, "%q must not report the ChatGPT plan", other)
	}
}

func TestPlanUsageAbsentUntilReported(t *testing.T) {
	recordPlan(t, openai.Usage{})

	_, ok := PlanUsageFor(openaiID)
	require.False(t, ok, "no snapshot means no readout")
}

func TestPlanReadoutDetailNamesEachWindow(t *testing.T) {
	sty := styles.CharmtonePantera()
	recordPlan(t, spentPlan())

	plan, ok := PlanUsageFor(openaiID)
	require.True(t, ok)

	got := stripANSI(plan.Detail(sty.ModelInfo.PlanUsage))
	require.Contains(t, got, styles.ChatGPTUsageIcon)
	require.Contains(t, got, "100% 5h")
	require.Contains(t, got, "64% weekly")
}

// The header has no room for labels, so the windows read as one figure with a
// single percent sign: "100/64%", not "100%/64%".
func TestPlanReadoutCompactReadsAsOneFigure(t *testing.T) {
	sty := styles.CharmtonePantera()
	recordPlan(t, spentPlan())

	plan, ok := PlanUsageFor(openaiID)
	require.True(t, ok)

	require.Equal(t,
		styles.ChatGPTUsageIcon+" 100/64%",
		stripANSI(plan.Compact(sty.Header.PlanUsage)))
}

func TestPlanReadoutSaysWhenBlocked(t *testing.T) {
	sty := styles.CharmtonePantera()
	resets := time.Now().Add(42 * time.Minute)

	// A plan at 100% that is still being served must not claim to be
	// blocked: the provider keeps serving a spent window while credits or
	// the other window have room.
	t.Run("still serving", func(t *testing.T) {
		u := spentPlan()
		u.LimitReached = false
		recordPlan(t, u)

		plan, ok := PlanUsageFor(openaiID)
		require.True(t, ok)

		got := stripANSI(plan.Detail(sty.ModelInfo.PlanUsage))
		require.Contains(t, got, "100% 5h")
		require.NotContains(t, got, "blocked")
		require.NotContains(t, got, "limit reached")
	})

	t.Run("refused", func(t *testing.T) {
		u := spentPlan()
		u.LimitReached = true
		u.Windows[0].Resets = resets
		recordPlan(t, u)

		plan, ok := PlanUsageFor(openaiID)
		require.True(t, ok)

		got := stripANSI(plan.Detail(sty.ModelInfo.PlanUsage))
		require.Contains(t, got, "blocked until "+resets.Format("3:04pm"))
	})

	t.Run("refused with no refill time", func(t *testing.T) {
		u := spentPlan()
		u.LimitReached = true
		recordPlan(t, u)

		plan, ok := PlanUsageFor(openaiID)
		require.True(t, ok)

		got := stripANSI(plan.Detail(sty.ModelInfo.PlanUsage))
		require.Contains(t, got, "limit reached")
	})
}

func stripANSI(s string) string {
	var (
		b        strings.Builder
		inEscape bool
	)
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape && (r == 'm' || r == 'K'):
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Every plan icon has to measure one cell, or the header's width arithmetic
// drifts by a column wherever a readout is shown. None of these glyphs live in
// a typical monospace font, so they arrive by font fallback and their declared
// width is the only thing holding the layout together.
func TestPlanIconsAreOneCell(t *testing.T) {
	t.Parallel()

	for _, icon := range []struct{ name, glyph string }{
		{"claude", styles.ClaudeUsageIcon},
		{"chatgpt", styles.ChatGPTUsageIcon},
	} {
		require.Equal(t, 1, ansi.StringWidth(icon.glyph), "%s icon must be one cell", icon.name)
	}
}

// The ChatGPT mark carries its own colour, so a glance says whose plan the
// figures belong to before any of them are read.
func TestChatGPTMarkIsTinted(t *testing.T) {
	sty := styles.CharmtonePantera()
	recordPlan(t, spentPlan())

	plan, ok := PlanUsageFor(openaiID)
	require.True(t, ok)

	for _, p := range []styles.PlanUsage{sty.Header.PlanUsage, sty.ModelInfo.PlanUsage} {
		require.Contains(t, plan.Compact(p), p.ChatGPTIcon.Render(styles.ChatGPTUsageIcon))
		require.NotEqual(t,
			p.ChatGPTIcon.Render(styles.ChatGPTUsageIcon),
			p.Icon.Render(styles.ChatGPTUsageIcon),
			"the ChatGPT mark must be distinguishable from the default one")
	}
}
