package logo

import (
	"fmt"
	"image/color"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/MakeNowJust/heredoc"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
)

type letterform func(bool) string

type Opts struct {
	FieldColor   color.Color
	TitleColorA  color.Color
	TitleColorB  color.Color
	CharmColor   color.Color
	VersionColor color.Color
	Width        int
}

const diag = `╱`

func Render(version string, compact bool, o Opts) string {
	const charm = " Blush"
	fg := func(c color.Color, s string) string {
		return lipgloss.NewStyle().Foreground(c).Render(s)
	}

	letterforms := []letterform{
		letterB,
		letterL,
		letterU,
		letterSStylized,
		letterH,
	}
	stretchIndex := -1
	if !compact {
		stretchIndex = rand.Intn(len(letterforms))
	}

	blush := renderWord(1, stretchIndex, letterforms...)
	b := new(strings.Builder)
	for r := range strings.SplitSeq(blush, "\n") {
		fmt.Fprintln(b, styles.ApplyForegroundGrad(r, o.TitleColorA, o.TitleColorB))
	}
	blush = b.String()

	metaRow := fg(o.CharmColor, charm) + " " + fg(o.VersionColor, version)
	return strings.TrimSpace(metaRow + "\n" + blush)
}

func SmallRender(width int) string {
	t := styles.CurrentTheme()
	title := styles.ApplyBoldForegroundGrad("Blush", t.Secondary, t.Primary)
	return ansi.Truncate(title, width, "…")
}

func renderWord(spacing int, stretchIndex int, letterforms ...letterform) string {
	rendered := make([]string, len(letterforms))
	for i, letter := range letterforms {
		rendered[i] = letter(i == stretchIndex)
	}
	if spacing > 0 {
		rendered = append(rendered[:0], rendered...)
	}
	return strings.TrimSpace(lipgloss.JoinHorizontal(lipgloss.Top, rendered...))
}

func letterB(stretch bool) string {
	left := heredoc.Doc(`█
█
█`)
	mid := heredoc.Doc(`▀▀
▀▀
▄▄`)
	right := heredoc.Doc(`▄
▄
▀`)
	return joinLetterform(left, stretchLetterformPart(mid, stretch, 2, 6, 10), right)
}

func letterL(stretch bool) string {
	side := heredoc.Doc(`█
█
█`)
	bottom := heredoc.Doc(`▀`)
	return joinLetterform(side, stretchLetterformPart(bottom, stretch, 3, 7, 12))
}

func letterU(stretch bool) string {
	side := heredoc.Doc(`█
█`)
	mid := heredoc.Doc(`▀`)
	return joinLetterform(side, stretchLetterformPart(mid, stretch, 3, 7, 12), side)
}

func letterSStylized(stretch bool) string {
	left := heredoc.Doc(`▄
▀
▀`)
	center := heredoc.Doc(`▀
▀
▀`)
	right := heredoc.Doc(`▀
█`)
	return joinLetterform(left, stretchLetterformPart(center, stretch, 3, 7, 12), right)
}

func letterH(stretch bool) string {
	side := heredoc.Doc(`█
█
▀`)
	mid := heredoc.Doc(`▀`)
	return joinLetterform(side, stretchLetterformPart(mid, stretch, 3, 8, 12), side)
}

func joinLetterform(letters ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, letters...)
}

func stretchLetterformPart(s string, stretch bool, width, minStretch, maxStretch int) string {
	n := width
	if stretch {
		n = rand.Intn(maxStretch-minStretch) + minStretch
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = s
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
