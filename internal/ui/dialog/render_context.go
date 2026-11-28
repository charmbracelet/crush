package dialog

import (
	"image/color"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// RenderContext is a dialog rendering context that can be used to render
// common dialog layouts.
type RenderContext struct {
	// Styles is the styles to use for rendering.
	Styles *styles.Styles
	// TitleStyle is the style of the dialog title by default it uses Styles.Dialog.Title
	TitleStyle lipgloss.Style
	// ViewStyle is the style of the dialog title by default it uses Styles.Dialog.View
	ViewStyle lipgloss.Style
	// TitleGradientFromColor is the color the title gradient starts by default
	// its Styles.Dialog.TitleGradFromColor
	TitleGradientFromColor color.Color
	// TitleGradientToColor is the color the title gradient ends by default its
	// Styles.Dialog.TitleGradToColor
	TitleGradientToColor color.Color
	// Width is the total width of the dialog including any margins, borders,
	// and paddings.
	Width int
	// Gap is the gap between content parts. Zero means no gap.
	Gap int
	// Title is the title of the dialog. This will be styled using the default
	// dialog title style and prepended to the content parts slice.
	Title string
	// TitleInfo is additional information to display next to the title. This
	// part is displayed as is, any styling must be applied before setting this
	// field.
	TitleInfo string
	// Parts are the rendered parts of the dialog.
	Parts []string
	// Help is the fully rendered help footer line. Produce it with
	// renderDialogHelp so it is sized and padded consistently; it is
	// appended as-is without further styling.
	Help string
	// IsOnboarding indicates whether to render the dialog as part of the
	// onboarding flow. This means that the content will be rendered at the
	// bottom left of the screen.
	IsOnboarding bool
}

// NewRenderContext creates a new RenderContext with the provided styles and width.
func NewRenderContext(t *styles.Styles, width int) *RenderContext {
	return &RenderContext{
		Styles:                 t,
		TitleStyle:             t.Dialog.Title,
		ViewStyle:              t.Dialog.View,
		TitleGradientFromColor: t.Dialog.TitleGradFromColor,
		TitleGradientToColor:   t.Dialog.TitleGradToColor,
		Width:                  width,
		Parts:                  []string{},
	}
}

// AddPart adds a rendered part to the dialog.
func (rc *RenderContext) AddPart(part string) {
	if len(part) > 0 {
		rc.Parts = append(rc.Parts, part)
	}
}

// Render renders the dialog using the provided context.
func (rc *RenderContext) Render() string {
	titleStyle := rc.TitleStyle
	dialogStyle := rc.ViewStyle.Width(rc.Width)

	var parts []string

	if len(rc.Title) > 0 {
		contentWidth := rc.Width - dialogStyle.GetHorizontalFrameSize() -
			titleStyle.GetHorizontalFrameSize()
		titleInfo := rc.TitleInfo
		titleInfoWidth := lipgloss.Width(titleInfo)
		// Drop the title info entirely when it can't sit beside the title
		// text with at least a one-cell gap. Title info is often styled
		// (e.g. radio toggles with backgrounds and padding); truncating it
		// mid-segment leaves broken colored fragments, so hide it instead.
		if titleInfoWidth > 0 && lipgloss.Width(rc.Title)+1+titleInfoWidth > contentWidth {
			titleInfo = ""
			titleInfoWidth = 0
		}
		title := common.DialogTitle(rc.Styles, rc.Title,
			max(0, contentWidth-titleInfoWidth), rc.TitleGradientFromColor, rc.TitleGradientToColor)
		if len(titleInfo) > 0 {
			title += titleInfo
		}
		parts = append(parts, titleStyle.Render(title))
		if rc.Gap > 0 {
			parts = append(parts, make([]string, rc.Gap)...)
		}
	}

	if rc.Gap <= 0 {
		parts = append(parts, rc.Parts...)
	} else {
		for i, p := range rc.Parts {
			if len(p) > 0 {
				parts = append(parts, p)
			}
			if i < len(rc.Parts)-1 {
				parts = append(parts, make([]string, rc.Gap)...)
			}
		}
	}

	if len(rc.Help) > 0 {
		if rc.Gap > 0 {
			parts = append(parts, make([]string, rc.Gap)...)
		}
		parts = append(parts, rc.Help)
	}

	content := strings.Join(parts, "\n")
	if rc.IsOnboarding {
		return content
	}
	return dialogStyle.Render(content)
}

// ChromeHeight is the height Render adds around one more body part: title, gaps, parts so far, help and frame.
func (rc *RenderContext) ChromeHeight() int {
	probe := *rc
	probe.Parts = append(slices.Clip(rc.Parts), " ")
	return lipgloss.Height(probe.Render()) - 1
}
