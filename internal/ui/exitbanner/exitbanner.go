// Package exitbanner renders what Crush prints after the TUI exits.
package exitbanner

import (
	"math/rand/v2"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/logo"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
)

// FallbackWidth is used when stdout is not a terminal, so the banner still
// wraps to something sane when output is piped or captured.
const FallbackWidth = 80

// leftPadding is the number of spaces every content line of the full banner is
// indented by.
const leftPadding = 3

// Render returns the exit banner for the given style, or an empty string when
// there is nothing to print. A nil or untitled session means the resume hint is
// omitted, which for the compact banner leaves nothing at all.
func Render(banner config.ExitBanner, sess *session.Session, width int) string {
	if width <= 0 {
		width = FallbackWidth
	}
	hasSession := sess != nil && sess.ID != ""

	switch banner {
	case config.ExitBannerNone:
		return ""

	case config.ExitBannerCompact:
		if !hasSession {
			return ""
		}
		return "\n" + sessionResumeLines(sess, width)

	default:
		// Unrecognized values render the full banner rather than nothing.
		contentWidth := width - 2*leftPadding

		sections := []string{logoSection(contentWidth)}
		if hasSession {
			sections = append(sections, sessionResumeLines(sess, contentWidth))
		}

		// A blank line first, to separate the banner from the shell prompt
		// the command was run from.
		return "\n" + indent(strings.Join(sections, "\n\n"), leftPadding)
	}
}

// indent prefixes every non-empty line with n spaces. Unlike a rectangular
// style block (lipgloss pads every line to the widest line of the block), this
// never pads lines out to the width of the terminal: a line exactly as wide as
// the terminal wraps even a space or two of remainder on terminals reporting a
// slightly larger width than what they can actually display (scrollbars,
// terminal-side padding), which printed a blank line after every row.
func indent(s string, n int) string {
	lines := strings.Split(s, "\n")
	pad := strings.Repeat(" ", n)
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

// logoSection returns the ASCII art logo followed by the parting message. The
// greeting is word-wrapped to fit narrow terminals, but never padded, so no
// line of the banner can end up as wide as the terminal.
func logoSection(contentWidth int) string {
	if contentWidth < 1 {
		contentWidth = 1
	}
	t := styles.ThemeForProvider("")
	crushLogo := logo.Render(t.Logo.GradCanvas, version.Version, true, logo.Opts{
		FieldColor:   t.Logo.FieldColor,
		TitleColorA:  t.Logo.TitleColorA,
		TitleColorB:  t.Logo.TitleColorB,
		CharmColor:   t.Logo.CharmColor,
		VersionColor: t.Logo.VersionColor,
		Hyper:        false,
	})
	greeting := strings.TrimRight("Thanks for using Crush! "+randomExitMessage(), " ")
	return crushLogo + "\n" + wrapGreeting(greeting, contentWidth)
}

// wrapGreeting word-wraps the parting message so every line fits within limit.
// An overly long style.Width-based block would pad the greeting to the full
// content width, making the whole banner exactly as wide as the terminal.
func wrapGreeting(greeting string, limit int) string {
	return ansi.Wordwrap(greeting, max(limit, 1), "")
}

// sessionResumeLines returns the "Session  <title>\nContinue crush -s <hash>"
// pair used by the exit banner.
func sessionResumeLines(sess *session.Session, width int) string {
	title := strings.ReplaceAll(sess.Title, "\n", " ")

	labelWidth := lipgloss.Width("Session  ")
	// Leave the last column empty: a line exactly as wide as the terminal
	// wraps even a remainder of a single space on terminals that report a
	// slightly wider display than they can actually fill.
	titleWidth := width - labelWidth - 1
	if titleWidth > 0 {
		title = ansi.Truncate(title, titleWidth, "…")
	}

	hash := session.HashID(sess.ID)[:7]
	label := lipgloss.NewStyle().Foreground(charmtone.Charple)
	sessionLine := label.Render("Session  ") + title
	continueLine := label.Render("Continue ") + "crush -s " + hash
	return sessionLine + "\n" + continueLine
}

// randomExitMessage returns a random exit message.
func randomExitMessage() string {
	messages := []string{
		"",
		"See ya later.",
		"You look great.",
		"Have a gorgeous time.",
		"Get some rest.",
		"Come back soon.",
		"You worked handsomely.",
		"Time for a snack.",
		"Who’s hungry?",
		"That was fun.",
		"See you at breakfast?",
		"Time for a nap.",
		"Who wants some spaghetti?",
		"Take care of yourself.",
		"Remember to hydrate.",
		"Time for a swim?",
		"You’re quite glamorous, you know.",
		"Nice work.",
		"You’re a sensation.",
		"Where’s my eyeliner?",
		"It’s tea time.",
	}
	return messages[rand.IntN(len(messages))]
}
