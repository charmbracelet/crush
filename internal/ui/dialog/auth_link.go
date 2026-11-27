package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// wrappedLink renders url in style as a hyperlink broken at fixed
// character widths of at most width cells, rather than at hyphens, so it
// reads as one unbroken token. Each line carries its own hyperlink to
// the full URL, so clicking any of them opens it. params are the OSC 8
// hyperlink parameters, such as "id=oauth-verify".
func wrappedLink(style lipgloss.Style, url, params string, width int) string {
	link := style.Hyperlink(url, params)
	lines := strings.Split(ansi.Hardwrap(url, max(1, width), false), "\n")
	for i, line := range lines {
		lines[i] = link.Render(line)
	}
	return strings.Join(lines, "\n")
}
