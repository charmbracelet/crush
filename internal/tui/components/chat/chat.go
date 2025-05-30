package chat

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/logo"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/version"
)

type SendMsg struct {
	Text        string
	Attachments []message.Attachment
}

type SessionSelectedMsg = session.Session

type SessionClearedMsg struct{}

type EditorFocusMsg bool

func header() string {
	return lipgloss.JoinVertical(
		lipgloss.Top,
		logoBlock(),
		repo(),
		"",
		cwd(),
	)
}

func lspsConfigured() string {
	cfg := config.Get()
	title := "LSP Configuration"

	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	lsps := baseStyle.
		Foreground(t.Primary()).
		Bold(true).
		Render(title)

	// Get LSP names and sort them for consistent ordering
	var lspNames []string
	for name := range cfg.LSP {
		lspNames = append(lspNames, name)
	}
	sort.Strings(lspNames)

	var lspViews []string
	for _, name := range lspNames {
		lsp := cfg.LSP[name]
		lspName := baseStyle.
			Foreground(t.Text()).
			Render(fmt.Sprintf("• %s", name))

		cmd := lsp.Command

		lspPath := baseStyle.
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf(" (%s)", cmd))

		lspViews = append(lspViews,
			baseStyle.
				Render(
					lipgloss.JoinHorizontal(
						lipgloss.Left,
						lspName,
						lspPath,
					),
				),
		)
	}

	return baseStyle.
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				lsps,
				lipgloss.JoinVertical(
					lipgloss.Left,
					lspViews...,
				),
			),
		)
}

func logoBlock() string {
	t := theme.CurrentTheme()
	return logo.Render(version.Version, true, logo.Opts{
		FieldColor:   t.Accent(),
		TitleColorA:  t.Primary(),
		TitleColorB:  t.Secondary(),
		CharmColor:   t.Primary(),
		VersionColor: t.Secondary(),
	})
}

func repo() string {
	repo := "https://github.com/opencode-ai/opencode"
	t := theme.CurrentTheme()

	return styles.BaseStyle().
		Foreground(t.TextMuted()).
		Render(repo)
}

func cwd() string {
	cwd := fmt.Sprintf("cwd: %s", config.WorkingDirectory())
	t := theme.CurrentTheme()

	return styles.BaseStyle().
		Foreground(t.TextMuted()).
		Render(cwd)
}

func initialScreen() string {
	baseStyle := styles.BaseStyle()

	return baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Top,
			header(),
			"",
			lspsConfigured(),
		),
	)
}
