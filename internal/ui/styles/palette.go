package styles

import "image/color"

// Palette is the semantic color set used to build a theme. It mirrors the
// unexported quickStyleOpts palette but with exported fields so other
// packages (e.g. the theme catalog) can construct a theme from a 16-color
// terminal scheme.
type Palette struct {
	// Brand.
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color
	Keyword   color.Color

	// Default foreground and background colors.
	FgBase color.Color
	BgBase color.Color

	// Low-contrast dividers, separators, and rule lines.
	Separator color.Color

	FgSubtle     color.Color
	FgMoreSubtle color.Color
	FgMostSubtle color.Color

	// Contrast pairings: foregrounds designed to sit on top of a
	// matching background role.
	OnPrimary color.Color

	BgMostVisible  color.Color
	BgLessVisible  color.Color
	BgLeastVisible color.Color

	// Statuses.
	Destructive       color.Color
	Error             color.Color
	Warning           color.Color
	WarningSubtle     color.Color
	Attention         color.Color
	Busy              color.Color
	Info              color.Color
	InfoMoreSubtle    color.Color
	InfoMostSubtle    color.Color
	Success           color.Color
	SuccessMoreSubtle color.Color
	SuccessMostSubtle color.Color

	// ANSI 16-color palette. These remap the basic terminal colors that
	// programs emit onto legible, on-brand colors.
	AnsiBlack         color.Color
	AnsiRed           color.Color
	AnsiGreen         color.Color
	AnsiYellow        color.Color
	AnsiBlue          color.Color
	AnsiMagenta       color.Color
	AnsiCyan          color.Color
	AnsiWhite         color.Color
	AnsiBrightBlack   color.Color
	AnsiBrightRed     color.Color
	AnsiBrightGreen   color.Color
	AnsiBrightYellow  color.Color
	AnsiBrightBlue    color.Color
	AnsiBrightMagenta color.Color
	AnsiBrightCyan    color.Color
	AnsiBrightWhite   color.Color
}

// FromPalette builds a Styles from a semantic color palette.
func FromPalette(p Palette) Styles {
	return quickStyle(quickStyleOpts{
		primary:   p.Primary,
		secondary: p.Secondary,
		accent:    p.Accent,
		keyword:   p.Keyword,

		fgBase:       p.FgBase,
		bgBase:       p.BgBase,
		separator:    p.Separator,
		fgSubtle:     p.FgSubtle,
		fgMoreSubtle: p.FgMoreSubtle,
		fgMostSubtle: p.FgMostSubtle,
		onPrimary:    p.OnPrimary,

		bgMostVisible:  p.BgMostVisible,
		bgLessVisible:  p.BgLessVisible,
		bgLeastVisible: p.BgLeastVisible,

		destructive:       p.Destructive,
		error:             p.Error,
		warning:           p.Warning,
		warningSubtle:     p.WarningSubtle,
		attention:         p.Attention,
		busy:              p.Busy,
		info:              p.Info,
		infoMoreSubtle:    p.InfoMoreSubtle,
		infoMostSubtle:    p.InfoMostSubtle,
		success:           p.Success,
		successMoreSubtle: p.SuccessMoreSubtle,
		successMostSubtle: p.SuccessMostSubtle,

		ansiBlack:   p.AnsiBlack,
		ansiRed:     p.AnsiRed,
		ansiGreen:   p.AnsiGreen,
		ansiYellow:  p.AnsiYellow,
		ansiBlue:    p.AnsiBlue,
		ansiMagenta: p.AnsiMagenta,
		ansiCyan:    p.AnsiCyan,
		ansiWhite:   p.AnsiWhite,

		ansiBrightBlack:   p.AnsiBrightBlack,
		ansiBrightRed:     p.AnsiBrightRed,
		ansiBrightGreen:   p.AnsiBrightGreen,
		ansiBrightYellow:  p.AnsiBrightYellow,
		ansiBrightBlue:    p.AnsiBrightBlue,
		ansiBrightMagenta: p.AnsiBrightMagenta,
		ansiBrightCyan:    p.AnsiBrightCyan,
		ansiBrightWhite:   p.AnsiBrightWhite,
	})
}
