package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/diff"
	"github.com/opencode-ai/opencode/internal/highlight"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

const responseContextHeight = 10

func (m *messageCmp) renderUnfinishedToolCall() string {
	toolName := m.toolName()
	toolAction := m.getToolAction()
	return fmt.Sprintf("%s: %s", toolName, toolAction)
}

func (m *messageCmp) renderToolError() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()
	err := strings.ReplaceAll(m.toolResult.Content, "\n", " ")
	err = fmt.Sprintf("Error: %s", err)
	return baseStyle.Foreground(t.Error()).Render(m.fit(err))
}

func (m *messageCmp) renderBashTool() string {
	name := m.toolName()
	prefix := fmt.Sprintf("%s: ", name)
	var params tools.BashParams
	json.Unmarshal([]byte(m.toolCall.Input), &params)
	command := strings.ReplaceAll(params.Command, "\n", " ")
	header := prefix + renderParams(m.textWidth()-lipgloss.Width(prefix), command)

	if result, ok := m.toolResultErrorOrMissing(header); ok {
		return result
	}
	return m.renderTool(header, m.renderPlainContent(m.toolResult.Content))
}

func (m *messageCmp) renderViewTool() string {
	name := m.toolName()
	prefix := fmt.Sprintf("%s: ", name)
	var params tools.ViewParams
	json.Unmarshal([]byte(m.toolCall.Input), &params)
	filePath := removeWorkingDirPrefix(params.FilePath)
	toolParams := []string{
		filePath,
	}
	if params.Limit != 0 {
		toolParams = append(toolParams, "limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset != 0 {
		toolParams = append(toolParams, "offset", fmt.Sprintf("%d", params.Offset))
	}
	header := prefix + renderParams(m.textWidth()-lipgloss.Width(prefix), toolParams...)

	if result, ok := m.toolResultErrorOrMissing(header); ok {
		return result
	}

	metadata := tools.ViewResponseMetadata{}
	json.Unmarshal([]byte(m.toolResult.Metadata), &metadata)

	return m.renderTool(header, m.renderCodeContent(metadata.FilePath, metadata.Content, params.Offset))
}

func (m *messageCmp) renderCodeContent(path, content string, offset int) string {
	t := theme.CurrentTheme()
	originalHeight := lipgloss.Height(content)
	fileContent := truncateHeight(content, responseContextHeight)

	highlighted, _ := highlight.SyntaxHighlight(fileContent, path, t.BackgroundSecondary())

	lines := strings.Split(highlighted, "\n")

	if originalHeight > responseContextHeight {
		lines = append(lines,
			lipgloss.NewStyle().Background(t.BackgroundSecondary()).
				Foreground(t.TextMuted()).
				Render(
					fmt.Sprintf("... (%d lines)", originalHeight-responseContextHeight),
				),
		)
	}
	for i, line := range lines {
		lineNumber := lipgloss.NewStyle().
			PaddingLeft(4).
			PaddingRight(2).
			Background(t.BackgroundSecondary()).
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf("%d", i+1+offset))
		formattedLine := lipgloss.NewStyle().
			Width(m.textWidth() - lipgloss.Width(lineNumber)).
			Background(t.BackgroundSecondary()).Render(line)
		lines[i] = lipgloss.JoinHorizontal(lipgloss.Left, lineNumber, formattedLine)
	}
	return lipgloss.NewStyle().Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			lines...,
		),
	)
}

func (m *messageCmp) renderPlainContent(content string) string {
	t := theme.CurrentTheme()
	content = strings.TrimSuffix(content, "\n")
	content = strings.TrimPrefix(content, "\n")
	lines := strings.Split(fmt.Sprintf("\n%s\n", content), "\n")

	for i, line := range lines {
		line = " " + line // add padding
		if len(line) > m.textWidth() {
			line = m.fit(line)
		}
		lines[i] = lipgloss.NewStyle().
			Width(m.textWidth()).
			Background(t.BackgroundSecondary()).
			Foreground(t.TextMuted()).
			Render(line)
	}
	if len(lines) > responseContextHeight {
		lines = lines[:responseContextHeight]
		lines = append(lines,
			lipgloss.NewStyle().Background(t.BackgroundSecondary()).
				Foreground(t.TextMuted()).
				Render(
					fmt.Sprintf("... (%d lines)", len(lines)-responseContextHeight),
				),
		)
	}
	return strings.Join(lines, "\n")
}

func (m *messageCmp) renderGenericTool() string {
	// Tool params
	name := m.toolName()
	prefix := fmt.Sprintf("%s: ", name)
	input := strings.ReplaceAll(m.toolCall.Input, "\n", " ")
	params := renderParams(m.textWidth()-lipgloss.Width(prefix), input)
	header := prefix + params

	if result, ok := m.toolResultErrorOrMissing(header); ok {
		return result
	}
	return m.renderTool(header, m.renderPlainContent(m.toolResult.Content))
}

func (m *messageCmp) renderEditTool() string {
	// Tool params
	name := m.toolName()
	prefix := fmt.Sprintf("%s: ", name)
	var params tools.EditParams
	json.Unmarshal([]byte(m.toolCall.Input), &params)
	filePath := removeWorkingDirPrefix(params.FilePath)
	header := prefix + renderParams(m.textWidth()-lipgloss.Width(prefix), filePath)

	if result, ok := m.toolResultErrorOrMissing(header); ok {
		return result
	}
	metadata := tools.EditResponseMetadata{}
	json.Unmarshal([]byte(m.toolResult.Metadata), &metadata)
	truncDiff := truncateHeight(metadata.Diff, maxResultHeight)
	formattedDiff, _ := diff.FormatDiff(truncDiff, diff.WithTotalWidth(m.textWidth()))
	return m.renderTool(header, formattedDiff)
}

func (m *messageCmp) renderWriteTool() string {
	// Tool params
	name := m.toolName()
	prefix := fmt.Sprintf("%s: ", name)
	var params tools.WriteParams
	json.Unmarshal([]byte(m.toolCall.Input), &params)
	filePath := removeWorkingDirPrefix(params.FilePath)
	header := prefix + renderParams(m.textWidth()-lipgloss.Width(prefix), filePath)
	if result, ok := m.toolResultErrorOrMissing(header); ok {
		return result
	}
	return m.renderTool(header, m.renderCodeContent(filePath, params.Content, 0))
}

func (m *messageCmp) renderToolCallMessage() string {
	if !m.toolCall.Finished && !m.cancelledToolCall {
		return m.renderUnfinishedToolCall()
	}
	content := ""
	switch m.toolCall.Name {
	case tools.ViewToolName:
		content = m.renderViewTool()
	case tools.BashToolName:
		content = m.renderBashTool()
	case tools.EditToolName:
		content = m.renderEditTool()
	case tools.WriteToolName:
		content = m.renderWriteTool()
	default:
		content = m.renderGenericTool()
	}
	return m.style().PaddingLeft(1).Render(content)
}

func (m *messageCmp) toolResultErrorOrMissing(header string) (string, bool) {
	result := "Waiting for tool to finish..."
	if m.toolResult.IsError {
		result = m.renderToolError()
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			result,
		), true
	} else if m.cancelledToolCall {
		result = "Cancelled"
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			result,
		), true
	} else if m.toolResult.ToolCallID == "" {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			result,
		), true
	}

	return "", false
}

func (m *messageCmp) renderTool(header, result string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		result,
		"",
	)
}

func removeWorkingDirPrefix(path string) string {
	wd := config.WorkingDirectory()
	path = strings.TrimPrefix(path, wd)
	return path
}

func truncateHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		return strings.Join(lines[:height], "\n")
	}
	return content
}

func (m *messageCmp) fit(content string) string {
	return ansi.Truncate(content, m.textWidth(), "...")
}

func (m *messageCmp) toolName() string {
	switch m.toolCall.Name {
	case agent.AgentToolName:
		return "Task"
	case tools.BashToolName:
		return "Bash"
	case tools.EditToolName:
		return "Edit"
	case tools.FetchToolName:
		return "Fetch"
	case tools.GlobToolName:
		return "Glob"
	case tools.GrepToolName:
		return "Grep"
	case tools.LSToolName:
		return "List"
	case tools.SourcegraphToolName:
		return "Sourcegraph"
	case tools.ViewToolName:
		return "View"
	case tools.WriteToolName:
		return "Write"
	case tools.PatchToolName:
		return "Patch"
	default:
		return m.toolCall.Name
	}
}

func (m *messageCmp) getToolAction() string {
	switch m.toolCall.Name {
	case agent.AgentToolName:
		return "Preparing prompt..."
	case tools.BashToolName:
		return "Building command..."
	case tools.EditToolName:
		return "Preparing edit..."
	case tools.FetchToolName:
		return "Writing fetch..."
	case tools.GlobToolName:
		return "Finding files..."
	case tools.GrepToolName:
		return "Searching content..."
	case tools.LSToolName:
		return "Listing directory..."
	case tools.SourcegraphToolName:
		return "Searching code..."
	case tools.ViewToolName:
		return "Reading file..."
	case tools.WriteToolName:
		return "Preparing write..."
	case tools.PatchToolName:
		return "Preparing patch..."
	default:
		return "Working..."
	}
}

// renders params, params[0] (params[1]=params[2] ....)
func renderParams(paramsWidth int, params ...string) string {
	if len(params) == 0 {
		return ""
	}
	mainParam := params[0]
	if len(mainParam) > paramsWidth {
		mainParam = mainParam[:paramsWidth-3] + "..."
	}

	if len(params) == 1 {
		return mainParam
	}
	otherParams := params[1:]
	// create pairs of key/value
	// if odd number of params, the last one is a key without value
	if len(otherParams)%2 != 0 {
		otherParams = append(otherParams, "")
	}
	parts := make([]string, 0, len(otherParams)/2)
	for i := 0; i < len(otherParams); i += 2 {
		key := otherParams[i]
		value := otherParams[i+1]
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	partsRendered := strings.Join(parts, ", ")
	remainingWidth := paramsWidth - lipgloss.Width(partsRendered) - 3 // count for " ()"
	if remainingWidth < 30 {
		// No space for the params, just show the main
		return mainParam
	}

	if len(parts) > 0 {
		mainParam = fmt.Sprintf("%s (%s)", mainParam, strings.Join(parts, ", "))
	}

	return ansi.Truncate(mainParam, paramsWidth, "...")
}
