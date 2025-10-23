package styles

import (
	"github.com/charmbracelet/lipgloss/v2"
)

// NewNordTheme creates the Nord theme
func NewNordTheme() *Theme {
	return &Theme{
		Name:   "nord",
		IsDark: true,

		Primary:   ParseHex("#88C0D0"), // Nord Blue
		Secondary: ParseHex("#EBCB8B"), // Nord Yellow
		Tertiary:  ParseHex("#A3BE8C"), // Nord Green
		Accent:    ParseHex("#B48EAD"), // Nord Purple

		BgBase:        ParseHex("#2E3440"), // Nord Dark BG
		BgBaseLighter: ParseHex("#3B4252"), // Nord Medium BG
		BgSubtle:      ParseHex("#434C5E"), // Nord Bright BG
		BgOverlay:     ParseHex("#4C566A"), // Nord Highlight BG

		FgBase:      ParseHex("#D8DEE9"), // Nord Light FG
		FgMuted:     ParseHex("#E5E9F0"), // Nord Brighter FG
		FgHalfMuted: ParseHex("#D8DEE9"), // Nord Light FG
		FgSubtle:    ParseHex("#88C0D0"), // Nord Blue
		FgSelected:  ParseHex("#2E3440"), // Nord Dark BG

		Border:      ParseHex("#4C566A"), // Nord Highlight BG
		BorderFocus: ParseHex("#88C0D0"), // Nord Blue

		Success: ParseHex("#A3BE8C"), // Nord Green
		Error:   ParseHex("#BF616A"), // Nord Red
		Warning: ParseHex("#EBCB8B"), // Nord Yellow
		Info:    ParseHex("#88C0D0"), // Nord Blue

		White: ParseHex("#E5E9F0"), // Nord Brighter FG

		BlueLight: ParseHex("#88C0D0"), // Nord Blue
		Blue:      ParseHex("#5E81AC"), // Nord Dark Blue

		Yellow: ParseHex("#EBCB8B"), // Nord Yellow
		Citron: ParseHex("#D08770"), // Nord Orange

		Green:      ParseHex("#A3BE8C"), // Nord Green
		GreenDark:  ParseHex("#8FBCBB"), // Nord Cyan
		GreenLight: ParseHex("#A3BE8C"), // Nord Green

		Red:      ParseHex("#BF616A"), // Nord Red
		RedDark:  ParseHex("#BF616A"), // Nord Red
		RedLight: ParseHex("#D08770"), // Nord Orange
		Cherry:   ParseHex("#B48EAD"), // Nord Purple

		TextSelection: lipgloss.NewStyle().Background(ParseHex("#88C0D0")).Foreground(ParseHex("#2E3440")),

		ItemOfflineIcon: lipgloss.NewStyle().Foreground(ParseHex("#4C566A")),
		ItemBusyIcon:    lipgloss.NewStyle().Foreground(ParseHex("#EBCB8B")),
		ItemErrorIcon:   lipgloss.NewStyle().Foreground(ParseHex("#BF616A")),
		ItemOnlineIcon:  lipgloss.NewStyle().Foreground(ParseHex("#A3BE8C")),

		YoloIconFocused: lipgloss.NewStyle().Foreground(ParseHex("#BF616A")).Bold(true),
		YoloIconBlurred: lipgloss.NewStyle().Foreground(ParseHex("#4C566A")),
		YoloDotsFocused: lipgloss.NewStyle().Foreground(ParseHex("#BF616A")),
		YoloDotsBlurred: lipgloss.NewStyle().Foreground(ParseHex("#4C566A")),
	}
}

// NewDraculaTheme creates the Dracula theme
func NewDraculaTheme() *Theme {
	return &Theme{
		Name:   "dracula",
		IsDark: true,

		Primary:   ParseHex("#BD93F9"), // Dracula Purple
		Secondary: ParseHex("#FF79C6"), // Dracula Pink
		Tertiary:  ParseHex("#50FA7B"), // Dracula Green
		Accent:    ParseHex("#F1FA8C"), // Dracula Yellow

		BgBase:        ParseHex("#282A36"), // Dracula Background
		BgBaseLighter: ParseHex("#44475A"), // Dracula Current Line
		BgSubtle:      ParseHex("#6272A4"), // Dracula Comment
		BgOverlay:     ParseHex("#44475A"), // Dracula Current Line

		FgBase:      ParseHex("#F8F8F2"), // Dracula Foreground
		FgMuted:     ParseHex("#E9E9F4"), // Lighter Foreground
		FgHalfMuted: ParseHex("#F8F8F2"), // Dracula Foreground
		FgSubtle:    ParseHex("#BD93F9"), // Dracula Purple
		FgSelected:  ParseHex("#282A36"), // Dracula Background

		Border:      ParseHex("#44475A"), // Dracula Current Line
		BorderFocus: ParseHex("#BD93F9"), // Dracula Purple

		Success: ParseHex("#50FA7B"), // Dracula Green
		Error:   ParseHex("#FF5555"), // Dracula Red
		Warning: ParseHex("#F1FA8C"), // Dracula Yellow
		Info:    ParseHex("#BD93F9"), // Dracula Purple

		White: ParseHex("#F8F8F2"), // Dracula Foreground

		BlueLight: ParseHex("#8BE9FD"), // Dracula Cyan
		Blue:      ParseHex("#6272A4"), // Dracula Comment

		Yellow: ParseHex("#F1FA8C"), // Dracula Yellow
		Citron: ParseHex("#FFB86C"), // Dracula Orange

		Green:      ParseHex("#50FA7B"), // Dracula Green
		GreenDark:  ParseHex("#8BE9FD"), // Dracula Cyan
		GreenLight: ParseHex("#50FA7B"), // Dracula Green

		Red:      ParseHex("#FF5555"), // Dracula Red
		RedDark:  ParseHex("#FF5555"), // Dracula Red
		RedLight: ParseHex("#FFB86C"), // Dracula Orange
		Cherry:   ParseHex("#FF79C6"), // Dracula Pink

		TextSelection: lipgloss.NewStyle().Background(ParseHex("#BD93F9")).Foreground(ParseHex("#282A36")),

		ItemOfflineIcon: lipgloss.NewStyle().Foreground(ParseHex("#44475A")),
		ItemBusyIcon:    lipgloss.NewStyle().Foreground(ParseHex("#F1FA8C")),
		ItemErrorIcon:   lipgloss.NewStyle().Foreground(ParseHex("#FF5555")),
		ItemOnlineIcon:  lipgloss.NewStyle().Foreground(ParseHex("#50FA7B")),

		YoloIconFocused: lipgloss.NewStyle().Foreground(ParseHex("#FF5555")).Bold(true),
		YoloIconBlurred: lipgloss.NewStyle().Foreground(ParseHex("#44475A")),
		YoloDotsFocused: lipgloss.NewStyle().Foreground(ParseHex("#FF5555")),
		YoloDotsBlurred: lipgloss.NewStyle().Foreground(ParseHex("#44475A")),
	}
}

// NewMonokaiTheme creates the Monokai theme
func NewMonokaiTheme() *Theme {
	return &Theme{
		Name:   "monokai",
		IsDark: true,

		Primary:   ParseHex("#66D9EF"), // Monokai Blue
		Secondary: ParseHex("#F92672"), // Monokai Pink
		Tertiary:  ParseHex("#A6E22E"), // Monokai Green
		Accent:    ParseHex("#E6DB74"), // Monokai Yellow

		BgBase:        ParseHex("#272822"), // Monokai Background
		BgBaseLighter: ParseHex("#3E3D32"), // Monokai Line Highlight
		BgSubtle:      ParseHex("#75715E"), // Monokai Comment
		BgOverlay:     ParseHex("#3E3D32"), // Monokai Line Highlight

		FgBase:      ParseHex("#F8F8F2"), // Monokai Foreground
		FgMuted:     ParseHex("#F9F8F5"), // Lighter Foreground
		FgHalfMuted: ParseHex("#F8F8F2"), // Monokai Foreground
		FgSubtle:    ParseHex("#66D9EF"), // Monokai Blue
		FgSelected:  ParseHex("#272822"), // Monokai Background

		Border:      ParseHex("#3E3D32"), // Monokai Line Highlight
		BorderFocus: ParseHex("#66D9EF"), // Monokai Blue

		Success: ParseHex("#A6E22E"), // Monokai Green
		Error:   ParseHex("#F92672"), // Monokai Pink
		Warning: ParseHex("#E6DB74"), // Monokai Yellow
		Info:    ParseHex("#66D9EF"), // Monokai Blue

		White: ParseHex("#F8F8F2"), // Monokai Foreground

		BlueLight: ParseHex("#66D9EF"), // Monokai Blue
		Blue:      ParseHex("#75715E"), // Monokai Comment

		Yellow: ParseHex("#E6DB74"), // Monokai Yellow
		Citron: ParseHex("#FD971F"), // Monokai Orange

		Green:      ParseHex("#A6E22E"), // Monokai Green
		GreenDark:  ParseHex("#66D9EF"), // Monokai Blue
		GreenLight: ParseHex("#A6E22E"), // Monokai Green

		Red:      ParseHex("#F92672"), // Monokai Pink
		RedDark:  ParseHex("#F92672"), // Monokai Pink
		RedLight: ParseHex("#FD971F"), // Monokai Orange
		Cherry:   ParseHex("#AE81FF"), // Monokai Purple

		TextSelection: lipgloss.NewStyle().Background(ParseHex("#66D9EF")).Foreground(ParseHex("#272822")),

		ItemOfflineIcon: lipgloss.NewStyle().Foreground(ParseHex("#3E3D32")),
		ItemBusyIcon:    lipgloss.NewStyle().Foreground(ParseHex("#E6DB74")),
		ItemErrorIcon:   lipgloss.NewStyle().Foreground(ParseHex("#F92672")),
		ItemOnlineIcon:  lipgloss.NewStyle().Foreground(ParseHex("#A6E22E")),

		YoloIconFocused: lipgloss.NewStyle().Foreground(ParseHex("#F92672")).Bold(true),
		YoloIconBlurred: lipgloss.NewStyle().Foreground(ParseHex("#3E3D32")),
		YoloDotsFocused: lipgloss.NewStyle().Foreground(ParseHex("#F92672")),
		YoloDotsBlurred: lipgloss.NewStyle().Foreground(ParseHex("#3E3D32")),
	}
}

// NewDefaultTheme creates the default theme (alias for Charmtone)
func NewDefaultTheme() *Theme {
	return NewCharmtoneTheme()
}

// GetAllThemes returns all available theme presets
func GetAllThemes() []*Theme {
	return []*Theme{
		NewDefaultTheme(),
		NewNordTheme(),
		NewDraculaTheme(),
		NewMonokaiTheme(),
	}
}
