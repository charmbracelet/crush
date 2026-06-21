package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// ThemeForProvider returns the Styles associated with the given provider
// ID and terminal background. Unknown or empty provider IDs yield the
// default Charmtone theme. When hasDarkBG is false, a light-friendly
// variant is returned instead.
func ThemeForProvider(providerID string, hasDarkBG bool) Styles {
	switch providerID {
	case "hyper":
		if hasDarkBG {
			return HypercrushObsidiana()
		}
		return HypercrushLight()
	default:
		if hasDarkBG {
			return CharmtonePantera()
		}
		return CharmtoneLight()
	}
}

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	s := quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,
		keyword:   charmtone.Blush,

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
		ansiBlue:    charmtone.Charple,
		ansiMagenta: charmtone.Dolly,
		ansiCyan:    charmtone.Malibu,
		ansiWhite:   charmtone.Smoke,

		ansiBrightBlack:   charmtone.Iron,
		ansiBrightRed:     charmtone.Tuna,
		ansiBrightGreen:   charmtone.Julep,
		ansiBrightYellow:  charmtone.Zest,
		ansiBrightBlue:    charmtone.Guppy,
		ansiBrightMagenta: charmtone.Blush,
		ansiBrightCyan:    charmtone.Sardine,
		ansiBrightWhite:   charmtone.Salt,
	})

	// Bang ! prompt overrides - use Salt/Hazy/Larple colors.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Hazy)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(charmtone.Hazy)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(charmtone.Larple)

	// Shell bar/prompt overrides - use Charple/Iron/Hazy colors.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(charmtone.Charple)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(charmtone.Iron)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(charmtone.Hazy)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(charmtone.Hazy)

	return s
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return CharmtonePantera()
}

// HypercrushLight returns the Hypercrush light theme.
func HypercrushLight() Styles {
	return CharmtoneLight()
}

// CharmtoneLight returns the Charmtone light theme for terminals with
// light (white) backgrounds. It inverts the neutral ramp so that
// foreground colors are dark and backgrounds are light, while keeping
// brand colors and most status hues unchanged.
func CharmtoneLight() Styles {
	s := quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,
		keyword:   charmtone.Darple,

		fgBase:       charmtone.Pepper,
		fgMoreSubtle: charmtone.BBQ,
		fgSubtle:     charmtone.Char,
		fgMostSubtle: charmtone.Iron,

		onPrimary: charmtone.Salt,

		bgBase:         charmtone.Salt,
		bgLeastVisible: charmtone.Soda,
		bgLessVisible:  charmtone.Sash,
		bgMostVisible:  charmtone.Oyster,

		separator: charmtone.Steep,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Yam,
		warning:           charmtone.Paprika,
		denied:            charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Pickle,

		ansiBlack:   charmtone.Pepper,
		ansiRed:     charmtone.Coral,
		ansiGreen:   charmtone.Pickle,
		ansiYellow:  charmtone.Yam,
		ansiBlue:    charmtone.Charple,
		ansiMagenta: charmtone.Dolly,
		ansiCyan:    charmtone.Malibu,
		ansiWhite:   charmtone.Soda,

		ansiBrightBlack:   charmtone.Iron,
		ansiBrightRed:     charmtone.Tuna,
		ansiBrightGreen:   charmtone.Julep,
		ansiBrightYellow:  charmtone.Zest,
		ansiBrightBlue:    charmtone.Guppy,
		ansiBrightMagenta: charmtone.Blush,
		ansiBrightCyan:    charmtone.Sardine,
		ansiBrightWhite:   charmtone.Salt,
	})

	// Bang ! prompt overrides - use Darple/Larple/Charple colors on light.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Darple)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(charmtone.Larple)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(charmtone.Charple)

	// Shell bar/prompt overrides.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(charmtone.Charple)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(charmtone.Oyster)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(charmtone.Darple)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(charmtone.Darple)

	// Diff overrides - use light insert/delete backgrounds.
	lightInsertNum := lipgloss.NewStyle().
		Foreground(charmtone.Pickle).
		Background(lipgloss.Color("#c8e6c9"))
	lightInsertSym := lipgloss.NewStyle().
		Foreground(charmtone.Pickle).
		Background(lipgloss.Color("#e8f5e9"))
	lightInsertCode := lipgloss.NewStyle().
		Foreground(charmtone.Pepper).
		Background(lipgloss.Color("#e8f5e9"))
	lightDeleteNum := lipgloss.NewStyle().
		Foreground(charmtone.Cherry).
		Background(lipgloss.Color("#ffcdd2"))
	lightDeleteSym := lipgloss.NewStyle().
		Foreground(charmtone.Cherry).
		Background(lipgloss.Color("#ffebee"))
	lightDeleteCode := lipgloss.NewStyle().
		Foreground(charmtone.Pepper).
		Background(lipgloss.Color("#ffebee"))

	s.Diff.InsertLine.LineNumber = lightInsertNum
	s.Diff.InsertLine.Symbol = lightInsertSym
	s.Diff.InsertLine.Code = lightInsertCode
	s.Diff.DeleteLine.LineNumber = lightDeleteNum
	s.Diff.DeleteLine.Symbol = lightDeleteSym
	s.Diff.DeleteLine.Code = lightDeleteCode

	return s
}
