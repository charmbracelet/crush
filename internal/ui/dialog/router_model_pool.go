// internal/ui/dialog/router_model_pool.go
package dialog

import (
	"fmt"
	"log/slog"
	"sort"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// RouterModelPoolID is the identifier for the router model-pool picker
// dialog, opened from the Model Pool row in Router Settings.
const RouterModelPoolID = "router-model-pool"

const routerModelDialogMaxWidth = defaultModelsDialogMaxWidth

// RouterModelPool lets the user build the router's model_pool by checking
// models off the provider catalog (every provider, not just OpenRouter),
// instead of typing comma-separated ids by hand into the Router Settings
// "Model Pool" row. It reuses the same ModelsList/ModelGroup/ModelItem
// building blocks as the regular model switcher (see models.go), just with
// checkboxes instead of single-select, so the two dialogs look and behave
// the same way. Toggling a checkbox persists immediately — the same
// instant-persist convention every other Router Settings field already
// uses — so there is no separate save step; closing the dialog just
// returns to Router Settings.
type RouterModelPool struct {
	com   *common.Common
	list  *ModelsList
	input textinput.Model
	help  help.Model
	items []*ModelItem // all catalog models, unfiltered, stable order

	keyMap struct {
		Toggle   key.Binding
		UpDown   key.Binding
		Next     key.Binding
		Previous key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*RouterModelPool)(nil)

// NewRouterModelPool builds the dialog from the provider catalog and the
// router's currently configured pool. The pool may name a chat model from
// any configured or catalog provider, grouped exactly like the regular
// model switcher, so a non-OpenRouter model is pickable too.
func NewRouterModelPool(com *common.Common) *RouterModelPool {
	providers, err := config.Providers(com.Config())
	if err != nil && len(providers) == 0 {
		slog.Warn("Listing the router model pool without a provider catalog", "error", err)
	}
	return newRouterModelPool(com, providers)
}

// newRouterModelPool is the catalog-injected constructor, split out so
// tests can supply a fixed provider list without network access.
func newRouterModelPool(com *common.Common, providers []catwalk.Provider) *RouterModelPool {
	cfg := com.Config()
	pool := make(map[string]bool)
	if cfg.Options.Router != nil {
		for _, id := range cfg.Options.Router.ModelPool {
			pool[id] = true
		}
	}

	t := com.Styles
	groups, _, _ := providerModelGroups(t, cfg, providers, ModelTypeLarge, config.SelectedModel{})

	var items []*ModelItem
	for i := range groups {
		for _, item := range groups[i].Items {
			item.SetCheckable(true)
			item.SetChecked(pool[item.ModelID()])
			item.SetInfo(routerModelPrice(item.model))
			items = append(items, item)
		}
	}

	l := NewModelsList(t)
	l.SetGroups(groups...)
	l.Focus()
	l.SelectFirst()

	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Placeholder = "Filter chat models"
	input.SetStyles(t.TextInput)
	input.Focus()

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	d := &RouterModelPool{
		com:   com,
		list:  l,
		input: input,
		help:  h,
		items: items,
	}
	d.keyMap.Toggle = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "toggle"))
	d.keyMap.UpDown = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose"))
	d.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next"))
	d.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous"))
	d.keyMap.Close = CloseKey

	return d
}

// routerModelPrice renders a catalog model's per-1M input/output pricing
// for the pool row's right-hand column, e.g. "$1.10/$5.50 per 1M".
func routerModelPrice(model catwalk.Model) string {
	return fmt.Sprintf("$%.2f/$%.2f per 1M", model.CostPer1MIn, model.CostPer1MOut)
}

// ID implements Dialog.
func (d *RouterModelPool) ID() string {
	return RouterModelPoolID
}

// dialogTitle names this picker. It stays deliberately distinct from the
// decision-model picker's title so the two are never confused.
func (d *RouterModelPool) dialogTitle() string {
	return "Model Pool"
}

// purposeLine explains, in the dialog itself, that this picker is not the
// one that chooses the router's classifier model.
func (d *RouterModelPool) purposeLine() string {
	return "Chat models the router may switch to. Not the decision model."
}

func (d *RouterModelPool) selected() *ModelItem {
	item := d.list.SelectedItem()
	if item == nil {
		return nil
	}
	mi, _ := item.(*ModelItem)
	return mi
}

// HandleMsg implements Dialog.
func (d *RouterModelPool) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, d.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, d.keyMap.Previous):
		if d.list.IsSelectedFirst() {
			d.list.SelectLast()
		} else {
			d.list.SelectPrev()
		}
		d.list.ScrollToSelected()
	case key.Matches(keyMsg, d.keyMap.Next):
		if d.list.IsSelectedLast() {
			d.list.SelectFirst()
		} else {
			d.list.SelectNext()
		}
		d.list.ScrollToSelected()
	case key.Matches(keyMsg, d.keyMap.Toggle):
		if item := d.selected(); item != nil {
			item.SetChecked(!item.Checked())
			return ActionCmd{d.persistPool()}
		}
	default:
		prevValue := d.input.Value()
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(keyMsg)
		if d.input.Value() != prevValue {
			d.list.SetFilter(d.input.Value())
			d.list.SelectFirst()
			d.list.ScrollToTop()
		}
		return ActionCmd{cmd}
	}
	return nil
}

// persistPool writes the currently checked models back to
// options.router.model_pool, sorted for a stable, diffable config file.
func (d *RouterModelPool) persistPool() tea.Cmd {
	var pool []string
	for _, it := range d.items {
		if it.Checked() {
			pool = append(pool, it.ModelID())
		}
	}
	sort.Strings(pool)
	if err := d.com.Workspace.SetConfigField(config.ScopeGlobal, "options.router.model_pool", pool); err != nil {
		return util.ReportError(err)
	}
	return nil
}

// Cursor implements Dialog.
func (d *RouterModelPool) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *RouterModelPool) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(routerModelDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	d.input.SetWidth(dialogInputTextWidth(t, d.input, innerWidth))

	// The purpose line below the title is one row the list budget does
	// not know about, so reserve it here rather than overflowing the
	// dialog.
	listHeight, listTotalHeight, _ := sizeDialogList(t, d.list, innerWidth, max(0, height-1))

	rc := NewRenderContext(t, width)
	rc.Title = d.dialogTitle()
	rc.AddPart(t.Dialog.ListItem.InfoBlurred.Render(d.purposeLine()))

	inputView := t.Dialog.InputPrompt.Render(d.input.View())
	rc.AddPart(inputView)

	listView := t.Dialog.List.Height(d.list.Height()).Render(d.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, d.list.Offset())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)

	// The purpose line rendered above the input shifts it down a row,
	// which InputCursor does not know about.
	cur := offsetInputCursor(d.Cursor(), 1)
	view := rc.Render()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *RouterModelPool) ShortHelp() []key.Binding {
	return []key.Binding{d.keyMap.UpDown, d.keyMap.Toggle, d.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (d *RouterModelPool) FullHelp() [][]key.Binding {
	return [][]key.Binding{d.ShortHelp()}
}
