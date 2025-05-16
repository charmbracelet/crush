package styles

import (
	"image/color"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

var ImageBakcground = "#212121"

// Style generation functions that use the current theme

// BaseStyle returns the base style with background and foreground colors
func BaseStyle() lipgloss.Style {
	t := theme.CurrentTheme()
	return lipgloss.NewStyle().
		Background(t.Background()).
		Foreground(t.Text())
}

// Regular returns a basic unstyled lipgloss.Style
func Regular() lipgloss.Style {
	return lipgloss.NewStyle()
}

// Bold returns a bold style
func Bold() lipgloss.Style {
	return Regular().Bold(true)
}

// Padded returns a style with horizontal padding
func Padded() lipgloss.Style {
	return Regular().Padding(0, 1)
}

// Border returns a style with a normal border
func Border() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.BorderNormal())
}

// ThickBorder returns a style with a thick border
func ThickBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.ThickBorder()).
		BorderForeground(t.BorderNormal())
}

// DoubleBorder returns a style with a double border
func DoubleBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.BorderNormal())
}

// FocusedBorder returns a style with a border using the focused border color
func FocusedBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.BorderFocused())
}

// DimBorder returns a style with a border using the dim border color
func DimBorder() lipgloss.Style {
	t := theme.CurrentTheme()
	return Regular().
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.BorderDim())
}

// PrimaryColor returns the primary color from the current theme
func PrimaryColor() color.Color {
	return theme.CurrentTheme().Primary()
}

// SecondaryColor returns the secondary color from the current theme
func SecondaryColor() color.Color {
	return theme.CurrentTheme().Secondary()
}

// AccentColor returns the accent color from the current theme
func AccentColor() color.Color {
	return theme.CurrentTheme().Accent()
}

// ErrorColor returns the error color from the current theme
func ErrorColor() color.Color {
	return theme.CurrentTheme().Error()
}

// WarningColor returns the warning color from the current theme
func WarningColor() color.Color {
	return theme.CurrentTheme().Warning()
}

// SuccessColor returns the success color from the current theme
func SuccessColor() color.Color {
	return theme.CurrentTheme().Success()
}

// InfoColor returns the info color from the current theme
func InfoColor() color.Color {
	return theme.CurrentTheme().Info()
}

// TextColor returns the text color from the current theme
func TextColor() color.Color {
	return theme.CurrentTheme().Text()
}

// TextMutedColor returns the muted text color from the current theme
func TextMutedColor() color.Color {
	return theme.CurrentTheme().TextMuted()
}

// TextEmphasizedColor returns the emphasized text color from the current theme
func TextEmphasizedColor() color.Color {
	return theme.CurrentTheme().TextEmphasized()
}

// BackgroundColor returns the background color from the current theme
func BackgroundColor() color.Color {
	return theme.CurrentTheme().Background()
}

// BackgroundSecondaryColor returns the secondary background color from the current theme
func BackgroundSecondaryColor() color.Color {
	return theme.CurrentTheme().BackgroundSecondary()
}

// BackgroundDarkerColor returns the darker background color from the current theme
func BackgroundDarkerColor() color.Color {
	return theme.CurrentTheme().BackgroundDarker()
}

// BorderNormalColor returns the normal border color from the current theme
func BorderNormalColor() color.Color {
	return theme.CurrentTheme().BorderNormal()
}

// BorderFocusedColor returns the focused border color from the current theme
func BorderFocusedColor() color.Color {
	return theme.CurrentTheme().BorderFocused()
}

// BorderDimColor returns the dim border color from the current theme
func BorderDimColor() color.Color {
	return theme.CurrentTheme().BorderDim()
}
