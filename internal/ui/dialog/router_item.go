package dialog

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// routerFieldKind describes how a router settings row is edited: a
// boolean toggles in place, an enum cycles in place, and the rest open
// inline text editing.
type routerFieldKind int

const (
	routerFieldBool routerFieldKind = iota
	routerFieldEnum
	routerFieldText
	routerFieldFloat
	routerFieldInt
	routerFieldList
	// routerFieldModel opens the per-provider decision-model picker.
	routerFieldModel
)

// routerFieldSpec describes one editable router setting: its label, the
// key it maps to under "options.router.", and how it is edited.
type routerFieldSpec struct {
	key      string
	label    string
	kind     routerFieldKind
	enumOpts []string
}

// routerFieldSpecs lists every router setting shown in the dialog, in
// display order. There is no agent-selection field — see the spec's
// "TUI" section for why (agent switching was cut from the shipped
// router; it only ever chooses reasoning effort for the active agent).
var routerFieldSpecs = []routerFieldSpec{
	{key: "enabled", label: "Enabled", kind: routerFieldBool},
	{key: "provider", label: "Provider", kind: routerFieldEnum, enumOpts: router.ProviderNames()},
	{key: "base_url", label: "Base URL", kind: routerFieldText},
	{key: "api_key", label: "API Key", kind: routerFieldText},
	{key: "model", label: "Decision Model", kind: routerFieldModel},
	{key: "confidence_threshold", label: "Confidence Threshold", kind: routerFieldFloat},
	{key: "timeout_ms", label: "Timeout (ms)", kind: routerFieldInt},
	{key: "model_pool", label: "Model Pool", kind: routerFieldList},
	{key: "min_model_confidence", label: "Min Model Confidence", kind: routerFieldFloat},
	{key: "apply_subagents", label: "Apply to Sub-agents", kind: routerFieldBool},
}

// RouterItem is one editable row in the router settings dialog.
type RouterItem struct {
	*list.Versioned
	spec    routerFieldSpec
	value   string
	editing bool
	input   textinput.Model
	focused bool
	t       *styles.Styles
	cache   map[int]string
}

var _ list.Item = (*RouterItem)(nil)

// NewRouterItem creates a row for spec, seeded with its current value
// (already formatted as a string — the dialog is responsible for
// converting to/from the config's typed fields).
func NewRouterItem(t *styles.Styles, spec routerFieldSpec, value string) *RouterItem {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.SetStyles(t.TextInput)
	return &RouterItem{
		Versioned: list.NewVersioned(),
		spec:      spec,
		value:     value,
		input:     input,
		t:         t,
	}
}

// Finished implements list.Item. Router items are render-stable outside
// of explicit SetFocused / StartEdit / HandleInput / SetValue calls, all
// of which bump the version.
func (i *RouterItem) Finished() bool {
	return true
}

// SetFocused implements list.Focusable. Losing focus mid-edit cancels
// the edit (same as an explicit CancelEdit call) rather than leaving
// the input internally focused and editing true while the row renders
// as an ordinary blurred row — the caller may return focus later
// without an intervening StartEdit, and the row must not resume
// showing stale edit-mode state.
func (i *RouterItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.focused = focused
	if !focused && i.editing {
		i.CancelEdit()
		return
	}
	i.cache = nil
	i.Bump()
}

// Value returns the item's current, committed value.
func (i *RouterItem) Value() string {
	return i.value
}

// SetValue commits a new value and exits edit mode.
func (i *RouterItem) SetValue(v string) {
	i.cache = nil
	i.value = v
	i.editing = false
	i.input.Blur()
	i.Bump()
}

// StartEdit enters inline-edit mode, seeding the input with the current
// value. Meaningful only for text/float/int fields — the dialog toggles
// bool fields and cycles enum fields directly without ever calling this.
func (i *RouterItem) StartEdit() {
	i.input.SetValue(i.value)
	i.input.CursorEnd()
	i.input.Focus()
	i.editing = true
	i.cache = nil
	i.Bump()
}

// CancelEdit exits edit mode without changing the committed value.
func (i *RouterItem) CancelEdit() {
	i.editing = false
	i.input.Blur()
	i.cache = nil
	i.Bump()
}

// Editing reports whether the item is in inline-edit mode.
func (i *RouterItem) Editing() bool {
	return i.editing
}

// HandleInput forwards a message to the item's text input while editing.
func (i *RouterItem) HandleInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i.input, cmd = i.input.Update(msg)
	i.cache = nil
	i.Bump()
	return cmd
}

// InputValue returns the current, uncommitted text in the edit input.
func (i *RouterItem) InputValue() string {
	return i.input.Value()
}

// Cursor returns the edit input's cursor.
func (i *RouterItem) Cursor() *tea.Cursor {
	return i.input.Cursor()
}

// Render implements list.Item.
func (i *RouterItem) Render(width int) string {
	if i.editing && i.focused {
		style := i.t.Dialog.SelectedItem
		const cursorPadding = 1
		inputWidth := max(0, width-style.GetHorizontalFrameSize()-cursorPadding)
		i.input.SetWidth(inputWidth)
		return style.Render(i.spec.label + ": " + i.input.View())
	}

	itemStyles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	displayValue := i.value
	if i.spec.key == "api_key" {
		displayValue = maskSecret(displayValue)
	}
	return renderItem(itemStyles, i.spec.label, displayValue, i.focused, width, i.cache, nil)
}

// maskSecret returns s with all but its last 4 characters replaced by a
// bullet, so a value like an API key isn't rendered in plaintext while
// the dialog is simply open (not being edited). Short values are masked
// entirely rather than partially revealed.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	const revealLast = 4
	if len(s) <= revealLast {
		return strings.Repeat("•", len(s))
	}
	return strings.Repeat("•", len(s)-revealLast) + s[len(s)-revealLast:]
}
