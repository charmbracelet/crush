package styles

import (
	"image/color"
	"math"

	"charm.land/glamour/v2/ansi"
)

// SystemThemeName is the name of the theme generated from terminal colors.
const SystemThemeName = "system"

// TerminalPalette contains the terminal's default colors and ANSI palette.
type TerminalPalette struct {
	Foreground color.Color
	Background color.Color
	ANSI       [16]color.Color
}

// IsUsable reports whether the palette has both a foreground and background.
func (p TerminalPalette) IsUsable() bool {
	background := p.Background != nil || p.ANSI[0] != nil
	foreground := p.Foreground != nil || p.ANSI[7] != nil
	return background && foreground
}

// SystemThemeStyles builds styles from the terminal palette, falling back to
// Charmtone when the palette lacks usable foreground or background colors.
func SystemThemeStyles(palette TerminalPalette) Styles {
	if !palette.IsUsable() {
		return CharmtonePantera()
	}

	fallback := charmtoneOpts()
	fallbackANSI := [16]color.Color{
		fallback.ansiBlack, fallback.ansiRed, fallback.ansiGreen, fallback.ansiYellow,
		fallback.ansiBlue, fallback.ansiMagenta, fallback.ansiCyan, fallback.ansiWhite,
		fallback.ansiBrightBlack, fallback.ansiBrightRed, fallback.ansiBrightGreen, fallback.ansiBrightYellow,
		fallback.ansiBrightBlue, fallback.ansiBrightMagenta, fallback.ansiBrightCyan, fallback.ansiBrightWhite,
	}
	var terminalANSI [16]color.Color
	for i, c := range palette.ANSI {
		terminalANSI[i] = firstColor(c, fallbackANSI[i])
	}

	background := firstColor(palette.Background, terminalANSI[0], fallback.bgBase)
	foreground := firstColor(palette.Foreground, terminalANSI[7], fallback.fgBase)
	isDark := systemLuminance(background) < 0.5
	backgroundLeast := systemSurface(background, isDark, 0.10)
	backgroundLess := systemSurface(background, isDark, 0.24)
	backgroundMost := systemSurface(background, isDark, 0.40)

	foreground = systemReadable(foreground, background, 4.5)
	muted := systemReadable(systemMutedText(background, isDark), background, 4.5)
	fgSubtle := systemReadable(systemMix(foreground, background, 0.82), background, 4.5)
	fgMoreSubtle := systemReadable(systemMix(muted, background, 0.78), background, 3.0)
	fgMostSubtle := systemReadable(systemMix(muted, background, 0.55), background, 2.0)

	ansiColor := func(index int) color.Color {
		return systemReadable(terminalANSI[index], background, 3.0)
	}
	primary := ansiColor(6)
	secondary := ansiColor(5)
	green := ansiColor(2)
	brightGreen := ansiColor(10)
	red := ansiColor(1)
	brightRed := ansiColor(9)
	yellow := ansiColor(3)
	brightYellow := ansiColor(11)
	blue := ansiColor(4)
	cyan := ansiColor(6)

	opts := quickStyleOpts{
		primary:   primary,
		secondary: secondary,
		accent:    cyan,
		keyword:   secondary,

		fgBase:       foreground,
		fgSubtle:     fgSubtle,
		fgMoreSubtle: fgMoreSubtle,
		fgMostSubtle: fgMostSubtle,

		bgBase:         background,
		bgLeastVisible: backgroundLeast,
		bgLessVisible:  backgroundLess,
		bgMostVisible:  backgroundMost,
		separator:      backgroundLess,

		onPrimary: bestSystemInk(primary),

		destructive:       brightRed,
		destructiveSubtle: red,
		error:             red,
		warning:           yellow,
		warningSubtle:     brightYellow,
		attention:         yellow,
		busy:              brightYellow,
		info:              cyan,
		infoMoreSubtle:    blue,
		infoMostSubtle:    blue,
		success:           green,
		successMoreSubtle: brightGreen,
		successMostSubtle: green,
		yolo:              brightYellow,
		plan:              secondary,
		planMoreSubtle:    backgroundLess,

		button:         primary,
		buttonSubtle:   backgroundLess,
		buttonInactive: backgroundMost,
		buttonHovered:  fgMoreSubtle,

		ansiBlack:         ansiColor(0),
		ansiRed:           ansiColor(1),
		ansiGreen:         ansiColor(2),
		ansiYellow:        ansiColor(3),
		ansiBlue:          ansiColor(4),
		ansiMagenta:       ansiColor(5),
		ansiCyan:          ansiColor(6),
		ansiWhite:         ansiColor(7),
		ansiBrightBlack:   ansiColor(8),
		ansiBrightRed:     ansiColor(9),
		ansiBrightGreen:   ansiColor(10),
		ansiBrightYellow:  ansiColor(11),
		ansiBrightBlue:    ansiColor(12),
		ansiBrightMagenta: ansiColor(13),
		ansiBrightCyan:    ansiColor(14),
		ansiBrightWhite:   ansiColor(15),
	}

	s := legibleBadges(quickStyle(opts), opts)
	setSystemSyntax(s.Markdown.CodeBlock.Chroma, opts)
	setSystemSyntax(s.PlanMarkdown.CodeBlock.Chroma, opts)
	s.Markdown.Document.Color = hex(foreground)
	s.PlanMarkdown.Document.Color = s.Markdown.Document.Color
	s.Markdown.Link.Color = hex(blue)
	s.Markdown.Image.Color = hex(cyan)
	s.Markdown.Code.Color = hex(systemReadable(green, backgroundLess, 4.5))
	s.Markdown.Code.BackgroundColor = hex(backgroundLess)
	s.PlanMarkdown.Link.Color = s.Markdown.Link.Color
	s.PlanMarkdown.Image.Color = s.Markdown.Image.Color
	s.PlanMarkdown.Code.Color = s.Markdown.Code.Color
	s.PlanMarkdown.Code.BackgroundColor = s.Markdown.Code.BackgroundColor
	return s
}

func setSystemSyntax(chroma *ansi.Chroma, opts quickStyleOpts) {
	if chroma == nil {
		return
	}
	chroma.Comment.Color = hex(opts.fgMoreSubtle)
	chroma.CommentPreproc.Color = hex(opts.info)
	chroma.Keyword.Color = hex(opts.secondary)
	chroma.KeywordReserved.Color = hex(opts.secondary)
	chroma.KeywordNamespace.Color = hex(opts.accent)
	chroma.KeywordType.Color = hex(opts.accent)
	chroma.Operator.Color = hex(opts.primary)
	chroma.Punctuation.Color = hex(opts.fgSubtle)
	chroma.Name.Color = hex(opts.fgBase)
	chroma.NameBuiltin.Color = hex(opts.info)
	chroma.NameTag.Color = hex(opts.success)
	chroma.NameAttribute.Color = hex(opts.warning)
	chroma.NameClass.Color = hex(opts.primary)
	chroma.NameConstant.Color = hex(opts.warning)
	chroma.NameDecorator.Color = hex(opts.secondary)
	chroma.NameException.Color = hex(opts.destructive)
	chroma.NameFunction.Color = hex(opts.info)
	chroma.NameOther.Color = hex(opts.accent)
	chroma.Literal.Color = hex(opts.success)
	chroma.LiteralNumber.Color = hex(opts.warning)
	chroma.LiteralDate.Color = hex(opts.warning)
	chroma.LiteralString.Color = hex(opts.success)
	chroma.LiteralStringEscape.Color = hex(opts.destructive)
	chroma.GenericDeleted.Color = hex(opts.destructive)
	chroma.GenericInserted.Color = hex(opts.success)
}

func firstColor(colors ...color.Color) color.Color {
	for _, c := range colors {
		if c != nil {
			return c
		}
	}
	return nil
}

func systemLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 0xffff
}

func systemSurface(background color.Color, isDark bool, factor float64) color.Color {
	const maxChannel = 255.0
	converted := color.NRGBAModel.Convert(background).(color.NRGBA)
	r, g, b := float64(converted.R), float64(converted.G), float64(converted.B)
	luminance := 0.299*r + 0.587*g + 0.114*b
	if isDark && luminance < 10 {
		gray := uint8(math.Floor(factor * maxChannel))
		return color.NRGBA{R: gray, G: gray, B: gray, A: 0xff}
	}
	if !isDark && luminance > 245 {
		gray := uint8(math.Floor(maxChannel - factor*maxChannel))
		return color.NRGBA{R: gray, G: gray, B: gray, A: 0xff}
	}
	if luminance == 0 {
		return background
	}
	next := luminance * (1 - factor)
	if isDark {
		next = luminance + (maxChannel-luminance)*factor
	}
	ratio := next / luminance
	return color.NRGBA{
		R: uint8(math.Round(min(ratio*r, maxChannel))),
		G: uint8(math.Round(min(ratio*g, maxChannel))),
		B: uint8(math.Round(min(ratio*b, maxChannel))),
		A: 0xff,
	}
}

func systemMutedText(background color.Color, isDark bool) color.Color {
	luminance := systemLuminance(background) * 255
	var gray uint8
	if isDark {
		gray = uint8(min(math.Floor(160+luminance*0.3), 200))
		if luminance < 10 {
			gray = 180
		}
	} else {
		gray = uint8(max(math.Floor(100-(255-luminance)*0.2), 60))
		if luminance > 245 {
			gray = 75
		}
	}
	return color.NRGBA{R: gray, G: gray, B: gray, A: 0xff}
}

func systemMix(foreground, background color.Color, foregroundAmount float64) color.Color {
	fg := color.NRGBAModel.Convert(foreground).(color.NRGBA)
	bg := color.NRGBAModel.Convert(background).(color.NRGBA)
	return color.NRGBA{
		R: uint8(math.Round(float64(fg.R)*foregroundAmount + float64(bg.R)*(1-foregroundAmount))),
		G: uint8(math.Round(float64(fg.G)*foregroundAmount + float64(bg.G)*(1-foregroundAmount))),
		B: uint8(math.Round(float64(fg.B)*foregroundAmount + float64(bg.B)*(1-foregroundAmount))),
		A: 0xff,
	}
}

func systemReadable(foreground, background color.Color, minimumContrast float64) color.Color {
	if wcagContrast(foreground, background) >= minimumContrast {
		return foreground
	}
	target := color.Color(color.White)
	if wcagContrast(color.Black, background) > wcagContrast(color.White, background) {
		target = color.Black
	}
	for step := 1; step <= 100; step++ {
		candidate := systemMix(target, foreground, float64(step)/100)
		if wcagContrast(candidate, background) >= minimumContrast {
			return candidate
		}
	}
	return target
}

func bestSystemInk(background color.Color) color.Color {
	if wcagContrast(color.Black, background) >= wcagContrast(color.White, background) {
		return color.Black
	}
	return color.White
}
