package dialog

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	ThemeEditorID              = "theme_editor"
	themeEditorDialogMaxWidth  = 72
	themeEditorDialogMaxHeight = 24
)

type paletteSlot struct {
	name string
	get  func(styles.Palette) string
	set  func(*styles.Palette, string)
}

// ThemeEditor edits the active theme palette with live preview.
type ThemeEditor struct {
	com     *common.Common
	help    help.Model
	input   textinput.Model
	base    string
	palette styles.Palette
	slots   []paletteSlot
	index   int
	scroll  int

	keyMap struct {
		Save     key.Binding
		Next     key.Binding
		Previous key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*ThemeEditor)(nil)

func NewThemeEditor(com *common.Common) *ThemeEditor {
	ed := &ThemeEditor{com: com, base: "charmtone", slots: newPaletteSlots()}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	ed.help = h

	ed.input = textinput.New()
	ed.input.SetVirtualCursor(false)
	ed.input.SetStyles(com.Styles.TextInput)
	ed.input.Prompt = ""
	ed.input.Focus()

	ed.keyMap.Save = key.NewBinding(key.WithKeys("enter", "ctrl+s"), key.WithHelp("enter", "save"))
	ed.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n", "tab"), key.WithHelp("↓", "next"))
	ed.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p", "shift+tab"), key.WithHelp("↑", "previous"))
	ed.keyMap.Close = CloseKey

	ed.loadCurrentTheme()
	ed.syncInput()
	return ed
}

func (ed *ThemeEditor) ID() string {
	return ThemeEditorID
}

func (ed *ThemeEditor) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, ed.keyMap.Close):
			return ActionRevertThemePalette{}
		case key.Matches(msg, ed.keyMap.Save):
			ed.applyInput()
			return ActionSaveThemePalette{Base: ed.base, Palette: ed.palette}
		case key.Matches(msg, ed.keyMap.Previous):
			ed.applyInput()
			if ed.index == 0 {
				ed.index = len(ed.slots) - 1
			} else {
				ed.index--
			}
			ed.syncInput()
			ed.keepSelectedVisible(0)
			return ActionPreviewThemePalette{Base: ed.base, Palette: ed.palette}
		case key.Matches(msg, ed.keyMap.Next):
			ed.applyInput()
			ed.index = (ed.index + 1) % len(ed.slots)
			ed.syncInput()
			ed.keepSelectedVisible(0)
			return ActionPreviewThemePalette{Base: ed.base, Palette: ed.palette}
		default:
			ed.input, _ = ed.input.Update(msg)
			ed.applyInput()
			return ActionPreviewThemePalette{Base: ed.base, Palette: ed.palette}
		}
	}
	return nil
}

func (ed *ThemeEditor) Cursor() *tea.Cursor {
	return ed.input.Cursor()
}

func (ed *ThemeEditor) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := ed.com.Styles
	width := max(0, min(themeEditorDialogMaxWidth, area.Dx()))
	height := max(0, min(themeEditorDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	listHeight := max(1, height-heightOffset)
	ed.keepSelectedVisible(listHeight)

	listWidth := max(0, innerWidth-3) // Reserve space for scrollbar.
	// Compute label prefix width: 20-char name + space + swatch + space
	labelPrefix := fmt.Sprintf("%-20s %s ", "", styles.ColorSwatchIcon)
	inputPrefixWidth := lipgloss.Width(labelPrefix)
	inputWidth := listWidth - t.Dialog.NormalItem.GetPaddingLeft() - t.Dialog.NormalItem.GetPaddingRight() - inputPrefixWidth
	ed.input.SetWidth(max(0, inputWidth))
	ed.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = fmt.Sprintf("Edit Theme: %s", ed.base)
	rc.Gap = 1
	listView := t.Dialog.List.MarginBottom(0).Height(listHeight).Render(ed.renderSlots(listWidth, listHeight))
	scrollbar := common.Scrollbar(t, listHeight, len(ed.slots), listHeight, ed.scroll)
	if scrollbar != "" {
		listView = lipgloss.JoinHorizontal(lipgloss.Top, listView, scrollbar)
	}
	rc.AddPart(listView)
	rc.Help = ed.help.View(ed)

	view := rc.Render()

	// Position cursor at the inline input within the selected slot.
	cur := ed.Cursor()
	if cur != nil {
		selectedRow := ed.index - ed.scroll
		cur.Y += t.Dialog.View.GetBorderTopSize() +
			t.Dialog.View.GetPaddingTop() +
			t.Dialog.View.GetMarginTop() +
			titleContentHeight +
			rc.Gap +
			t.Dialog.List.GetBorderTopSize() +
			t.Dialog.List.GetPaddingTop() +
			t.Dialog.List.GetMarginTop() +
			selectedRow
		cur.X += t.Dialog.View.GetBorderLeftSize() +
			t.Dialog.View.GetPaddingLeft() +
			t.Dialog.View.GetMarginLeft() +
			t.Dialog.List.GetBorderLeftSize() +
			t.Dialog.List.GetPaddingLeft() +
			t.Dialog.List.GetMarginLeft() +
			t.Dialog.NormalItem.GetBorderLeftSize() +
			t.Dialog.NormalItem.GetPaddingLeft() +
			t.Dialog.NormalItem.GetMarginLeft() +
			inputPrefixWidth
	}
	
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (ed *ThemeEditor) ShortHelp() []key.Binding {
	return []key.Binding{ed.keyMap.Previous, ed.keyMap.Next, ed.keyMap.Save, ed.keyMap.Close}
}

func (ed *ThemeEditor) FullHelp() [][]key.Binding {
	return [][]key.Binding{{ed.keyMap.Previous, ed.keyMap.Next, ed.keyMap.Save, ed.keyMap.Close}}
}

func (ed *ThemeEditor) loadCurrentTheme() {
	cfg := ed.com.Config()
	if cfg == nil || cfg.Options == nil || cfg.Options.TUI == nil {
		ed.loadBuiltin("charmtone")
		return
	}
	activeTheme := cfg.Options.TUI.ActiveTheme
	if activeTheme == "" {
		activeTheme = "charmtone"
	}
	theme, ok := cfg.Options.TUI.Theme[activeTheme]
	if !ok {
		ed.loadBuiltin(activeTheme)
		return
	}
	if theme.IsObject() {
		ed.loadObject(theme)
		return
	}
	ed.loadBuiltin(activeTheme)
}

func (ed *ThemeEditor) loadBuiltin(name string) {
	p, err := styles.ThemePalette(name)
	if err != nil {
		name = "charmtone"
		p, _ = styles.ThemePalette(name)
	}
	ed.base = name
	ed.palette = p
}

func (ed *ThemeEditor) loadObject(theme config.ThemeConfig) {
	var custom struct {
		Base string `json:"base,omitempty"`
		styles.Palette
	}
	if err := json.Unmarshal(theme.RawObject, &custom); err != nil {
		ed.loadBuiltin(theme.Name())
		return
	}
	ed.base = custom.Base
	if ed.base == "" {
		ed.base = "charmtone"
	}
	merged, err := styles.MergePalette(ed.base, custom.Palette)
	if err != nil {
		ed.loadBuiltin(ed.base)
		return
	}
	ed.palette = merged
}

func (ed *ThemeEditor) selectedSlot() paletteSlot {
	return ed.slots[ed.index]
}

func (ed *ThemeEditor) applyInput() {
	raw := strings.TrimSpace(ed.input.Value())
	if resolved := styles.ParseColor(raw); resolved != "" {
		ed.selectedSlot().set(&ed.palette, resolved)
	}
}

func (ed *ThemeEditor) syncInput() {
	ed.input.SetValue(ed.selectedSlot().get(ed.palette))
	ed.input.CursorEnd()
}

func (ed *ThemeEditor) keepSelectedVisible(height int) {
	if height <= 0 {
		height = themeEditorDialogMaxHeight
	}
	if ed.index < ed.scroll {
		ed.scroll = ed.index
	}
	if ed.index >= ed.scroll+height {
		ed.scroll = ed.index - height + 1
	}
	if ed.scroll < 0 {
		ed.scroll = 0
	}
}

func (ed *ThemeEditor) renderSlots(width, height int) string {
	end := min(len(ed.slots), ed.scroll+height)
	lines := make([]string, 0, end-ed.scroll)
	for i := ed.scroll; i < end; i++ {
		slot := ed.slots[i]
		selected := i == ed.index

		var swatch string
		{
			value := slot.get(ed.palette)
			colorStr := styles.ColorString(value)
			swatch = lipgloss.NewStyle().Foreground(lipgloss.Color(colorStr)).Render(styles.ColorSwatchIcon)
		}

		var line string
		if selected {
			// Render input inline at the value position.
			label := fmt.Sprintf("%-20s %s ", slot.name, swatch)
			line = ed.com.Styles.Dialog.NormalItem.Width(width).Render(label + ed.input.View())
		} else {
			value := slot.get(ed.palette)
			label := fmt.Sprintf("%-20s %s %s", slot.name, swatch, value)
			line = ed.com.Styles.Dialog.NormalItem.Width(width).Render(label)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func newPaletteSlots() []paletteSlot {
	return []paletteSlot{
		{name: "primary", get: func(p styles.Palette) string { return p.Primary }, set: func(p *styles.Palette, v string) { p.Primary = v }},
		{name: "secondary", get: func(p styles.Palette) string { return p.Secondary }, set: func(p *styles.Palette, v string) { p.Secondary = v }},
		{name: "accent", get: func(p styles.Palette) string { return p.Accent }, set: func(p *styles.Palette, v string) { p.Accent = v }},
		{name: "keyword", get: func(p styles.Palette) string { return p.Keyword }, set: func(p *styles.Palette, v string) { p.Keyword = v }},
		{name: "fg_base", get: func(p styles.Palette) string { return p.FgBase }, set: func(p *styles.Palette, v string) { p.FgBase = v }},
		{name: "fg_subtle", get: func(p styles.Palette) string { return p.FgSubtle }, set: func(p *styles.Palette, v string) { p.FgSubtle = v }},
		{name: "fg_more_subtle", get: func(p styles.Palette) string { return p.FgMoreSubtle }, set: func(p *styles.Palette, v string) { p.FgMoreSubtle = v }},
		{name: "fg_most_subtle", get: func(p styles.Palette) string { return p.FgMostSubtle }, set: func(p *styles.Palette, v string) { p.FgMostSubtle = v }},
		{name: "bg_base", get: func(p styles.Palette) string { return p.BgBase }, set: func(p *styles.Palette, v string) { p.BgBase = v }},
		{name: "bg_most_visible", get: func(p styles.Palette) string { return p.BgMostVisible }, set: func(p *styles.Palette, v string) { p.BgMostVisible = v }},
		{name: "bg_less_visible", get: func(p styles.Palette) string { return p.BgLessVisible }, set: func(p *styles.Palette, v string) { p.BgLessVisible = v }},
		{name: "bg_least_visible", get: func(p styles.Palette) string { return p.BgLeastVisible }, set: func(p *styles.Palette, v string) { p.BgLeastVisible = v }},
		{name: "on_primary", get: func(p styles.Palette) string { return p.OnPrimary }, set: func(p *styles.Palette, v string) { p.OnPrimary = v }},
		{name: "separator", get: func(p styles.Palette) string { return p.Separator }, set: func(p *styles.Palette, v string) { p.Separator = v }},
		{name: "destructive", get: func(p styles.Palette) string { return p.Destructive }, set: func(p *styles.Palette, v string) { p.Destructive = v }},
		{name: "error", get: func(p styles.Palette) string { return p.Error }, set: func(p *styles.Palette, v string) { p.Error = v }},
		{name: "warning", get: func(p styles.Palette) string { return p.Warning }, set: func(p *styles.Palette, v string) { p.Warning = v }},
		{name: "warning_subtle", get: func(p styles.Palette) string { return p.WarningSubtle }, set: func(p *styles.Palette, v string) { p.WarningSubtle = v }},
		{name: "denied", get: func(p styles.Palette) string { return p.Denied }, set: func(p *styles.Palette, v string) { p.Denied = v }},
		{name: "busy", get: func(p styles.Palette) string { return p.Busy }, set: func(p *styles.Palette, v string) { p.Busy = v }},
		{name: "info", get: func(p styles.Palette) string { return p.Info }, set: func(p *styles.Palette, v string) { p.Info = v }},
		{name: "info_more_subtle", get: func(p styles.Palette) string { return p.InfoMoreSubtle }, set: func(p *styles.Palette, v string) { p.InfoMoreSubtle = v }},
		{name: "info_most_subtle", get: func(p styles.Palette) string { return p.InfoMostSubtle }, set: func(p *styles.Palette, v string) { p.InfoMostSubtle = v }},
		{name: "success", get: func(p styles.Palette) string { return p.Success }, set: func(p *styles.Palette, v string) { p.Success = v }},
		{name: "success_more_subtle", get: func(p styles.Palette) string { return p.SuccessMoreSubtle }, set: func(p *styles.Palette, v string) { p.SuccessMoreSubtle = v }},
		{name: "success_most_subtle", get: func(p styles.Palette) string { return p.SuccessMostSubtle }, set: func(p *styles.Palette, v string) { p.SuccessMostSubtle = v }},
	}
}

var _ help.KeyMap = (*ThemeEditor)(nil)
