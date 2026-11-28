package dialog

import (
	"image"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// sizeDialogInput fits input to contentWidth, re-scrolls it and returns its text width.
func sizeDialogInput(t *styles.Styles, input *textinput.Model, contentWidth int) int {
	const cursorPadding = 1
	w := max(0, contentWidth-
		t.Dialog.InputPrompt.GetHorizontalFrameSize()-
		lipgloss.Width(input.Prompt)-
		cursorPadding)
	if w != input.Width() {
		input.SetWidth(w)
		input.SetCursor(input.Position())
	}
	return w
}

// sizer is a list that reports its content height and accepts a viewport size.
type sizer interface {
	TotalHeight() int
	SetSize(width, height int)
}

// pickerList is a list a picker dialog can size, render and scroll.
type pickerList interface {
	sizer
	Render() string
	Offset() int
}

// sizeDialogList sizes l to bodyHeight less the list frame, reserving a scrollbar column only when it overflows.
func sizeDialogList(t *styles.Styles, l sizer, innerWidth, bodyHeight int) (listHeight, listTotalHeight, listWidth int) {
	listHeight = max(0, bodyHeight-t.Dialog.List.GetVerticalFrameSize())
	listTotalHeight = l.TotalHeight()
	scrollbarWidth := 0
	if listTotalHeight > listHeight {
		scrollbarWidth = 1
	}
	listWidth = max(0, innerWidth-scrollbarWidth)
	l.SetSize(listWidth, listHeight)
	return listHeight, listTotalHeight, listWidth
}

// renderPickerList sizes l into bodyHeight and renders it with its scrollbar, or emptyMsg when empty; prepare runs once it is sized. It returns "" when there is no room for rows.
func renderPickerList(t *styles.Styles, l pickerList, innerWidth, bodyHeight int, empty bool, emptyMsg string, prepare func(listWidth int)) (view string, listWidth, listHeight int) {
	listHeight, listTotalHeight, listWidth := sizeDialogList(t, l, innerWidth, bodyHeight)
	if prepare != nil {
		prepare(listWidth)
	}
	if listHeight <= 0 {
		return "", listWidth, listHeight
	}
	body := l.Render()
	if empty {
		body = dialogEmptyState(t, emptyMsg, listWidth)
	}
	view = t.Dialog.List.Height(listHeight).Render(body)
	return joinScrollbar(t, view, listHeight, listTotalHeight, listHeight, l.Offset()), listWidth, listHeight
}

// pickerFrame lays out a picker dialog: title, filter input, list and help.
type pickerFrame struct {
	*RenderContext
	input      *textinput.Model
	inputWidth int
	innerWidth int
	// height is the total dialog height, border included.
	height     int
	listView   string
	listHeight int
}

// newPickerFrame sizes a picker to area within maxWidth and maxHeight (border excluded), fitting input and rendering the help line.
func newPickerFrame(t *styles.Styles, area uv.Rectangle, maxWidth, maxHeight int, input *textinput.Model, h *help.Model, km help.KeyMap) *pickerFrame {
	width := max(0, min(maxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	f := &pickerFrame{
		RenderContext: NewRenderContext(t, width),
		input:         input,
		innerWidth:    width - t.Dialog.View.GetHorizontalFrameSize(),
	}
	f.fitHeight(area, maxHeight)
	f.inputWidth = sizeDialogInput(t, input, f.innerWidth)
	f.Help = renderDialogHelp(t, h, km, f.innerWidth)
	return f
}

// fitHeight sets the dialog height to maxHeight (border excluded), clamped to area.
func (f *pickerFrame) fitHeight(area uv.Rectangle, maxHeight int) {
	border := f.Styles.Dialog.View.GetVerticalBorderSize()
	f.height = max(0, min(maxHeight, area.Dy()-border)) + border
}

// addInput adds the filter input.
func (f *pickerFrame) addInput() {
	f.AddPart(f.Styles.Dialog.InputPrompt.Render(f.input.View()))
}

// addList adds l in the height left below the parts so far; see renderPickerList.
func (f *pickerFrame) addList(l pickerList, empty bool, emptyMsg string, prepare func(listWidth int)) {
	f.listView, _, f.listHeight = renderPickerList(f.Styles, l, f.innerWidth, f.height-f.ChromeHeight(), empty, emptyMsg, prepare)
	f.AddPart(f.listView)
}

// listArea returns the screen area of the list rows in view drawn into area, or an empty rectangle without a list.
func (f *pickerFrame) listArea(area uv.Rectangle, view string) image.Rectangle {
	if f.listView == "" {
		return image.Rectangle{}
	}
	viewStyle, bodyStyle := f.ViewStyle, f.Styles.Dialog.List
	viewWidth, viewHeight := lipgloss.Size(view)
	dialogArea := common.CenterRect(area, min(viewWidth, area.Dx()), min(viewHeight, area.Dy()))
	if f.IsOnboarding {
		dialogArea = common.BottomLeftRect(area, min(viewWidth, area.Dx()), min(viewHeight, area.Dy()))
		viewStyle = lipgloss.NewStyle()
	}
	top := dialogArea.Max.Y -
		viewStyle.GetMarginBottom() -
		viewStyle.GetBorderBottomSize() -
		viewStyle.GetPaddingBottom() -
		lipgloss.Height(f.Help) -
		lipgloss.Height(f.listView)
	minPt := image.Pt(
		dialogArea.Min.X+
			viewStyle.GetMarginLeft()+
			viewStyle.GetBorderLeftSize()+
			viewStyle.GetPaddingLeft()+
			bodyStyle.GetMarginLeft()+
			bodyStyle.GetBorderLeftSize()+
			bodyStyle.GetPaddingLeft(),
		top+
			bodyStyle.GetMarginTop()+
			bodyStyle.GetBorderTopSize()+
			bodyStyle.GetPaddingTop(),
	)
	return image.Rect(minPt.X, minPt.Y, minPt.X+f.innerWidth, minPt.Y+f.listHeight).
		Intersect(dialogArea).Intersect(area)
}

// Empty-state messages for picker lists.
const (
	emptyNoMatches = "No matches."
)

// dialogEmptyState renders msg as one padded, subtle line truncated to width.
func dialogEmptyState(t *styles.Styles, msg string, width int) string {
	st := t.Dialog.SecondaryText
	textWidth := max(0, width-st.GetHorizontalFrameSize())
	return st.Render(ansi.Truncate(msg, textWidth, "…"))
}

// joinScrollbar appends a scrollbar right of view when contentSize overflows viewportSize.
func joinScrollbar(t *styles.Styles, view string, height, contentSize, viewportSize, offset int) string {
	if sb := common.Scrollbar(t, height, contentSize, viewportSize, offset); sb != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top, view, sb)
	}
	return view
}

// infoColumnItem is a list item with a hideable info column, such as a timestamp or shortcut.
type infoColumnItem interface {
	// InfoText returns the raw info string, or "" when there is none.
	InfoText() string
	// SetHideInfo toggles whether the info column is rendered.
	SetHideInfo(bool)
}

// applyInfoColumnVisibility hides every item's info column when the widest would exceed maxPercent of rowWidth.
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
