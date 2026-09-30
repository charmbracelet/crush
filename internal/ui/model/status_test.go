package model

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// leadingSpaces counts the spaces at the start of a stripped line.
func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// drawStatusLines draws the status bar into a screen buffer and returns the
// visible content of each row.
func drawStatusLines(t *testing.T, st *Status, w, h int) []string {
	t.Helper()
	scr := uv.NewScreenBuffer(w, h)
	st.Draw(scr, uv.Rect(0, 0, w, h))
	lines := strings.Split(ansi.Strip(scr.Render()), "\n")
	return lines
}

func TestStatusDrawExpandedHelpRowsAlignWithBadge(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.ToggleHelp()
	st.SetMode(uiInputModePlan, false)

	lines := drawStatusLines(t, st, 100, 6)
	require.True(t, strings.HasPrefix(lines[0], strings.Repeat(" ", badgeLeftInset)+" "+"PLAN MODE"),
		"the badge row must start with the badge inset and its padding: %q", lines[0])

	// Every subsequent help row must start at the same column as the hints
	// on the badge row: badge inset + badge width + separator + help padding.
	badge := st.modeBadge()
	wantHintsCol := badgeLeftInset + lipgloss.Width(badge) + 1 + u.com.Styles.Status.Help.GetPaddingLeft()
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		require.Equal(t, wantHintsCol, leadingSpaces(line),
			"expanded help row %d must align with the badge row hints", i+1)
	}
}

func TestStatusDrawExpandedHelpRowsAlignWithoutBadge(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.ToggleHelp()
	st.SetMode(uiInputModeCode, false)

	lines := drawStatusLines(t, st, 100, 6)
	wantCol := u.com.Styles.Status.Help.GetPaddingLeft()
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			break
		}
		require.Equal(t, wantCol, leadingSpaces(line),
			"expanded help row %d must align with the first row", i)
	}
}

func TestStatusDrawShowsRouterBadge(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.SetRouterDecision(router.Decision{ReasoningEffort: "high", Confidence: 0.9}, true)

	lines := drawStatusLines(t, st, 100, 6)
	require.Contains(t, lines[0], "router: high")
	// Assert the exact badge text, not just a substring: WarnIndicator and
	// InfoIndicator both carry a baked-in SetString label ("WARNING" /
	// "OKAY!") that lipgloss's Render prepends unless cleared — this must
	// not leak into the badge text.
	require.Equal(t, "router: high", strings.TrimSpace(ansi.Strip(st.routerBadge())))
}

func TestStatusDrawRouterBadgeFlagsLowConfidence(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.SetRouterDecision(router.Decision{ReasoningEffort: "low", Confidence: 0.4, LowConfidence: true}, true)

	scr := uv.NewScreenBuffer(100, 6)
	st.Draw(scr, uv.Rect(0, 0, 100, 6))
	rendered := scr.Render() // keep ANSI codes for this one, unlike drawStatusLines
	require.Contains(t, rendered, "router: low")
	// Assert the exact badge text, not just a substring: WarnIndicator and
	// InfoIndicator both carry a baked-in SetString label ("WARNING" /
	// "OKAY!") that lipgloss's Render prepends unless cleared — this must
	// not leak into the badge text.
	require.Equal(t, "router: low", strings.TrimSpace(ansi.Strip(st.routerBadge())))
	// The low-confidence badge must use the warn token, not the info token —
	// compare against both rendered forms to confirm which one matched.
	// Each candidate clears the indicator's baked-in label (matching what
	// routerBadge does) and is rendered through the same screen-buffer
	// draw+Render pipeline as the actual output, since raw lipgloss.Render()
	// bytes never match uv.ScreenBuffer.Render() output (the buffer
	// re-serializes ANSI transitions differently even for identical styled
	// text).
	warnForm := renderBadgeThroughScreen(t, u.com.Styles.Status.WarnIndicator.SetString("").Render("router: low"))
	infoForm := renderBadgeThroughScreen(t, u.com.Styles.Status.InfoIndicator.SetString("").Render("router: low"))
	require.Contains(t, rendered, warnForm)
	require.NotContains(t, rendered, infoForm)
}

// renderBadgeThroughScreen draws a pre-styled string onto an isolated
// uv.ScreenBuffer and returns its rendered ANSI, so it can be compared
// byte-for-byte against ANSI produced by the same pipeline elsewhere.
func renderBadgeThroughScreen(t *testing.T, styled string) string {
	t.Helper()
	w := lipgloss.Width(styled)
	scr := uv.NewScreenBuffer(w, 1)
	uv.NewStyledString(styled).Draw(scr, uv.Rect(0, 0, w, 1))
	return scr.Render()
}

func TestStatusDrawShowsModelAndEffortWhenBothApplied(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.SetRouterDecision(router.Decision{ModelID: "anthropic/claude-haiku-4", ReasoningEffort: "high"}, true)

	require.Equal(t, "router: claude-haiku-4 · high", strings.TrimSpace(ansi.Strip(st.routerBadge())))
}

func TestStatusDrawShowsEffortOnlyWhenNoModelApplied(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.SetRouterDecision(router.Decision{ReasoningEffort: "high"}, true)

	require.Equal(t, "router: high", strings.TrimSpace(ansi.Strip(st.routerBadge())))
}

func TestStatusDrawShowsModelOnlyWithNoDanglingSeparator(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.SetRouterDecision(router.Decision{ModelID: "anthropic/claude-haiku-4"}, true)

	require.Equal(t, "router: claude-haiku-4", strings.TrimSpace(ansi.Strip(st.routerBadge())))
}

func TestStatusDrawNoRouterBadgeWhenNoDecisionYet(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	// SetRouterDecision never called.

	lines := drawStatusLines(t, st, 100, 6)
	require.NotContains(t, lines[0], "router:")
}
