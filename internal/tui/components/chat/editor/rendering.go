package editor

import (
	"fmt"

	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

// renderSelectedText renders textarea with selection highlighting
func (m *editorCmp) renderSelectedText() string {
	t := styles.CurrentTheme()
	value := m.textarea.Value()
	
	if !m.HasSelection() {
		return m.textarea.View()
	}
	
	// Get selection bounds
	selection := m.selection.GetSelection()
	start, end := selection.Bounds()
	
	// Render with selection highlighting
	before := value[:start]
	selected := value[start:end]
	after := value[end:]
	
	// Apply selection style to selected part
	selectedStyled := t.TextSelection.Render(selected)
	
	// Combine parts and set back to textarea
	fullContent := before + selectedStyled + after
	
	// Create a temporary textarea to render styled content
	tempTextarea := *m.textarea
	tempTextarea.SetValue(fullContent)
	
	return tempTextarea.View()
}

// attachmentsContent renders attachments display
func (m *editorCmp) attachmentsContent() string {
	var styledAttachments []string
	t := styles.CurrentTheme()
	attachmentStyles := t.S().Base.
		MarginLeft(1).
		Background(t.FgMuted).
		Foreground(t.FgBase)
	for i, attachment := range m.attachments {
		var filename string
		if len(attachment.FileName) > 10 {
			filename = fmt.Sprintf(" %s %s...", styles.DocumentIcon, attachment.FileName[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s", styles.DocumentIcon, attachment.FileName)
		}
		if m.deleteMode {
			filename = fmt.Sprintf("%d%s", i, filename)
		}
		styledAttachments = append(styledAttachments, attachmentStyles.Render(filename))
	}
	content := lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...)
	return content
}

// View renders the editor component with selection highlighting and attachments
func (m *editorCmp) View() string {
	t := styles.CurrentTheme()
	// Update placeholder
	if m.app.AgentCoordinator != nil && m.app.AgentCoordinator.IsBusy() {
		m.textarea.Placeholder = m.workingPlaceholder
	} else {
		m.textarea.Placeholder = m.readyPlaceholder
	}
	if m.app.Permissions.SkipRequests() {
		m.textarea.Placeholder = "Yolo mode!"
	}
	if len(m.attachments) == 0 {
		content := t.S().Base.Padding(1).Render(
			m.renderSelectedText(),
		)
		return content
	}
	content := t.S().Base.Padding(0, 1, 1, 1).Render(
		lipgloss.JoinVertical(lipgloss.Top,
			m.attachmentsContent(),
			m.renderSelectedText(),
		),
	)
	return content
}
