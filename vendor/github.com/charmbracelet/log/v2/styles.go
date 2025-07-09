package log

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

// Styles defines the styles for the text logger.
type Styles struct {
	// Timestamp is the style for timestamps.
	Timestamp lipgloss.Style

	// Caller is the style for source caller.
	Caller lipgloss.Style

	// Prefix is the style for prefix.
	Prefix lipgloss.Style

	// Message is the style for messages.
	Message lipgloss.Style

	// Key is the style for keys.
	Key lipgloss.Style

	// Value is the style for values.
	Value lipgloss.Style

	// Separator is the style for separators.
	Separator lipgloss.Style

	// Levels are the styles for each level.
	Levels map[Level]lipgloss.Style

	// Keys overrides styles for specific keys.
	Keys map[string]lipgloss.Style

	// Values overrides value styles for specific keys.
	Values map[string]lipgloss.Style
}

// DefaultStyles returns the default styles.
func DefaultStyles() *Styles {
	// TODO handle this based on light/dark colors
	return &Styles{
		Timestamp: lipgloss.NewStyle(),
		Caller:    lipgloss.NewStyle().Faint(true),
		Prefix:    lipgloss.NewStyle().Bold(true).Faint(true),
		Message:   lipgloss.NewStyle(),
		Key:       lipgloss.NewStyle().Faint(true),
		Value:     lipgloss.NewStyle(),
		Separator: lipgloss.NewStyle().Faint(true),
		Levels: map[Level]lipgloss.Style{
			DebugLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(DebugLevel.String())).
				Bold(true).
				MaxWidth(4).
				Foreground(lipgloss.Color("63")),
			InfoLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(InfoLevel.String())).
				Bold(true).
				MaxWidth(4).
				Foreground(lipgloss.Color("86")),
			WarnLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(WarnLevel.String())).
				Bold(true).
				MaxWidth(4).
				Foreground(lipgloss.Color("192")),
			ErrorLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(ErrorLevel.String())).
				Bold(true).
				MaxWidth(4).
				Foreground(lipgloss.Color("204")),
			FatalLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(FatalLevel.String())).
				Bold(true).
				MaxWidth(4).
				Foreground(lipgloss.Color("134")),
		},
		Keys:   map[string]lipgloss.Style{},
		Values: map[string]lipgloss.Style{},
	}
}
