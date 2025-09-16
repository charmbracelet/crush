// Package playground implements an interactive code playground within the Blush TUI.
// It provides a built-in code editor, execution environment, and debugging capabilities.
package playground

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/v2/textarea"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
)

// Model represents the playground component state
type Model struct {
	width     int
	height    int
	editor    *textarea.Model
	output    outputModel
	focus     focusState
	executing bool
	language  string
	executor  *Executor
}

// focusState represents which panel currently has focus
type focusState int

const (
	editorFocus focusState = iota
	outputFocus
)

// New creates a new playground model
func New() *Model {
	ta := textarea.New()
	ta.Placeholder = "Enter your code here..."
	ta.Focus()
	
	output := newOutputModel()
	
	return &Model{
		editor:   ta,
		output:   output,
		focus:    editorFocus,
		language: "go", // Default to Go
		executor: NewExecutor(),
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+e":
			m.focus = editorFocus
			m.editor.Focus()
		case "ctrl+o":
			m.focus = outputFocus
			m.editor.Blur()
		case "ctrl+r":
			return m, m.runCode()
		case "tab":
			if m.focus == editorFocus {
				m.focus = outputFocus
				m.editor.Blur()
			} else {
				m.focus = editorFocus
				m.editor.Focus()
			}
		case "ctrl+l":
			return m, m.changeLanguage()
		}

	case runResultMsg:
		m.output.content = append(m.output.content, string(msg))
		m.executing = false
		return m, nil

	case runErrorMsg:
		m.output.content = append(m.output.content, fmt.Sprintf("Error: %s", string(msg)))
		m.executing = false
		return m, nil
	}

	// Update the focused component
	if m.focus == editorFocus {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		model, cmd := m.output.Update(msg)
		if outputModel, ok := model.(outputModel); ok {
			m.output = outputModel
		}
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m *Model) View() string {
	t := styles.CurrentTheme()
	
	// Create header
	header := t.S().Base.Bold(true).Render(fmt.Sprintf("Interactive Code Playground (%s)", m.language))
	
	// Create editor and output views
	editorView := m.editor.View()
	outputView := m.output.View()
	
	// Create focus indicators
	editorTitle := "Editor"
	outputTitle := "Output"
	if m.focus == editorFocus {
		editorTitle = "> Editor <"
	} else {
		outputTitle = "> Output <"
	}
	
	// Style the panels
	editorPanel := lipgloss.JoinVertical(lipgloss.Left,
		t.S().Base.Foreground(t.Secondary).Render(editorTitle),
		editorView,
	)
	
	outputPanel := lipgloss.JoinVertical(lipgloss.Left,
		t.S().Base.Foreground(t.Secondary).Render(outputTitle),
		outputView,
	)
	
	// Create control bar
	controls := m.createControlBar()
	
	// Layout the components
	content := lipgloss.JoinHorizontal(lipgloss.Top,
		editorPanel,
		outputPanel,
	)
	
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		controls,
	)
}

// createControlBar creates the control bar with action buttons
func (m *Model) createControlBar() string {
	t := styles.CurrentTheme()
	
	// Create control buttons
	runButton := "[Run]"
	if m.executing {
		runButton = "[Running...]"
	}
	
	buttons := []string{
		t.S().Base.Foreground(t.Primary).Render(runButton),
		t.S().Base.Foreground(t.Primary).Render("[Run Selection]"),
		t.S().Base.Foreground(t.Primary).Render("[Stop]"),
		t.S().Base.Foreground(t.Primary).Render("[Debug]"),
		t.S().Base.Foreground(t.Primary).Render("[Save]"),
		t.S().Base.Foreground(t.Primary).Render("[Load]"),
		t.S().Base.Foreground(t.Primary).Render(fmt.Sprintf("[Language: %s]", m.language)),
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
}

// SetSize sets the size of the playground
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	
	// Distribute space between editor and output (roughly 50/50)
	panelWidth := width / 2
	panelHeight := height - 5 // Account for header and controls
	
	m.editor.SetWidth(panelWidth)
	m.editor.SetHeight(panelHeight)
	m.output.SetSize(panelWidth, panelHeight)
}

// runCode executes the current code in the editor
func (m *Model) runCode() tea.Cmd {
	code := m.editor.Value()
	
	m.executing = true
	m.output.content = append(m.output.content, fmt.Sprintf("Executing %s code...", m.language))
	
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.executor.Execute(ctx, m.language, code)
		if err != nil {
			return runErrorMsg(err.Error())
		}
		
		if result.Success {
			return runResultMsg(result.Stdout)
		}
		return runErrorMsg(result.Stderr)
	}
}

// changeLanguage cycles through supported languages
func (m *Model) changeLanguage() tea.Cmd {
	languages := []string{"go", "javascript", "python"}
	for i, lang := range languages {
		if lang == m.language && i < len(languages)-1 {
			m.language = languages[i+1]
			return nil
		} else if lang == m.language && i == len(languages)-1 {
			m.language = languages[0]
			return nil
		}
	}
	return nil
}

// runResultMsg represents the result of code execution
type runResultMsg string

// runErrorMsg represents an error during code execution
type runErrorMsg string