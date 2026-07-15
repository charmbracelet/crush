package dialog

import (
	"cmp"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// renderDialogHelp renders keybind hints as a single padded footer line at
// contentWidth (the dialog's inner width: total minus the View border). The
// hints are packed greedily and truncated with an ellipsis so the line never
// wraps or overflows the border, and never ends on a dangling separator.
func renderDialogHelp(t *styles.Styles, h *help.Model, km help.KeyMap, contentWidth int) string {
	textWidth := max(0, contentWidth-t.Dialog.HelpView.GetHorizontalFrameSize())
	return t.Dialog.HelpView.Render(shortHelpLine(h, km.ShortHelp(), textWidth))
}

// shortHelpLine builds a single-line short help view truncated to width.
// It reimplements the bubbles help packing to avoid a component bug where
// items are kept even when they overflow (when the ellipsis itself does not
// fit), and to guarantee the line ends cleanly rather than on a separator.
func shortHelpLine(h *help.Model, bindings []key.Binding, width int) string {
	if width <= 0 {
		return ""
	}
	sep := h.Styles.ShortSeparator.Inline(true).Render(h.ShortSeparator)
	ellipsis := h.Styles.Ellipsis.Inline(true).Render(cmp.Or(h.Ellipsis, "…"))

	var b strings.Builder
	total := 0
	for _, kb := range bindings {
		if !kb.Enabled() {
			continue
		}
		seg := ""
		if total > 0 {
			seg = sep
		}
		seg += h.Styles.ShortKey.Inline(true).Render(kb.Help().Key) + " " +
			h.Styles.ShortDesc.Inline(true).Render(kb.Help().Desc)
		w := lipgloss.Width(seg)
		if total+w > width {
			// The next item doesn't fit; add an ellipsis if there's room.
			// The separator belongs to this dropped item, so what we've
			// written already ends on a real hint, not a dangling dot. A
			// leading space joins the ellipsis to prior hints, but only
			// when there are prior hints.
			tail := ellipsis
			if total > 0 {
				tail = " " + ellipsis
			}
			if total+lipgloss.Width(tail) <= width {
				b.WriteString(tail)
			}
			break
		}
		total += w
		b.WriteString(seg)
	}
	return b.String()
}

// InputCursor adjusts the cursor position for an input field within a dialog.
func InputCursor(t *styles.Styles, cur *tea.Cursor) *tea.Cursor {
	if cur != nil {
		titleStyle := t.Dialog.Title
		dialogStyle := t.Dialog.View
		inputStyle := t.Dialog.InputPrompt
		// Adjust cursor position to account for dialog layout
		cur.X += inputStyle.GetBorderLeftSize() +
			inputStyle.GetMarginLeft() +
			inputStyle.GetPaddingLeft() +
			dialogStyle.GetBorderLeftSize() +
			dialogStyle.GetPaddingLeft() +
			dialogStyle.GetMarginLeft()
		cur.Y += titleStyle.GetVerticalFrameSize() +
			inputStyle.GetBorderTopSize() +
			inputStyle.GetMarginTop() +
			inputStyle.GetPaddingTop() +
			inputStyle.GetBorderBottomSize() +
			inputStyle.GetMarginBottom() +
			inputStyle.GetPaddingBottom() +
			dialogStyle.GetPaddingTop() +
			dialogStyle.GetMarginTop() +
			dialogStyle.GetBorderTopSize()
	}
	return cur
}

// adjustOnboardingInputCursor removes the dialog view frame offset from an
// input cursor. Onboarding dialogs render without Dialog.View frame, while
// InputCursor includes that frame offset for regular dialogs.
func adjustOnboardingInputCursor(t *styles.Styles, cur *tea.Cursor) *tea.Cursor {
	if cur == nil {
		return nil
	}

	dialogStyle := t.Dialog.View
	cur.X -= dialogStyle.GetBorderLeftSize() +
		dialogStyle.GetPaddingLeft() +
		dialogStyle.GetMarginLeft()
	cur.Y -= dialogStyle.GetBorderTopSize() +
		dialogStyle.GetPaddingTop() +
		dialogStyle.GetMarginTop()
	return cur
}

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
		var titleInfoWidth int
		titleInfo := rc.TitleInfo
		if len(titleInfo) > 0 {
			titleInfoWidth = lipgloss.Width(titleInfo)
			// Truncate TitleInfo if it would push past dialog width.
			if titleInfoWidth > contentWidth {
				titleInfo = ansi.Truncate(titleInfo, max(0, contentWidth), "…")
				titleInfoWidth = lipgloss.Width(titleInfo)
			}
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
