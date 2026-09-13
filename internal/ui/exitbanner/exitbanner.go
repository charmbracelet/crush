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
		return sessionResumeLines(sess, width)

	default:
		// Unrecognized values render the full banner rather than nothing.
		style := lipgloss.NewStyle().Padding(1, 3)
		contentWidth := width - style.GetHorizontalFrameSize()

		sections := []string{logoSection(contentWidth)}
		if hasSession {
			sections = append(sections, sessionResumeLines(sess, contentWidth))
		}
		return style.Render(strings.Join(sections, "\n\n"))
	}
}

// logoSection returns the ASCII art logo followed by the parting message.
func logoSection(contentWidth int) string {
	t := styles.ThemeForProvider("")
	crushLogo := logo.Render(t.Logo.GradCanvas, version.Version, true, logo.Opts{
		FieldColor:   t.Logo.FieldColor,
		TitleColorA:  t.Logo.TitleColorA,
		TitleColorB:  t.Logo.TitleColorB,
		CharmColor:   t.Logo.CharmColor,
		VersionColor: t.Logo.VersionColor,
		Hyper:        false,
	})
	// Wrap the greeting and the message together: wrapping only the message
	// leaves the greeting's own width unaccounted for and overflows the frame.
	return crushLogo + "\n" +
		lipgloss.NewStyle().Width(contentWidth).Render("Thanks for using Crush! "+randomExitMessage())
}

// sessionResumeLines returns the "Session  <title>\nContinue crush -s <hash>"
// pair used by the exit banner.
func sessionResumeLines(sess *session.Session, contentWidth int) string {
	title := strings.ReplaceAll(sess.Title, "\n", " ")

	labelWidth := lipgloss.Width("Session  ")
	titleWidth := contentWidth - labelWidth
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
