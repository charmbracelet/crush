package dialog

import (
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// ModelGroup represents a group of model items.
type ModelGroup struct {
	*list.Versioned
	Title      string
	Items      []*ModelItem
	configured bool
	t          *styles.Styles
}

// NewModelGroup creates a new ModelGroup.
func NewModelGroup(t *styles.Styles, title string, configured bool, items ...*ModelItem) ModelGroup {
	return ModelGroup{
		Versioned:  list.NewVersioned(),
		Title:      title,
		Items:      items,
		configured: configured,
		t:          t,
	}
}

// Finished implements list.Item. Model groups are immutable headers.
func (m *ModelGroup) Finished() bool {
	return true
}

// AppendItems appends [ModelItem]s to the group.
func (m *ModelGroup) AppendItems(items ...*ModelItem) {
	m.Items = append(m.Items, items...)
}

// Render implements [list.Item].
func (m *ModelGroup) Render(width int) string {
	var configured string
	if m.configured {
		configuredIcon := m.t.ToolCallSuccess.Render()
		configuredText := m.t.Dialog.Models.ConfiguredText.Render("Configured")
		configured = configuredIcon + " " + configuredText
	}

	title := " " + m.Title + " "
	// Keep the "Configured" badge only when the full title fits beside it
	// (plus a separator). Otherwise drop it and let the title use the whole
	// width, rather than truncating the title to reserve room for a badge
	// that common.Section would then drop anyway, leaving dead space.
	if configured != "" && lipgloss.Width(title)+lipgloss.Width(configured)+3 > width {
		configured = ""
	}
	if configured == "" {
		title = ansi.Truncate(title, max(0, width-1), "…")
	}

	return common.Section(m.t, title, width, configured)
}

// ModelItem represents a list item for a model type.
type ModelItem struct {
	*list.Versioned

	prov      catwalk.Provider
	model     catwalk.Model
	modelType ModelType

	cache        map[int]string
	t            *styles.Styles
	m            fuzzy.Match
	focused      bool
	showProvider bool

	// info overrides the right-hand column text when set. Multi-select
	// pickers (the router model pool) use it to show per-1M pricing,
	// which the regular switcher does not need.
	info string

	// checkable/checked support multi-select pickers (e.g. the router
	// model pool dialog) that reuse this item to stay visually and
	// behaviorally identical to the regular single-select model
	// switcher, just with a checkbox in front of the title.
	checkable bool
	checked   bool
}

// Finished implements list.Item. Model items are render-stable
// outside of explicit SetFocused / SetMatch.
func (m *ModelItem) Finished() bool {
	return true
}

// SelectedModel returns this model item as a [config.SelectedModel] instance.
func (m *ModelItem) SelectedModel() config.SelectedModel {
	return config.SelectedModel{
		Model:           m.model.ID,
		Provider:        string(m.prov.ID),
		ReasoningEffort: m.model.DefaultReasoningEffort,
		MaxTokens:       m.model.DefaultMaxTokens,
	}
}

// SelectedModelType returns the type of model represented by this item.
func (m *ModelItem) SelectedModelType() config.SelectedModelType {
	return m.modelType.Config()
}

var _ ListItem = &ModelItem{}

// NewModelItem creates a new ModelItem.
func NewModelItem(t *styles.Styles, prov catwalk.Provider, model catwalk.Model, typ ModelType, showProvider bool) *ModelItem {
	return &ModelItem{
		Versioned:    list.NewVersioned(),
		prov:         prov,
		model:        model,
		modelType:    typ,
		t:            t,
		cache:        make(map[int]string),
		showProvider: showProvider,
	}
}

// Filter implements ListItem.
func (m *ModelItem) Filter() string {
	return m.model.Name
}

// ID implements ListItem.
func (m *ModelItem) ID() string {
	return modelKey(string(m.prov.ID), m.model.ID)
}

// ModelID returns the raw catalog id of the underlying model (e.g. an
// OpenRouter model id), independent of the provider it's grouped under.
func (m *ModelItem) ModelID() string {
	return m.model.ID
}

// SetInfo overrides the right-hand column text, replacing the provider
// name that showProvider would otherwise render.
func (m *ModelItem) SetInfo(info string) {
	if m.info == info {
		return
	}
	m.info = info
	m.cache = nil
	if m.Versioned != nil {
		m.Bump()
	}
}

// SetCheckable turns this item into a checkbox row, used by multi-select
// pickers such as the router model pool dialog.
func (m *ModelItem) SetCheckable(checkable bool) {
	if m.checkable == checkable {
		return
	}
	m.checkable = checkable
	m.cache = nil
	if m.Versioned != nil {
		m.Bump()
	}
}

// Checked reports whether a checkable item is currently checked.
func (m *ModelItem) Checked() bool {
	return m.checked
}

// SetChecked sets a checkable item's checkbox state.
func (m *ModelItem) SetChecked(checked bool) {
	if m.checked == checked {
		return
	}
	m.checked = checked
	m.cache = nil
	if m.Versioned != nil {
		m.Bump()
	}
}

// Render implements ListItem.
func (m *ModelItem) Render(width int) string {
	info := m.info
	if info == "" && m.showProvider {
		info = string(m.prov.Name)
	}
	title := m.model.Name
	if m.checkable {
		box := "[ ]"
		if m.checked {
			box = "[x]"
		}
		title = box + " " + title
	}
	styles := ListItemStyles{
		ItemBlurred:     m.t.Dialog.NormalItem,
		ItemFocused:     m.t.Dialog.SelectedItem,
		InfoTextBlurred: m.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: m.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, title, info, m.focused, width, m.cache, &m.m)
}

// SetFocused implements ListItem.
func (m *ModelItem) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.cache = nil
	m.focused = focused
	if m.Versioned != nil {
		m.Bump()
	}
}

// SetMatch implements ListItem.
func (m *ModelItem) SetMatch(fm fuzzy.Match) {
	if sameFuzzyMatch(m.m, fm) {
		return
	}
	m.cache = nil
	m.m = fm
	if m.Versioned != nil {
		m.Bump()
	}
}
