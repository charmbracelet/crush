package fang

import (
	"image/color"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ColorScheme describes a colorscheme.
type ColorScheme struct {
	Base           color.Color
	Title          color.Color
	Description    color.Color
	Codeblock      color.Color
	Program        color.Color
	DimmedArgument color.Color
	Comment        color.Color
	Flag           color.Color
	FlagDefault    color.Color
	Command        color.Color
	QuotedString   color.Color
	Argument       color.Color
	Help           color.Color
	Dash           color.Color
	ErrorHeader    [2]color.Color // 0=fg 1=bg
	ErrorDetails   color.Color
}

// DefaultTheme is the default colorscheme.
//
// Deprecated: use [DefaultColorScheme] instead.
func DefaultTheme(isDark bool) ColorScheme {
	return DefaultColorScheme(lipgloss.LightDark(isDark))
}

// DefaultColorScheme is the default colorscheme.
func DefaultColorScheme(c lipgloss.LightDarkFunc) ColorScheme {
	return ColorScheme{
		Base:           c(charmtone.Charcoal, charmtone.Ash),
		Title:          charmtone.Charple,
		Codeblock:      c(charmtone.Salt, lipgloss.Color("#2F2E36")),
		Program:        c(charmtone.Malibu, charmtone.Guppy),
		Command:        c(charmtone.Pony, charmtone.Cheeky),
		DimmedArgument: c(charmtone.Squid, charmtone.Oyster),
		Comment:        c(charmtone.Squid, lipgloss.Color("#747282")),
		Flag:           c(lipgloss.Color("#0CB37F"), charmtone.Guac),
		Argument:       c(charmtone.Charcoal, charmtone.Ash),
		Description:    c(charmtone.Charcoal, charmtone.Ash), // flag and command descriptions
		FlagDefault:    c(charmtone.Smoke, charmtone.Squid),  // flag default values in descriptions
		QuotedString:   c(charmtone.Coral, charmtone.Salmon),
		ErrorHeader: [2]color.Color{
			charmtone.Butter,
			charmtone.Cherry,
		},
	}
}

// AnsiColorScheme is a ANSI colorscheme.
func AnsiColorScheme(c lipgloss.LightDarkFunc) ColorScheme {
	base := c(lipgloss.Black, lipgloss.White)
	return ColorScheme{
		Base:         base,
		Title:        lipgloss.Blue,
		Description:  base,
		Comment:      c(lipgloss.BrightWhite, lipgloss.BrightBlack),
		Flag:         lipgloss.Magenta,
		FlagDefault:  lipgloss.BrightMagenta,
		Command:      lipgloss.Cyan,
		QuotedString: lipgloss.Green,
		Argument:     base,
		Help:         base,
		Dash:         base,
		ErrorHeader:  [2]color.Color{lipgloss.Black, lipgloss.Red},
		ErrorDetails: lipgloss.Red,
	}
}

// Styles represents all the styles used.
type Styles struct {
	Text            lipgloss.Style
	Title           lipgloss.Style
	Span            lipgloss.Style
	ErrorHeader     lipgloss.Style
	ErrorText       lipgloss.Style
	FlagDescription lipgloss.Style
	FlagDefault     lipgloss.Style
	Codeblock       Codeblock
	Program         Program
}

// Codeblock styles.
type Codeblock struct {
	Base    lipgloss.Style
	Program Program
	Text    lipgloss.Style
	Comment lipgloss.Style
}

// Program name, args, flags, styling.
type Program struct {
	Name           lipgloss.Style
	Command        lipgloss.Style
	Flag           lipgloss.Style
	Argument       lipgloss.Style
	DimmedArgument lipgloss.Style
	QuotedString   lipgloss.Style
}

func mustColorscheme(cs func(lipgloss.LightDarkFunc) ColorScheme) ColorScheme {
	var isDark bool
	if term.IsTerminal(os.Stdout.Fd()) {
		isDark = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	}
	return cs(lipgloss.LightDark(isDark))
}

func makeStyles(cs ColorScheme) Styles {
	//nolint:mnd
	return Styles{
		Text: lipgloss.NewStyle().Foreground(cs.Base),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(cs.Title).
			Transform(strings.ToUpper).
			Padding(1, 0).
			Margin(0, 2),
		FlagDescription: lipgloss.NewStyle().
			Foreground(cs.Description).
			Transform(titleFirstWord),
		FlagDefault: lipgloss.NewStyle().
			Foreground(cs.FlagDefault),
		Codeblock: Codeblock{
			Base: lipgloss.NewStyle().
				Background(cs.Codeblock).
				Foreground(cs.Base).
				MarginLeft(2).
				Padding(1, 2),
			Text: lipgloss.NewStyle().
				Background(cs.Codeblock),
			Comment: lipgloss.NewStyle().
				Background(cs.Codeblock).
				Foreground(cs.Comment),
			Program: Program{
				Name: lipgloss.NewStyle().
					Background(cs.Codeblock).
					Foreground(cs.Program),
				Flag: lipgloss.NewStyle().
					Background(cs.Codeblock).
					Foreground(cs.Flag),
				Argument: lipgloss.NewStyle().
					Background(cs.Codeblock).
					Foreground(cs.Argument),
				DimmedArgument: lipgloss.NewStyle().
					Background(cs.Codeblock).
					Foreground(cs.DimmedArgument),
				Command: lipgloss.NewStyle().
					Background(cs.Codeblock).
					Foreground(cs.Command),
				QuotedString: lipgloss.NewStyle().
					Background(cs.Codeblock).
					Foreground(cs.QuotedString),
			},
		},
		Program: Program{
			Name: lipgloss.NewStyle().
				Foreground(cs.Program),
			Argument: lipgloss.NewStyle().
				Foreground(cs.Argument),
			DimmedArgument: lipgloss.NewStyle().
				Foreground(cs.DimmedArgument),
			Flag: lipgloss.NewStyle().
				Foreground(cs.Flag),
			Command: lipgloss.NewStyle().
				Foreground(cs.Command),
			QuotedString: lipgloss.NewStyle().
				Foreground(cs.QuotedString),
		},
		Span: lipgloss.NewStyle().
			Background(cs.Codeblock),
		ErrorText: lipgloss.NewStyle().
			MarginLeft(2).
			Width(width() - 4).
			Transform(titleFirstWord),
		ErrorHeader: lipgloss.NewStyle().
			Foreground(cs.ErrorHeader[0]).
			Background(cs.ErrorHeader[1]).
			Bold(true).
			Padding(0, 1).
			Margin(1).
			MarginLeft(2).
			SetString("ERROR"),
	}
}

func titleFirstWord(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	words[0] = cases.Title(language.AmericanEnglish).String(words[0])
	return strings.Join(words, " ")
}
