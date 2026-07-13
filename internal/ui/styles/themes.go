package styles

import (
	"github.com/charmbracelet/x/exp/charmtone"
)

// ThemeKeyForProvider returns a stable identifier for the theme
// associated with the given provider ID. Providers that share a theme
// yield the same key, so callers can cheaply detect when switching
// providers would not actually change the active theme and skip the
// expensive style rebuild. This is the single source of truth for the
// provider-to-theme mapping; [ThemeForProvider] builds on it.
func ThemeKeyForProvider(providerID string) string {
	switch providerID {
	case "hyper":
		return "hyper"
	default:
		return "default"
	}
}

// ThemeForProvider returns the Styles associated with the given provider
// ID. Unknown or empty provider IDs yield the default Charmtone Pantera
// theme.
func ThemeForProvider(providerID string) Styles {
	switch ThemeKeyForProvider(providerID) {
	case "hyper":
		return HypercrushObsidiana()
	default:
		return CharmtonePantera()
	}
}

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	s := quickStyle(quickStyleOpts{
		primary:   charmtone.Sriracha,
		secondary: charmtone.Salmon,
		accent:    charmtone.Zest,
		keyword:   charmtone.Zest,

		fgBase:       charmtone.Sash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Char,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Char,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		denied:            charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		// ANSI 16-color palette for remapping raw terminal output
		// (e.g. bang-mode shell commands) onto legible Charmtone colors.
		ansiBlack:   charmtone.BBQ,
		ansiRed:     charmtone.Coral,
		ansiGreen:   charmtone.Guac,
		ansiYellow:  charmtone.Mustard,
		ansiBlue:    charmtone.Sriracha,
		ansiMagenta: charmtone.Salmon,
		ansiCyan:    charmtone.Malibu,
		ansiWhite:   charmtone.Smoke,

		ansiBrightBlack:   charmtone.Iron,
		ansiBrightRed:     charmtone.Tuna,
		ansiBrightGreen:   charmtone.Julep,
		ansiBrightYellow:  charmtone.Zest,
		ansiBrightBlue:    charmtone.Tuna,
		ansiBrightMagenta: charmtone.Zest,
		ansiBrightCyan:    charmtone.Sardine,
		ansiBrightWhite:   charmtone.Salt,
	})

	// Bang ! prompt overrides.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Sriracha)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(charmtone.Salmon)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(charmtone.Zest)

	logoOrange := charmtone.Salmon
	logoRed := charmtone.Sriracha
	logoLabel := charmtone.Zest

	// Logo and working indicator overrides use a warmer re:configured palette.
	s.Header.Charm = s.Header.Charm.Foreground(logoOrange)
	s.Header.Diagonals = s.Header.Diagonals.Foreground(logoRed)
	s.Header.LogoGradFromColor = logoOrange
	s.Header.LogoGradToColor = logoRed
	s.Logo.FieldColor = logoRed
	s.Logo.TitleColorA = logoOrange
	s.Logo.TitleColorB = logoRed
	s.Logo.CharmColor = logoLabel
	s.Logo.VersionColor = logoRed
	s.Logo.SmallCharm = s.Logo.SmallCharm.Foreground(logoLabel)
	s.Logo.SmallDiagonals = s.Logo.SmallDiagonals.Foreground(logoRed)
	s.Logo.SmallGradFromColor = logoOrange
	s.Logo.SmallGradToColor = logoRed
	s.WorkingGradFromColor = logoOrange
	s.WorkingGradToColor = logoRed
	s.WorkingLabelColor = logoLabel

	// Shell bar/prompt overrides.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(charmtone.Sriracha)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(charmtone.Iron)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(charmtone.Salmon)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(charmtone.Zest)
	s.Tool.ContentLine = s.Tool.ContentLine.
		Background(charmtone.Pepper)
	s.Tool.ContentTruncation = s.Tool.ContentTruncation.
		Background(charmtone.Pepper)
	s.Tool.ContentBg = s.Tool.ContentBg.
		Background(charmtone.Pepper)
	s.TextSelection = s.TextSelection.
		Foreground(charmtone.Salt).
		Background(charmtone.Salmon)
	if s.Markdown.H1.StylePrimitive.BackgroundColor != nil {
		s.Markdown.H1.StylePrimitive.BackgroundColor = nil
	}

	return s
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return CharmtonePantera()
}
