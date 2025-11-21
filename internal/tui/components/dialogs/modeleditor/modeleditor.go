package modeleditor

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/confirm"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	fieldID = iota
	fieldName
	fieldContextWindow
	fieldDefaultMaxTokens
	fieldCostPer1MIn
	fieldCostPer1MOut
	fieldCostPer1MInCached
	fieldCostPer1MOutCached
	fieldCanReason
	fieldReasoningLevels
	fieldDefaultReasoningEffort
	fieldSupportsImages
	fieldCount
)

type ModelEditorSaveMsg struct {
	Model      catwalk.Model
	ProviderID string
}

type ModelEditorCancelMsg struct{}

type ModelEditor struct {
	wWidth  int
	wHeight int
	width   int

	model      catwalk.Model
	providerID string

	viewport viewport.Model
	keyMap   KeyMap
	help     help.Model

	// Model editing
	inputs []textinput.Model

	focusedField int

	// Validation
	errors map[int]string

	// State
	isDirty       bool
	showConfirm   bool
	confirmDialog *confirm.Model
}

type editorSection struct {
	title  string
	fields []struct {
		label string
		index int
		key   string // validation key, if different from index logic (not needed if we use index)
	}
}

var sections = []editorSection{
	{
		title: "Basic Info",
		fields: []struct {
			label string
			index int
			key   string
		}{
			{"ID", fieldID, "id"},
			{"Name", fieldName, "name"},
		},
	},
	{
		title: "Context & Tokens",
		fields: []struct {
			label string
			index int
			key   string
		}{
			{"Context Window", fieldContextWindow, ""},
			{"Max Tokens", fieldDefaultMaxTokens, ""},
		},
	},
	{
		title: "Cost",
		fields: []struct {
			label string
			index int
			key   string
		}{
			{"Cost/1M In", fieldCostPer1MIn, ""},
			{"Cost/1M Out", fieldCostPer1MOut, ""},
			{"Cost/1M In (Cached)", fieldCostPer1MInCached, ""},
			{"Cost/1M Out (Cached)", fieldCostPer1MOutCached, ""},
		},
	},
	{
		title: "Capabilities",
		fields: []struct {
			label string
			index int
			key   string
		}{
			{"Can Reason", fieldCanReason, ""},
			{"Reasoning Levels", fieldReasoningLevels, ""},
			{"Default Effort", fieldDefaultReasoningEffort, ""},
			{"Supports Images", fieldSupportsImages, ""},
		},
	},
}

func newInputs(model catwalk.Model, t *styles.Theme) []textinput.Model {
	inputs := make([]textinput.Model, fieldCount)

	createInput := func(value string, placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.SetValue(value)
		ti.SetStyles(t.S().TextInput)
		return ti
	}

	inputs[fieldID] = createInput(model.ID, "Model ID")
	inputs[fieldName] = createInput(model.Name, "Model Name")
	inputs[fieldContextWindow] = createInput(fmt.Sprintf("%d", model.ContextWindow), "128000")
	inputs[fieldDefaultMaxTokens] = createInput(fmt.Sprintf("%d", model.DefaultMaxTokens), "4096")
	inputs[fieldCostPer1MIn] = createInput(fmt.Sprintf("%.2f", model.CostPer1MIn), "0.00")
	inputs[fieldCostPer1MOut] = createInput(fmt.Sprintf("%.2f", model.CostPer1MOut), "0.00")
	inputs[fieldCostPer1MInCached] = createInput(fmt.Sprintf("%.2f", model.CostPer1MInCached), "0.00")
	inputs[fieldCostPer1MOutCached] = createInput(fmt.Sprintf("%.2f", model.CostPer1MOutCached), "0.00")
	inputs[fieldCanReason] = createInput(fmt.Sprintf("%t", model.CanReason), "false")
	inputs[fieldReasoningLevels] = createInput(strings.Join(model.ReasoningLevels, ","), "low,medium,high")
	inputs[fieldDefaultReasoningEffort] = createInput(model.DefaultReasoningEffort, "medium")
	inputs[fieldSupportsImages] = createInput(fmt.Sprintf("%t", model.SupportsImages), "false")

	return inputs
}

func NewModelEditor(model catwalk.Model, providerID string) *ModelEditor {
	t := styles.CurrentTheme()

	// Create viewport
	vp := viewport.New()

	// Initialize inputs
	inputs := newInputs(model, t)

	// Focus first field
	inputs[fieldID].Focus()

	h := help.New()
	h.Styles = t.S().Help

	return &ModelEditor{
		model:      model,
		providerID: providerID,
		viewport:   vp,
		keyMap:     DefaultKeyMap(),
		help:       h,
		inputs:     inputs,
		width:      80,
		errors:     make(map[int]string),
		isDirty:    false,
	}
}

func (e *ModelEditor) Init() tea.Cmd {
	if e.wWidth > 0 && e.wHeight > 0 {
		e.updateViewport()
	}
	return e.viewport.Init()
}

func (e *ModelEditor) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	if e.showConfirm {
		return e.handleConfirmDialog(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.wWidth = msg.Width
		e.wHeight = msg.Height
		e.help.SetWidth(msg.Width - 4)
		e.updateViewport()

	case tea.KeyPressMsg:
		return e.handleKeyPress(msg)
	}

	return e, nil
}

func (e *ModelEditor) SetSize(w, h int) {
	e.wWidth = w
	e.wHeight = h
	e.help.SetWidth(w - 4)
	e.updateViewport()
}

func (e *ModelEditor) handleConfirmDialog(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case confirm.ResultMsg:
		e.showConfirm = false
		if msg.Confirmed {
			if strings.HasPrefix(msg.Context, "discard") {
				return e, util.CmdHandler(ModelEditorCancelMsg{})
			}
		}
		e.confirmDialog = nil
		return e, nil

	default:
		var cmd tea.Cmd
		var m util.Model
		m, cmd = e.confirmDialog.Update(msg)
		e.confirmDialog = m.(*confirm.Model)
		return e, cmd
	}
}

func (e *ModelEditor) handleKeyPress(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	// Check for Save
	if key.Matches(msg, e.keyMap.Save) {
		return e.save()
	}

	// Check for Close/Esc
	if key.Matches(msg, e.keyMap.Close) {
		if e.isDirty {
			e.showConfirm = true
			e.confirmDialog = confirm.New(
				"Unsaved Changes",
				"You have unsaved changes. Discard them?",
				"discard-changes",
			)
			var m util.Model
			m, _ = e.confirmDialog.Update(tea.WindowSizeMsg{
				Width:  e.wWidth,
				Height: e.wHeight,
			})
			e.confirmDialog = m.(*confirm.Model)
			return e, nil
		}
		return e, util.CmdHandler(ModelEditorCancelMsg{})
	}

	// Check for navigation
	if key.Matches(msg, e.keyMap.Previous) {
		e.prevField()
		return e, nil
	}

	if key.Matches(msg, e.keyMap.Next) {
		e.nextField()
		return e, nil
	}

	// Default: pass to focused input
	e.isDirty = true
	return e.updateFocusedInput(msg)
}

func (e *ModelEditor) save() (util.Model, tea.Cmd) {
	if !e.validate() {
		e.updateViewport()
		return e, nil
	}

	model := e.buildModel()

	return e, util.CmdHandler(ModelEditorSaveMsg{
		Model:      model,
		ProviderID: e.providerID,
	})
}

func (e *ModelEditor) validate() bool {
	e.errors = make(map[int]string)

	if strings.TrimSpace(e.inputs[fieldID].Value()) == "" {
		e.errors[fieldID] = "Model ID is required"
	}
	if strings.TrimSpace(e.inputs[fieldName].Value()) == "" {
		e.errors[fieldName] = "Model name is required"
	}

	// Validate numeric fields
	if _, err := strconv.ParseInt(e.inputs[fieldContextWindow].Value(), 10, 64); err != nil {
		e.errors[fieldContextWindow] = "Must be a valid number"
	}
	if _, err := strconv.ParseInt(e.inputs[fieldDefaultMaxTokens].Value(), 10, 64); err != nil {
		e.errors[fieldDefaultMaxTokens] = "Must be a valid number"
	}

	// Validate cost fields
	costFields := []int{
		fieldCostPer1MIn,
		fieldCostPer1MOut,
		fieldCostPer1MInCached,
		fieldCostPer1MOutCached,
	}

	for _, f := range costFields {
		if _, err := strconv.ParseFloat(e.inputs[f].Value(), 64); err != nil {
			e.errors[f] = "Must be a valid number"
		}
	}

	return len(e.errors) == 0
}

func (e *ModelEditor) buildModel() catwalk.Model {
	parseInt := func(s string, def int64) int64 {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
		return def
	}

	parseFloat := func(s string, def float64) float64 {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
		return def
	}

	parseBool := func(s string) bool {
		return strings.ToLower(strings.TrimSpace(s)) == "true"
	}

	return catwalk.Model{
		ID:                     e.inputs[fieldID].Value(),
		Name:                   e.inputs[fieldName].Value(),
		ContextWindow:          parseInt(e.inputs[fieldContextWindow].Value(), 128000),
		DefaultMaxTokens:       parseInt(e.inputs[fieldDefaultMaxTokens].Value(), 4096),
		CostPer1MIn:            parseFloat(e.inputs[fieldCostPer1MIn].Value(), 0),
		CostPer1MOut:           parseFloat(e.inputs[fieldCostPer1MOut].Value(), 0),
		CostPer1MInCached:      parseFloat(e.inputs[fieldCostPer1MInCached].Value(), 0),
		CostPer1MOutCached:     parseFloat(e.inputs[fieldCostPer1MOutCached].Value(), 0),
		CanReason:              parseBool(e.inputs[fieldCanReason].Value()),
		ReasoningLevels:        strings.Split(e.inputs[fieldReasoningLevels].Value(), ","),
		DefaultReasoningEffort: e.inputs[fieldDefaultReasoningEffort].Value(),
		SupportsImages:         parseBool(e.inputs[fieldSupportsImages].Value()),
	}
}

func (e *ModelEditor) nextField() {
	e.blurCurrentField()
	e.focusedField = (e.focusedField + 1) % fieldCount
	e.focusCurrentField()
	e.updateViewport()
	e.scrollToFocusedField()
}

func (e *ModelEditor) prevField() {
	e.blurCurrentField()
	e.focusedField = (e.focusedField - 1 + fieldCount) % fieldCount
	e.focusCurrentField()
	e.updateViewport()
	e.scrollToFocusedField()
}

func (e *ModelEditor) focusCurrentField() {
	if e.focusedField >= 0 && e.focusedField < len(e.inputs) {
		e.inputs[e.focusedField].Focus()
	}
}

func (e *ModelEditor) blurCurrentField() {
	for i := range e.inputs {
		e.inputs[i].Blur()
	}
}

func (e *ModelEditor) scrollToFocusedField() {
	linePos := 0

	// Calculate line position based on sections
	found := false
	for _, section := range sections {
		linePos += 1 // Section header
		for _, field := range section.fields {
			if field.index == e.focusedField {
				found = true
				break
			}
			linePos += 1 // Field line
			if _, hasError := e.errors[field.index]; hasError {
				linePos += 1 // Error line
			}
		}
		if found {
			break
		}
		linePos += 1 // Empty line after section
	}

	visibleLines := e.viewport.VisibleLineCount()
	halfVisible := visibleLines / 2

	targetOffset := linePos - halfVisible
	if targetOffset < 0 {
		targetOffset = 0
	}

	maxOffset := e.viewport.TotalLineCount() - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if targetOffset > maxOffset {
		targetOffset = maxOffset
	}

	e.viewport.SetYOffset(targetOffset)
}

func (e *ModelEditor) updateFocusedInput(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	var cmd tea.Cmd
	if e.focusedField >= 0 && e.focusedField < len(e.inputs) {
		e.inputs[e.focusedField], cmd = e.inputs[e.focusedField].Update(msg)
	}
	e.updateViewport()
	return e, cmd
}

func (e *ModelEditor) updateViewport() {
	content := e.renderContent()
	contentHeight := lipgloss.Height(content)

	minViewportHeight := 10
	availableHeight := max(minViewportHeight, e.wHeight/2-10)
	viewportHeight := min(contentHeight, availableHeight)

	e.viewport.SetWidth(e.width - 4)
	e.viewport.SetHeight(viewportHeight)
	e.viewport.SetContent(content)
}

func (e *ModelEditor) renderContent() string {
	t := styles.CurrentTheme()
	var lines []string

	for _, section := range sections {
		lines = append(lines, t.S().Base.Foreground(t.FgMuted).Render("    "+section.title))
		for _, field := range section.fields {
			lines = append(lines, e.renderModelField(field.label, e.inputs[field.index], field.index))
		}
		lines = append(lines, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (e *ModelEditor) renderModelField(label string, input textinput.Model, fieldIndex int) string {
	t := styles.CurrentTheme()
	fieldLabel := t.S().Base.Width(24).Render(label + ":")
	fieldView := input.View()

	field := lipgloss.JoinHorizontal(lipgloss.Top, "      ", fieldLabel, " ", fieldView)

	if err, ok := e.errors[fieldIndex]; ok {
		errorMsg := t.S().Base.Foreground(t.Error).Render("        ↳ " + err)
		return lipgloss.JoinVertical(lipgloss.Left, field, errorMsg)
	}

	return field
}

func (e *ModelEditor) View() string {
	if e.showConfirm {
		return e.confirmDialog.View()
	}

	t := styles.CurrentTheme()

	title := core.Title("Edit Model Configuration", e.width-4)
	viewportView := e.viewport.View()

	helpView := e.help.View(e.keyMap)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(title),
		viewportView,
		"",
		t.S().Base.Width(e.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(helpView),
	)

	return t.S().Base.
		Width(e.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Render(content)
}

func (e *ModelEditor) Position() (int, int) {
	helpHeight := lipgloss.Height(e.help.View(e.keyMap))
	totalHeight := 5 + e.viewport.Height() + helpHeight

	row := (e.wHeight - totalHeight) / 2
	col := (e.wWidth - e.width) / 2

	return row, col
}

func (e *ModelEditor) Cursor() *tea.Cursor {
	var cursor *tea.Cursor

	if e.focusedField >= 0 && e.focusedField < len(e.inputs) {
		cursor = e.inputs[e.focusedField].Cursor()
	}

	if cursor != nil {
		row, col := e.Position()
		offset := row + 3
		cursor.Y += offset
		cursor.X = cursor.X + col + 2
	}

	return cursor
}
