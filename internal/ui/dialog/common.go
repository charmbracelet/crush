package dialog

import (
	"cmp"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// dialogInputTextWidth returns the text-area width for a dialog input so
// that the input frame, its prompt (e.g. "> "), the text, and a trailing
// cursor cell all fit within contentWidth. The prompt is rendered outside
// the text area, so it must be subtracted or long values wrap past the
// dialog border.
func dialogInputTextWidth(t *styles.Styles, input textinput.Model, contentWidth int) int {
	const cursorPadding = 1
	return max(0, contentWidth-
		t.Dialog.InputPrompt.GetHorizontalFrameSize()-
		lipgloss.Width(input.Prompt)-
		cursorPadding)
}

// sizer is satisfied by any list type that can report its total content
// height and accept a viewport size. Both *list.List and *list.FilterableList
// (and wrappers embedding them) implement this.
type sizer interface {
	TotalHeight() int
	SetSize(width, height int)
}

// sizeDialogList computes the list dimensions within a dialog and calls
// l.SetSize. It accounts for the title, input, help, and view frame sizes
// so callers don't have to repeat the arithmetic. The scrollbar column is
// reserved only when content overflows the viewport.
//
// Returns listHeight, listTotalHeight, and listWidth for callers that need
// them (e.g. to pass to joinScrollbar or applyInfoColumnVisibility).
//
// Parameters:
//   - t: styles for frame/border measurements.
//   - l: the list to size.
//   - innerWidth: dialog content width (total minus View horizontal frame).
//   - dialogHeight: total dialog content height (already clamped).
func sizeDialogList(t *styles.Styles, l sizer, innerWidth, dialogHeight int) (listHeight, listTotalHeight, listWidth int) {
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	listHeight = max(0, dialogHeight-heightOffset)
	listTotalHeight = l.TotalHeight()

	// Reserve one column for the scrollbar only when it will actually
	// show, so the list otherwise spans the full content width.
	scrollbarWidth := 0
	if listTotalHeight > listHeight {
		scrollbarWidth = 1
	}
	listWidth = max(0, innerWidth-scrollbarWidth)
	l.SetSize(listWidth, listHeight)
	return listHeight, listTotalHeight, listWidth
}

// joinScrollbar appends a vertical scrollbar to the right of view when the
// content overflows its viewport, and returns view unchanged otherwise.
// contentSize is the total content height, viewportSize the visible height,
// and offset the current scroll position.
func joinScrollbar(t *styles.Styles, view string, height, contentSize, viewportSize, offset int) string {
	if sb := common.Scrollbar(t, height, contentSize, viewportSize, offset); sb != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top, view, sb)
	}
	return view
}

// Maximum share of a list row width the secondary info column may take
// before it is hidden entirely, so it never crowds out the item name.
// Command shortcuts are small and non-essential, so they yield sooner
// than the larger, more useful session timestamps.
const (
	sessionInfoMaxPercent = 35
	commandInfoMaxPercent = 25
)

// infoColumnItem is a list item with a secondary info column (a session
// timestamp, a command shortcut) that can be hidden when space is tight.
type infoColumnItem interface {
	// InfoText returns the raw info string, or "" when there is none.
	InfoText() string
	// SetHideInfo toggles whether the info column is rendered.
	SetHideInfo(bool)
}

// applyInfoColumnVisibility hides the secondary info column across every
// item uniformly when its widest entry would take more than maxPercent of
// rowWidth, so item names keep their room. It returns once rowWidth grows
// enough for the widest entry to fit within the budget again.
func applyInfoColumnVisibility(items []list.Item, rowWidth, maxPercent int) {
	widest := 0
	for _, it := range items {
		if ic, ok := it.(infoColumnItem); ok {
			if info := ic.InfoText(); info != "" {
				widest = max(widest, lipgloss.Width(" "+info+" "))
			}
		}
	}
	hide := rowWidth > 0 && widest*100 > rowWidth*maxPercent
	for _, it := range items {
		if ic, ok := it.(infoColumnItem); ok {
			ic.SetHideInfo(hide)
		}
	}
}

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
