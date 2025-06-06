package diffview

import (
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

type LineStyle struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

type Style struct {
	DividerLine LineStyle
	MissingLine LineStyle
	EqualLine   LineStyle
	InsertLine  LineStyle
	DeleteLine  LineStyle
}

var DefaultLightStyle = Style{
	DividerLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Iron).
			Background(charmtone.Thunder).
			Align(lipgloss.Right).
			Padding(0, 1),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Oyster).
			Background(charmtone.Anchovy),
	},
	MissingLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Background(charmtone.Ash).
			Padding(0, 1),
		Code: lipgloss.NewStyle().
			Background(charmtone.Ash),
	},
	EqualLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Charcoal).
			Background(charmtone.Ash).
			Align(lipgloss.Right).
			Padding(0, 1),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Pepper).
			Background(charmtone.Salt),
	},
	InsertLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Turtle).
			Background(lipgloss.Color("#c8e6c9")).
			Align(lipgloss.Right).
			Padding(0, 1),
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Turtle).
			Background(lipgloss.Color("#e8f5e9")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Pepper).
			Background(lipgloss.Color("#e8f5e9")),
	},
	DeleteLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Cherry).
			Background(lipgloss.Color("#ffcdd2")).
			Align(lipgloss.Left).
			Padding(0, 1),
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Cherry).
			Background(lipgloss.Color("#ffebee")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Pepper).
			Background(lipgloss.Color("#ffebee")),
	},
}

var DefaultDarkStyle = Style{
	DividerLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Smoke).
			Background(charmtone.Sapphire).
			Align(lipgloss.Right).
			Padding(0, 1),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Smoke).
			Background(charmtone.Ox),
	},
	MissingLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Background(charmtone.Charcoal).
			Padding(0, 1),
		Code: lipgloss.NewStyle().
			Background(charmtone.Charcoal),
	},
	EqualLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Ash).
			Background(charmtone.Charcoal).
			Align(lipgloss.Right).
			Padding(0, 1),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Salt).
			Background(charmtone.Pepper),
	},
	InsertLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Turtle).
			Background(lipgloss.Color("#293229")).
			Align(lipgloss.Right).
			Padding(0, 1),
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Turtle).
			Background(lipgloss.Color("#303a30")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Salt).
			Background(lipgloss.Color("#303a30")),
	},
	DeleteLine: LineStyle{
		LineNumber: lipgloss.NewStyle().
			Foreground(charmtone.Cherry).
			Background(lipgloss.Color("#332929")).
			Align(lipgloss.Left).
			Padding(0, 1),
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Cherry).
			Background(lipgloss.Color("#3a3030")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Salt).
			Background(lipgloss.Color("#3a3030")),
	},
}
