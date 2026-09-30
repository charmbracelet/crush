package dialog

import (
	"context"
	"net/http"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// RouterModelID is the identifier for the router decision-model picker,
// opened from the Model row in Router Settings.
const RouterModelID = "router-model"

const routerModelListTimeout = 10 * time.Second

// routerModelsLoadedMsg carries OpenRouter's live decision-model catalog
// back to the RouterModel dialog.
type routerModelsLoadedMsg struct {
	presets []router.Preset
	err     error
}

// RouterModel lets the user pick the router's decision model (Jev, Kev,
// Laya, ...) for the configured provider, the same way the regular model
// switcher works for chat models. OpenRouter's list is fetched live;
// other providers offer the presets from their docs. Whatever is typed in
// the filter can also be used verbatim, so nothing is locked to the list.
//
// This is deliberately distinct from the Router Model Pool picker: this
// dialog chooses the single System One model that classifies each prompt,
// while the pool lists the chat models the router may switch the
// conversation to.
type RouterModel struct {
	com      *common.Common
	provider string
	list     *list.List
	input    textinput.Model
	help     help.Model
	items    []*routerPresetItem
	status   string

	keyMap struct {
		Select   key.Binding
		UpDown   key.Binding
		Next     key.Binding
		Previous key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*RouterModel)(nil)

// NewRouterModel builds the picker for the currently configured router
// provider.
func NewRouterModel(com *common.Common) *RouterModel {
	provider := "openrouter"
	if ro := com.Config().Options.Router; ro != nil && ro.Provider != "" {
		provider = ro.Provider
	}

	l := list.NewList()
	l.RegisterRenderCallback(list.FocusedRenderCallback(l))
	l.Focus()

	t := com.Styles
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Placeholder = "Filter decision models or type an id"
	input.SetStyles(t.TextInput)
	input.Focus()

	h := help.New()
	h.Styles = t.DialogHelpStyles()

	d := &RouterModel{com: com, provider: provider, list: l, input: input, help: h}
	d.keyMap.Select = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
	d.keyMap.UpDown = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose"))
	d.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next"))
	d.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous"))
	d.keyMap.Close = CloseKey

	if provider == "openrouter" {
		d.status = "Loading decision models from OpenRouter..."
	}
	d.setPresets(router.Backends[provider].Presets)
	return d
}

// Init returns the command that fetches OpenRouter's live catalog, or nil
// for providers whose presets are static.
func (d *RouterModel) Init() tea.Cmd {
	if d.provider != "openrouter" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), routerModelListTimeout)
		defer cancel()
		presets, err := router.ListOpenRouterDecisionModels(ctx, &http.Client{Timeout: routerModelListTimeout})
		return routerModelsLoadedMsg{presets: presets, err: err}
	}
}

// ID implements Dialog.
func (d *RouterModel) ID() string {
	return RouterModelID
}

// dialogTitle names this picker after what it chooses — the decision
// model — so it reads differently from the model-pool picker's title.
func (d *RouterModel) dialogTitle() string {
	return "Decision Model (" + d.provider + ")"
}

// purposeLine explains, in the dialog itself, that this picker chooses
// the classifier model rather than a chat model.
func (d *RouterModel) purposeLine() string {
	return "System One model that classifies each prompt. Not a chat model."
}

func (d *RouterModel) setPresets(presets []router.Preset) {
	d.items = make([]*routerPresetItem, 0, len(presets))
	for _, p := range presets {
		d.items = append(d.items, newRouterPresetItem(d.com.Styles, p))
	}
	d.applyFilter()
}

// applyFilter shows the presets matching the typed text, plus a trailing
// "use as typed" entry whenever the text isn't already an exact model id.
func (d *RouterModel) applyFilter() {
	raw := strings.TrimSpace(d.input.Value())
	query := strings.ToLower(raw)
	visible := make([]list.Item, 0, len(d.items)+1)
	exact := false
	for _, it := range d.items {
		if strings.EqualFold(it.preset.Model, raw) {
			exact = true
		}
		if query == "" ||
			strings.Contains(strings.ToLower(it.preset.Model), query) ||
			strings.Contains(strings.ToLower(it.preset.Label), query) {
			visible = append(visible, it)
		}
	}
	if raw != "" && !exact {
		visible = append(visible, newRouterPresetItem(d.com.Styles, router.Preset{Label: "Use as typed", Model: raw}))
	}
	d.list.SetItems(visible...)
	d.list.SelectFirst()
	d.list.ScrollToTop()
}

func (d *RouterModel) selected() *routerPresetItem {
	item, _ := d.list.SelectedItem().(*routerPresetItem)
	return item
}

// HandleMsg implements Dialog.
func (d *RouterModel) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case routerModelsLoadedMsg:
		if msg.err != nil {
			d.status = "Could not load OpenRouter models; type a model id instead"
			return nil
		}
		d.status = ""
		d.setPresets(msg.presets)
		return nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Previous):
			d.list.SelectPrev()
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Next):
			d.list.SelectNext()
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Select):
			if item := d.selected(); item != nil {
				return d.persist(item.preset)
			}
		default:
			prev := d.input.Value()
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			if d.input.Value() != prev {
				d.applyFilter()
			}
			return ActionCmd{cmd}
		}
	}
	return nil
}

// persist writes the chosen model, plus the preset's base URL when it has
// one (local servers), then closes. The API key is never touched.
func (d *RouterModel) persist(p router.Preset) Action {
	if p.BaseURL != "" {
		if err := d.com.Workspace.SetConfigField(config.ScopeGlobal, "options.router.base_url", p.BaseURL); err != nil {
			return ActionCmd{util.ReportError(err)}
		}
	}
	if err := d.com.Workspace.SetConfigField(config.ScopeGlobal, "options.router.model", p.Model); err != nil {
		return ActionCmd{util.ReportError(err)}
	}
	return ActionClose{}
}

// Cursor implements Dialog.
func (d *RouterModel) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *RouterModel) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(routerModelDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	d.input.SetWidth(dialogInputTextWidth(t, d.input, innerWidth))

	// The purpose line below the title, and the status line when one is
	// showing, are rows the list budget does not know about, so reserve
	// them here rather than overflowing the dialog.
	extraRows := 1
	if d.status != "" {
		extraRows++
	}
	listHeight, listTotalHeight, _ := sizeDialogList(t, d.list, innerWidth, max(0, height-extraRows))

	rc := NewRenderContext(t, width)
	rc.Title = d.dialogTitle()
	rc.AddPart(t.Dialog.ListItem.InfoBlurred.Render(d.purposeLine()))
	rc.AddPart(t.Dialog.InputPrompt.Render(d.input.View()))
	if d.status != "" {
		rc.AddPart(t.Dialog.ListItem.InfoBlurred.Render(d.status))
	}
	listView := t.Dialog.List.Height(d.list.Height()).Render(d.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, d.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)

	// The purpose line rendered above the input shifts it down a row,
	// which InputCursor does not know about.
	cur := offsetInputCursor(d.Cursor(), 1)
	DrawCenterCursor(scr, area, rc.Render(), cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *RouterModel) ShortHelp() []key.Binding {
	return []key.Binding{d.keyMap.UpDown, d.keyMap.Select, d.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (d *RouterModel) FullHelp() [][]key.Binding {
	return [][]key.Binding{d.ShortHelp()}
}

// routerPresetItem is one selectable row in the RouterModel list.
type routerPresetItem struct {
	*list.Versioned
	t       *styles.Styles
	preset  router.Preset
	focused bool
}

func newRouterPresetItem(t *styles.Styles, p router.Preset) *routerPresetItem {
	return &routerPresetItem{Versioned: list.NewVersioned(), t: t, preset: p}
}

var _ list.Item = (*routerPresetItem)(nil)

// Finished implements list.Item.
func (p *routerPresetItem) Finished() bool {
	return true
}

// SetFocused implements list.Focusable.
func (p *routerPresetItem) SetFocused(focused bool) {
	if p.focused == focused {
		return
	}
	p.focused = focused
	p.Bump()
}

// Render implements list.Item.
func (p *routerPresetItem) Render(width int) string {
	info := p.preset.Model
	if p.preset.BaseURL != "" {
		info += " @ " + p.preset.BaseURL
	}
	itemStyles := ListItemStyles{
		ItemBlurred:     p.t.Dialog.NormalItem,
		ItemFocused:     p.t.Dialog.SelectedItem,
		InfoTextBlurred: p.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: p.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(itemStyles, p.preset.Label, info, p.focused, width, nil, nil)
}
