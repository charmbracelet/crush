// internal/ui/dialog/router.go
package dialog

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// RouterID is the identifier for the router settings dialog.
const RouterID = "router"

const routerDialogMaxWidth = 60

// Router is a settings dialog for the pre-call model/reasoning router. It
// has no agent picker: the shipped router only ever chooses a reasoning
// effort override for the agent already active (see the spec's "TUI"
// section for why agent-selection was cut).
type Router struct {
	com  *common.Common
	list *list.List
	help help.Model
	// items are direct handles to every row so Draw can refresh their
	// displayed values from config on every frame: the model and model
	// pool pickers (stacked on top) persist straight to config, and a
	// provider change resets base_url and model.
	items []*RouterItem

	keyMap struct {
		Toggle   key.Binding
		Confirm  key.Binding
		Cancel   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*Router)(nil)

// NewRouter builds the dialog from the current config. A nil
// Options.Router is treated as an all-defaults, unconfigured router.
func NewRouter(com *common.Common) *Router {
	values := routerValues(com.Config().Options.Router)

	items := make([]list.Item, 0, len(routerFieldSpecs))
	rows := make([]*RouterItem, 0, len(routerFieldSpecs))
	for _, spec := range routerFieldSpecs {
		item := NewRouterItem(com.Styles, spec, values[spec.key])
		rows = append(rows, item)
		items = append(items, item)
	}

	l := list.NewList(items...)
	l.RegisterRenderCallback(list.FocusedRenderCallback(l))
	l.Focus()
	l.SetSelected(0)

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()

	r := &Router{com: com, list: l, help: h, items: rows}
	r.keyMap.Toggle = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "toggle/edit"))
	r.keyMap.Confirm = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm"))
	r.keyMap.Cancel = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel edit"))
	r.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓", "next"))
	r.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑", "previous"))
	r.keyMap.UpDown = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose"))
	r.keyMap.Close = CloseKey
	return r
}

// routerValues renders every router setting as the string its row
// displays. A nil ro is treated as an all-defaults, unconfigured router.
func routerValues(ro *config.RouterOptions) map[string]string {
	return map[string]string{
		"enabled":              strconv.FormatBool(ro != nil && ro.Enabled),
		"provider":             routerStringOrDefault(ro, func(o *config.RouterOptions) string { return o.Provider }, "openrouter"),
		"base_url":             routerStringOrDefault(ro, func(o *config.RouterOptions) string { return o.BaseURL }, ""),
		"api_key":              routerStringOrDefault(ro, func(o *config.RouterOptions) string { return o.APIKey }, ""),
		"model":                routerStringOrDefault(ro, func(o *config.RouterOptions) string { return o.Model }, ""),
		"confidence_threshold": strconv.FormatFloat(ro.EffectiveConfidenceThreshold(), 'g', -1, 64),
		"timeout_ms":           strconv.FormatInt(ro.EffectiveTimeout().Milliseconds(), 10),
		"model_pool":           strings.Join(routerStringSliceOrDefault(ro), ", "),
		"min_model_confidence": strconv.FormatFloat(ro.EffectiveMinModelConfidence(len(routerStringSliceOrDefault(ro))), 'g', -1, 64),
		"apply_subagents":      strconv.FormatBool(ro != nil && ro.ApplySubagents),
	}
}

// routerStringOrDefault reads a string field off ro via get, falling back
// to def when ro is nil or the field is empty.
func routerStringOrDefault(ro *config.RouterOptions, get func(*config.RouterOptions) string, def string) string {
	if ro == nil {
		return def
	}
	if v := get(ro); v != "" {
		return v
	}
	return def
}

// routerStringSliceOrDefault reads ro.ModelPool, or an empty slice when
// ro is nil.
func routerStringSliceOrDefault(ro *config.RouterOptions) []string {
	if ro == nil {
		return nil
	}
	return ro.ModelPool
}

// ID implements Dialog.
func (r *Router) ID() string {
	return RouterID
}

func (r *Router) selectedItem() *RouterItem {
	item := r.list.SelectedItem()
	if item == nil {
		return nil
	}
	ri, _ := item.(*RouterItem)
	return ri
}

// HandleMsg implements Dialog.
func (r *Router) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	item := r.selectedItem()
	if item != nil && item.Editing() {
		switch {
		case key.Matches(keyMsg, r.keyMap.Confirm):
			return r.commitEdit(item)
		case key.Matches(keyMsg, r.keyMap.Cancel):
			item.CancelEdit()
			return nil
		default:
			return ActionCmd{item.HandleInput(keyMsg)}
		}
	}

	switch {
	case key.Matches(keyMsg, r.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, r.keyMap.Previous):
		if r.list.IsSelectedFirst() {
			r.list.SelectLast()
		} else {
			r.list.SelectPrev()
		}
		r.list.ScrollToSelected()
	case key.Matches(keyMsg, r.keyMap.Next):
		if r.list.IsSelectedLast() {
			r.list.SelectFirst()
		} else {
			r.list.SelectNext()
		}
		r.list.ScrollToSelected()
	case key.Matches(keyMsg, r.keyMap.Toggle):
		if item != nil {
			return r.activateField(item)
		}
	}
	return nil
}

// activateField handles Enter on a non-editing row: booleans toggle and
// enums cycle in place and persist immediately; everything else enters
// inline-edit mode (persisted only on a later Confirm).
func (r *Router) activateField(item *RouterItem) Action {
	switch item.spec.kind {
	case routerFieldBool:
		current, _ := strconv.ParseBool(item.Value())
		return r.persist(item, strconv.FormatBool(!current))
	case routerFieldEnum:
		return r.persist(item, nextEnumValue(item.spec.enumOpts, item.Value()))
	case routerFieldList:
		return ActionOpenDialog{RouterModelPoolID}
	case routerFieldModel:
		return ActionOpenDialog{RouterModelID}
	default:
		item.StartEdit()
		return nil
	}
}

// commitEdit validates and persists a text/float/int field's edit buffer.
// Invalid input is rejected with a warning and the on-disk value is left
// untouched, matching the same validation the "option router" crushrc
// builtin already enforces.
func (r *Router) commitEdit(item *RouterItem) Action {
	val := item.InputValue()
	switch item.spec.kind {
	case routerFieldFloat:
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil || parsed < 0 || parsed > 1 {
			return ActionCmd{util.ReportWarn(item.spec.label + " must be a number between 0 and 1")}
		}
	case routerFieldInt:
		parsed, err := strconv.Atoi(val)
		if err != nil || parsed <= 0 {
			return ActionCmd{util.ReportWarn("Timeout must be a positive integer")}
		}
	}
	return r.persist(item, val)
}

// persist writes newValue to config, converting to the typed form
// RouterOptions expects, and only updates item's displayed value once the
// write actually succeeds — a failed write must never leave the row
// showing a value that was never saved.
func (r *Router) persist(item *RouterItem, newValue string) Action {
	configKey := "options.router." + item.spec.key
	var value any = newValue
	switch item.spec.kind {
	case routerFieldBool:
		value, _ = strconv.ParseBool(newValue)
	case routerFieldFloat:
		value, _ = strconv.ParseFloat(newValue, 64)
	case routerFieldInt:
		value, _ = strconv.Atoi(newValue)
	case routerFieldList:
		value = splitModelPool(newValue)
	}
	if err := r.com.Workspace.SetConfigField(config.ScopeGlobal, configKey, value); err != nil {
		return ActionCmd{util.ReportError(err)}
	}
	item.SetValue(newValue)
	if item.spec.key == "provider" {
		// A base URL or model from the previous backend would be sent to
		// the new one; clear both so the new backend's defaults apply.
		for _, k := range []string{"base_url", "model"} {
			if err := r.com.Workspace.SetConfigField(config.ScopeGlobal, "options.router."+k, ""); err != nil {
				return ActionCmd{util.ReportError(err)}
			}
		}
	}
	return nil
}

// splitModelPool parses the model_pool row's comma-separated edit buffer
// into a clean list: trimmed, with empty entries (from a trailing comma,
// blank input, or repeated commas) dropped.
func splitModelPool(raw string) []string {
	parts := strings.Split(raw, ",")
	pool := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			pool = append(pool, p)
		}
	}
	return pool
}

func nextEnumValue(opts []string, current string) string {
	for i, v := range opts {
		if v == current {
			return opts[(i+1)%len(opts)]
		}
	}
	if len(opts) > 0 {
		return opts[0]
	}
	return current
}

// Cursor returns the cursor for the dialog: only present while a row is
// in inline-edit mode. Router renders its editing row with
// Dialog.SelectedItem (a "label: " prefix followed by the input) inside
// a plain title+list dialog, not the InputPrompt-boxed search field
// InputCursor assumes and not always the first list row — so this
// computes the on-screen position directly instead of reusing that
// helper, walking the same visibleStart..selected loop
// renameCursorOffset uses for the analogous inline-rename-in-a-list case
// elsewhere.
func (r *Router) Cursor() *tea.Cursor {
	item := r.selectedItem()
	if item == nil || !item.Editing() {
		return nil
	}
	cur := item.Cursor()
	if cur == nil {
		return nil
	}

	t := r.com.Styles
	itemStyle := t.Dialog.SelectedItem
	dialogStyle := t.Dialog.View
	titleStyle := t.Dialog.Title

	cur.X += lipgloss.Width(item.spec.label+": ") +
		itemStyle.GetBorderLeftSize() + itemStyle.GetPaddingLeft() + itemStyle.GetMarginLeft() +
		dialogStyle.GetBorderLeftSize() + dialogStyle.GetPaddingLeft() + dialogStyle.GetMarginLeft()

	// +1 for the title's own rendered line: Router always renders a
	// non-empty title with no gap before the list, so the list's first
	// row starts exactly one line below the title.
	cur.Y += titleStyle.GetVerticalFrameSize() + 1 +
		dialogStyle.GetBorderTopSize() + dialogStyle.GetPaddingTop() + dialogStyle.GetMarginTop()

	visibleStart, visibleEnd := r.list.VisibleItemIndices()
	selected := r.list.Selected()
	for i := visibleStart; i <= visibleEnd && i != selected && selected > -1; i++ {
		cur.Y++
	}
	return cur
}

// refreshFromConfig re-reads the router config and updates every row
// that isn't being edited. Draw calls this every frame so values written
// by the stacked pickers, or reset by a provider change, show up here.
func (r *Router) refreshFromConfig() {
	values := routerValues(r.com.Config().Options.Router)
	for _, item := range r.items {
		if item.Editing() {
			continue
		}
		if v := values[item.spec.key]; v != item.Value() {
			item.SetValue(v)
		}
	}
}

// Draw implements Dialog.
func (r *Router) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	r.refreshFromConfig()
	t := r.com.Styles
	width := max(0, min(routerDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	listHeight, listTotalHeight, _ := sizeDialogList(t, r.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = "Router Settings (Experimental)"

	listView := t.Dialog.List.Height(r.list.Height()).Render(r.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, r.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &r.help, r, innerWidth)

	cur := r.Cursor()
	view := rc.Render()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (r *Router) ShortHelp() []key.Binding {
	item := r.selectedItem()
	if item != nil && item.Editing() {
		return []key.Binding{r.keyMap.Confirm, r.keyMap.Cancel}
	}
	return []key.Binding{r.keyMap.UpDown, r.keyMap.Toggle, r.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (r *Router) FullHelp() [][]key.Binding {
	return [][]key.Binding{r.ShortHelp()}
}
