package dialog

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// RouterEditorID is the identifier for the router editor dialog.
const RouterEditorID = "router_editor"

const (
	routerEditorMaxWidth = 80
	routerTargetPrefix   = "route:"
	routerTargetClass    = "classifier"
)

// Rows of the editor's main form, before the route rows.
const (
	rowName = iota
	rowClassifier
	rowInstructions
	rowRoutesStart
)

// Fields of the inline route form.
const (
	routeFieldName = iota
	routeFieldDescription
	routeFieldModel
	routeFieldCount
)

// RouterEditor creates or edits a model router.
type RouterEditor struct {
	com      *common.Common
	origName string
	existing []string

	name         textinput.Model
	instructions textinput.Model
	classifier   config.SelectedModel
	routes       []config.RouterRoute
	defaultRoute string
	timeout      int

	cursor     int
	editing    int
	routeField int
	routeName  textinput.Model
	routeDesc  textinput.Model

	err string

	help   help.Model
	keyMap struct {
		Up, Down, Next, Prev, Enter, Add, Delete, Default, Save, Close key.Binding
	}
}

var _ Dialog = (*RouterEditor)(nil)

// NewRouterEditor creates a router editor. An empty name creates a new
// router; existing lists the names already in use.
func NewRouterEditor(com *common.Common, name string, r config.RouterConfig, existing []string) *RouterEditor {
	e := &RouterEditor{
		com:          com,
		origName:     name,
		existing:     existing,
		classifier:   r.Classifier,
		routes:       slices.Clone(r.Routes),
		defaultRoute: r.Default,
		timeout:      r.Timeout,
		editing:      -1,
	}
	newInput := func(placeholder, value string) textinput.Model {
		in := textinput.New()
		in.SetStyles(com.Styles.TextInput)
		in.Prompt = ""
		in.Placeholder = placeholder
		in.SetValue(value)
		return in
	}
	e.name = newInput("e.g. auto", name)
	e.instructions = newInput("optional extra guidance for the classifier", r.Instructions)
	e.routeName = newInput("e.g. simple", "")
	e.routeDesc = newInput("when should this route be used?", "")

	e.help = help.New()
	e.help.Styles = com.Styles.DialogHelpStyles()
	e.keyMap.Up = key.NewBinding(key.WithKeys("up"), key.WithHelp("↑/↓", "move"))
	e.keyMap.Down = key.NewBinding(key.WithKeys("down"))
	e.keyMap.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next"))
	e.keyMap.Prev = key.NewBinding(key.WithKeys("shift+tab"))
	e.keyMap.Enter = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit/pick"))
	e.keyMap.Add = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add route"))
	e.keyMap.Delete = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete"))
	e.keyMap.Default = key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "default"))
	e.keyMap.Save = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save"))
	e.keyMap.Close = key.NewBinding(key.WithKeys("esc", "alt+esc"), key.WithHelp("esc", "back"))
	e.syncFocus()
	return e
}

// ID implements Dialog.
func (e *RouterEditor) ID() string {
	return RouterEditorID
}

// Err returns the current validation error, if any.
func (e *RouterEditor) Err() string {
	return e.err
}

func (e *RouterEditor) rowCount() int {
	return rowRoutesStart + len(e.routes) + 1
}

func (e *RouterEditor) routeIndex() (int, bool) {
	i := e.cursor - rowRoutesStart
	return i, i >= 0 && i < len(e.routes)
}

func (e *RouterEditor) isAddRow() bool {
	return e.cursor == rowRoutesStart+len(e.routes)
}

// focusedInput returns the text input that receives typed keys, if any.
func (e *RouterEditor) focusedInput() *textinput.Model {
	if e.editing >= 0 {
		switch e.routeField {
		case routeFieldName:
			return &e.routeName
		case routeFieldDescription:
			return &e.routeDesc
		}
		return nil
	}
	switch e.cursor {
	case rowName:
		return &e.name
	case rowInstructions:
		return &e.instructions
	}
	return nil
}

func (e *RouterEditor) syncFocus() {
	for _, in := range []*textinput.Model{&e.name, &e.instructions, &e.routeName, &e.routeDesc} {
		in.Blur()
	}
	if in := e.focusedInput(); in != nil {
		in.Focus()
	}
}

func (e *RouterEditor) move(delta int) {
	if e.editing >= 0 {
		e.routeField = (e.routeField + delta + routeFieldCount) % routeFieldCount
	} else {
		n := e.rowCount()
		e.cursor = (e.cursor + delta + n) % n
	}
	e.syncFocus()
}

func (e *RouterEditor) startEditing(i int) {
	e.editing = i
	e.routeField = routeFieldName
	e.routeName.SetValue(e.routes[i].Name)
	e.routeDesc.SetValue(e.routes[i].Description)
	e.syncFocus()
}

func (e *RouterEditor) commitRoute() {
	if e.editing < 0 || e.editing >= len(e.routes) {
		return
	}
	r := &e.routes[e.editing]
	newName := strings.TrimSpace(e.routeName.Value())
	if e.defaultRoute != "" && e.defaultRoute == r.Name {
		e.defaultRoute = newName
	}
	r.Name = newName
	r.Description = strings.TrimSpace(e.routeDesc.Value())
}

func (e *RouterEditor) stopEditing() {
	e.commitRoute()
	e.editing = -1
	e.syncFocus()
}

// SetPickedModel applies a model chosen in the picker to a target field.
func (e *RouterEditor) SetPickedModel(target string, sel config.SelectedModel) {
	if target == routerTargetClass {
		e.classifier = sel
		return
	}
	if s, ok := strings.CutPrefix(target, routerTargetPrefix); ok {
		if i, err := strconv.Atoi(s); err == nil && i >= 0 && i < len(e.routes) {
			e.routes[i].Model = sel
		}
	}
}

// Router returns the router as currently edited.
func (e *RouterEditor) Router() (string, config.RouterConfig) {
	e.commitRoute()
	return strings.TrimSpace(e.name.Value()), config.RouterConfig{
		Classifier:   e.classifier,
		Routes:       slices.Clone(e.routes),
		Default:      e.defaultRoute,
		Instructions: strings.TrimSpace(e.instructions.Value()),
		Timeout:      e.timeout,
	}
}

func (e *RouterEditor) validate() (string, config.RouterConfig, error) {
	name, r := e.Router()
	if name == "" {
		return "", r, fmt.Errorf("name is required")
	}
	if name != e.origName && slices.Contains(e.existing, name) {
		return "", r, fmt.Errorf("a router named %q already exists", name)
	}
	if err := config.ValidateRouter(name, r); err != nil {
		_, msg, _ := strings.Cut(err.Error(), ": ")
		return "", r, fmt.Errorf("%s", msg)
	}
	return name, r, nil
}

// HandleMsg implements Dialog.
func (e *RouterEditor) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if in := e.focusedInput(); in != nil {
			var cmd tea.Cmd
			*in, cmd = in.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:
		if key.Matches(msg, e.keyMap.Save) {
			name, r, err := e.validate()
			if err != nil {
				e.err = err.Error()
				return nil
			}
			e.err = ""
			return ActionSaveRouter{OrigName: e.origName, Name: name, Router: r}
		}
		if e.editing >= 0 {
			return e.handleRouteKey(msg)
		}
		return e.handleMainKey(msg)
	}
	return nil
}

func (e *RouterEditor) handleRouteKey(msg tea.KeyPressMsg) Action {
	switch {
	case key.Matches(msg, e.keyMap.Close):
		e.stopEditing()
	case key.Matches(msg, e.keyMap.Next, e.keyMap.Down):
		e.move(1)
	case key.Matches(msg, e.keyMap.Prev, e.keyMap.Up):
		e.move(-1)
	case key.Matches(msg, e.keyMap.Enter):
		if e.routeField == routeFieldModel {
			e.commitRoute()
			return ActionPickRouterModel{Target: routerTargetPrefix + strconv.Itoa(e.editing)}
		}
		e.move(1)
	default:
		if in := e.focusedInput(); in != nil {
			var cmd tea.Cmd
			*in, cmd = in.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

func (e *RouterEditor) handleMainKey(msg tea.KeyPressMsg) Action {
	onInput := e.focusedInput() != nil
	routeIdx, onRoute := e.routeIndex()
	switch {
	case key.Matches(msg, e.keyMap.Close):
		return ActionClose{}
	case key.Matches(msg, e.keyMap.Next, e.keyMap.Down):
		e.move(1)
	case key.Matches(msg, e.keyMap.Prev, e.keyMap.Up):
		e.move(-1)
	case key.Matches(msg, e.keyMap.Enter):
		switch {
		case e.cursor == rowClassifier:
			return ActionPickRouterModel{Target: routerTargetClass}
		case onRoute:
			e.startEditing(routeIdx)
		case e.isAddRow():
			e.addRoute()
		default:
			e.move(1)
		}
	case !onInput && key.Matches(msg, e.keyMap.Add):
		e.addRoute()
	case onRoute && key.Matches(msg, e.keyMap.Delete):
		if e.routes[routeIdx].Name == e.defaultRoute {
			e.defaultRoute = ""
		}
		e.routes = slices.Delete(e.routes, routeIdx, routeIdx+1)
	case onRoute && key.Matches(msg, e.keyMap.Default):
		e.defaultRoute = e.routes[routeIdx].Name
	case onInput:
		var cmd tea.Cmd
		in := e.focusedInput()
		*in, cmd = in.Update(msg)
		return ActionCmd{Cmd: cmd}
	}
	return nil
}

func (e *RouterEditor) addRoute() {
	e.routes = append(e.routes, config.RouterRoute{})
	e.cursor = rowRoutesStart + len(e.routes) - 1
	e.startEditing(len(e.routes) - 1)
}

func (e *RouterEditor) modelLabel(sel config.SelectedModel) string {
	if sel.Provider == "" || sel.Model == "" {
		return "(choose a model)"
	}
	if cfg := e.com.Config(); cfg != nil {
		if m := cfg.GetModel(sel.Provider, sel.Model); m != nil && m.Name != "" {
			return m.Name + " · " + sel.Provider
		}
	}
	return sel.Provider + "/" + sel.Model
}

// Draw implements Dialog.
func (e *RouterEditor) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := e.com.Styles
	width := max(0, min(routerEditorMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := max(0, width-t.Dialog.View.GetHorizontalFrameSize())
	rowWidth := max(0, innerWidth-2)
	const labelWidth = 14
	fieldWidth := max(1, rowWidth-labelWidth)

	for _, in := range []*textinput.Model{&e.name, &e.instructions, &e.routeName, &e.routeDesc} {
		in.SetWidth(max(1, fieldWidth-1))
	}

	pad := lipgloss.NewStyle().Padding(0, 1)
	field := func(selected bool, label, value string) string {
		labelStyle := t.Dialog.Arguments.InputLabelBlurred
		if selected {
			labelStyle = t.Dialog.Arguments.InputLabelFocused
		}
		l := labelStyle.Width(labelWidth).Render(ansi.Truncate(label, labelWidth-1, "…"))
		return pad.Render(l + ansi.Truncate(value, fieldWidth, "…"))
	}
	button := func(selected bool, label, value string) string {
		style := t.Dialog.NormalItem.UnsetPadding()
		if selected {
			style = t.Dialog.SelectedItem.UnsetPadding()
		}
		return field(selected, label, style.Render(ansi.Truncate(value, fieldWidth, "…")))
	}

	var rows []string
	if e.editing >= 0 {
		rows = append(rows,
			t.Dialog.SecondaryText.Render(fmt.Sprintf("Route %d", e.editing+1)),
			"",
			field(e.routeField == routeFieldName, "Name", e.routeName.View()),
			field(e.routeField == routeFieldDescription, "Description", e.routeDesc.View()),
			button(e.routeField == routeFieldModel, "Model", e.modelLabel(e.routes[e.editing].Model)),
		)
	} else {
		rows = append(rows,
			"",
			field(e.cursor == rowName, "Name", e.name.View()),
			button(e.cursor == rowClassifier, "Classifier", e.modelLabel(e.classifier)),
			field(e.cursor == rowInstructions, "Instructions", e.instructions.View()),
			"",
			t.Dialog.SecondaryText.Render("Routes"),
		)
		defaultName := e.defaultRoute
		if defaultName == "" && len(e.routes) > 0 {
			defaultName = e.routes[0].Name
		}
		for i, r := range e.routes {
			mark := "  "
			if r.Name == defaultName {
				mark = "* "
			}
			name := r.Name
			if name == "" {
				name = "(unnamed)"
			}
			text := mark + name + " · " + e.modelLabel(r.Model)
			if r.Description != "" {
				text += " · " + r.Description
			}
			rows = append(rows, e.listRow(e.cursor == rowRoutesStart+i, text, rowWidth))
		}
		rows = append(rows, e.listRow(e.isAddRow(), "+ Add route", rowWidth))
	}

	if e.err != "" {
		rows = append(rows, "", pad.Render(t.Dialog.OAuth.ErrorText.Render(ansi.Truncate(e.err, rowWidth, "…"))))
	}
	rows = append(rows, "")

	title := "New Router"
	if e.origName != "" {
		title = "Edit Router"
	}
	rc := NewRenderContext(t, width)
	rc.Title = title
	rc.AddPart(lipgloss.JoinVertical(lipgloss.Left, rows...))
	rc.Help = renderDialogHelp(t, &e.help, e, innerWidth)
	DrawCenter(scr, area, rc.Render())
	return nil
}

func (e *RouterEditor) listRow(selected bool, text string, width int) string {
	t := e.com.Styles
	text = ansi.Truncate(text, width, "…")
	if selected {
		return t.Dialog.SelectedItem.Width(width + 2).Render(text)
	}
	return t.Dialog.NormalItem.Width(width + 2).Render(text)
}

// ShortHelp implements help.KeyMap.
func (e *RouterEditor) ShortHelp() []key.Binding {
	h := []key.Binding{e.keyMap.Up, e.keyMap.Enter}
	if _, onRoute := e.routeIndex(); e.editing < 0 && onRoute {
		h = append(h, e.keyMap.Delete, e.keyMap.Default)
	}
	if e.editing < 0 && e.focusedInput() == nil {
		h = append(h, e.keyMap.Add)
	}
	return append(h, e.keyMap.Save, e.keyMap.Close)
}

// FullHelp implements help.KeyMap.
func (e *RouterEditor) FullHelp() [][]key.Binding {
	return [][]key.Binding{e.ShortHelp()}
}
