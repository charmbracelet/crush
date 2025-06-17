package messages

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/fileutil"
	"github.com/charmbracelet/crush/internal/highlight"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/lipgloss/v2/tree"
	"github.com/charmbracelet/x/ansi"
)

// responseContextHeight limits the number of lines displayed in tool output
const responseContextHeight = 10

// renderer defines the interface for tool-specific rendering implementations
type renderer interface {
	// Render returns the complete (already styled) tool‑call view, not
	// including the outer border.
	Render(v *toolCallCmp) string
}

// rendererFactory creates new renderer instances
type rendererFactory func() renderer

// renderRegistry manages the mapping of tool names to their renderers
type renderRegistry map[string]rendererFactory

// register adds a new renderer factory to the registry
func (rr renderRegistry) register(name string, f rendererFactory) { rr[name] = f }

// lookup retrieves a renderer for the given tool name, falling back to generic renderer
func (rr renderRegistry) lookup(name string) renderer {
	if f, ok := rr[name]; ok {
		return f()
	}
	return genericRenderer{} // sensible fallback
}

// registry holds all registered tool renderers
var registry = renderRegistry{}

// baseRenderer provides common functionality for all tool renderers
type baseRenderer struct{}

// paramBuilder helps construct parameter lists for tool headers
type paramBuilder struct {
	args []string
}

// newParamBuilder creates a new parameter builder
func newParamBuilder() *paramBuilder {
	return &paramBuilder{args: make([]string, 0)}
}

// addMain adds the main parameter (first argument)
func (pb *paramBuilder) addMain(value string) *paramBuilder {
	if value != "" {
		pb.args = append(pb.args, value)
	}
	return pb
}

// addKeyValue adds a key-value pair parameter
func (pb *paramBuilder) addKeyValue(key, value string) *paramBuilder {
	if value != "" {
		pb.args = append(pb.args, key, value)
	}
	return pb
}

// addFlag adds a boolean flag parameter
func (pb *paramBuilder) addFlag(key string, value bool) *paramBuilder {
	if value {
		pb.args = append(pb.args, key, "true")
	}
	return pb
}

// build returns the final parameter list
func (pb *paramBuilder) build() []string {
	return pb.args
}

// renderWithParams provides a common rendering pattern for tools with parameters
func (br baseRenderer) renderWithParams(v *toolCallCmp, toolName string, args []string, contentRenderer func() string) string {
	width := v.textWidth()
	if v.isNested {
		width -= 4 // Adjust for nested tool call indentation
	}
	header := br.makeHeader(v, toolName, width, args...)
	if v.isNested {
		return v.style().Render(header)
	}
	if res, done := earlyState(header, v); done {
		return res
	}
	body := contentRenderer()
	return joinHeaderBody(header, body)
}

// unmarshalParams safely unmarshal JSON parameters
func (br baseRenderer) unmarshalParams(input string, target any) error {
	return json.Unmarshal([]byte(input), target)
}

// makeHeader builds "<Tool>: param (key=value)" and truncates as needed.
func (br baseRenderer) makeHeader(v *toolCallCmp, tool string, width int, params ...string) string {
	t := styles.CurrentTheme()
	icon := t.S().Base.Foreground(t.GreenDark).Render(styles.ToolPending)
	if v.result.ToolCallID != "" {
		if v.result.IsError {
			icon = t.S().Base.Foreground(t.RedDark).Render(styles.ToolError)
		} else {
			icon = t.S().Base.Foreground(t.Green).Render(styles.ToolSuccess)
		}
	} else if v.cancelled {
		icon = t.S().Muted.Render(styles.ToolPending)
	}
	tool = t.S().Base.Foreground(t.Blue).Render(tool)
	prefix := fmt.Sprintf("%s %s: ", icon, tool)
	return prefix + renderParamList(width-lipgloss.Width(prefix), params...)
}

// renderError provides consistent error rendering
func (br baseRenderer) renderError(v *toolCallCmp, message string) string {
	t := styles.CurrentTheme()
	header := br.makeHeader(v, prettifyToolName(v.call.Name), v.textWidth(), "")
	message = t.S().Error.Render(v.fit(message, v.textWidth()-2)) // -2 for padding
	return joinHeaderBody(header, message)
}

// Register tool renderers
func init() {
	registry.register(tools.BashToolName, func() renderer { return bashRenderer{} })
	registry.register(tools.ViewToolName, func() renderer { return viewRenderer{} })
	registry.register(tools.EditToolName, func() renderer { return editRenderer{} })
	registry.register(tools.WriteToolName, func() renderer { return writeRenderer{} })
	registry.register(tools.FetchToolName, func() renderer { return fetchRenderer{} })
	registry.register(tools.GlobToolName, func() renderer { return globRenderer{} })
	registry.register(tools.GrepToolName, func() renderer { return grepRenderer{} })
	registry.register(tools.LSToolName, func() renderer { return lsRenderer{} })
	registry.register(tools.SourcegraphToolName, func() renderer { return sourcegraphRenderer{} })
	registry.register(tools.DiagnosticsToolName, func() renderer { return diagnosticsRenderer{} })
	registry.register(agent.AgentToolName, func() renderer { return agentRenderer{} })
}

// -----------------------------------------------------------------------------
//  Generic renderer
// -----------------------------------------------------------------------------

// genericRenderer handles unknown tool types with basic parameter display
type genericRenderer struct {
	baseRenderer
}

// Render displays the tool call with its raw input and plain content output
func (gr genericRenderer) Render(v *toolCallCmp) string {
	return gr.renderWithParams(v, prettifyToolName(v.call.Name), []string{v.call.Input}, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  Bash renderer
// -----------------------------------------------------------------------------

// bashRenderer handles bash command execution display
type bashRenderer struct {
	baseRenderer
}

// Render displays the bash command with sanitized newlines and plain output
func (br bashRenderer) Render(v *toolCallCmp) string {
	var params tools.BashParams
	if err := br.unmarshalParams(v.call.Input, &params); err != nil {
		return br.renderError(v, "Invalid bash parameters")
	}

	cmd := strings.ReplaceAll(params.Command, "\n", " ")
	args := newParamBuilder().addMain(cmd).build()

	return br.renderWithParams(v, "Bash", args, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  View renderer
// -----------------------------------------------------------------------------

// viewRenderer handles file viewing with syntax highlighting and line numbers
type viewRenderer struct {
	baseRenderer
}

// Render displays file content with optional limit and offset parameters
func (vr viewRenderer) Render(v *toolCallCmp) string {
	var params tools.ViewParams
	if err := vr.unmarshalParams(v.call.Input, &params); err != nil {
		return vr.renderError(v, "Invalid view parameters")
	}

	file := fileutil.PrettyPath(params.FilePath)
	args := newParamBuilder().
		addMain(file).
		addKeyValue("limit", formatNonZero(params.Limit)).
		addKeyValue("offset", formatNonZero(params.Offset)).
		build()

	return vr.renderWithParams(v, "View", args, func() string {
		var meta tools.ViewResponseMetadata
		if err := vr.unmarshalParams(v.result.Metadata, &meta); err != nil {
			return renderPlainContent(v, v.result.Content)
		}
		return renderCodeContent(v, meta.FilePath, meta.Content, params.Offset)
	})
}

// formatNonZero returns string representation of non-zero integers, empty string for zero
func formatNonZero(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

// -----------------------------------------------------------------------------
//  Edit renderer
// -----------------------------------------------------------------------------

// editRenderer handles file editing with diff visualization
type editRenderer struct {
	baseRenderer
}

// Render displays the edited file with a formatted diff of changes
func (er editRenderer) Render(v *toolCallCmp) string {
	var params tools.EditParams
	if err := er.unmarshalParams(v.call.Input, &params); err != nil {
		return er.renderError(v, "Invalid edit parameters")
	}

	file := fileutil.PrettyPath(params.FilePath)
	args := newParamBuilder().addMain(file).build()

	return er.renderWithParams(v, "Edit", args, func() string {
		var meta tools.EditResponseMetadata
		if err := er.unmarshalParams(v.result.Metadata, &meta); err != nil {
			return renderPlainContent(v, v.result.Content)
		}

		formatter := core.DiffFormatter().
			Before(fileutil.PrettyPath(params.FilePath), meta.OldContent).
			After(fileutil.PrettyPath(params.FilePath), meta.NewContent).
			Split().
			Width(v.textWidth() - 2) // -2 for padding
		return formatter.String()
	})
}

// -----------------------------------------------------------------------------
//  Write renderer
// -----------------------------------------------------------------------------

// writeRenderer handles file writing with syntax-highlighted content preview
type writeRenderer struct {
	baseRenderer
}

// Render displays the file being written with syntax highlighting
func (wr writeRenderer) Render(v *toolCallCmp) string {
	var params tools.WriteParams
	if err := wr.unmarshalParams(v.call.Input, &params); err != nil {
		return wr.renderError(v, "Invalid write parameters")
	}

	file := fileutil.PrettyPath(params.FilePath)
	args := newParamBuilder().addMain(file).build()

	return wr.renderWithParams(v, "Write", args, func() string {
		return renderCodeContent(v, file, params.Content, 0)
	})
}

// -----------------------------------------------------------------------------
//  Fetch renderer
// -----------------------------------------------------------------------------

// fetchRenderer handles URL fetching with format-specific content display
type fetchRenderer struct {
	baseRenderer
}

// Render displays the fetched URL with format and timeout parameters
func (fr fetchRenderer) Render(v *toolCallCmp) string {
	var params tools.FetchParams
	if err := fr.unmarshalParams(v.call.Input, &params); err != nil {
		return fr.renderError(v, "Invalid fetch parameters")
	}

	args := newParamBuilder().
		addMain(params.URL).
		addKeyValue("format", params.Format).
		addKeyValue("timeout", formatTimeout(params.Timeout)).
		build()

	return fr.renderWithParams(v, "Fetch", args, func() string {
		file := fr.getFileExtension(params.Format)
		return renderCodeContent(v, file, v.result.Content, 0)
	})
}

// getFileExtension returns appropriate file extension for syntax highlighting
func (fr fetchRenderer) getFileExtension(format string) string {
	switch format {
	case "text":
		return "fetch.txt"
	case "html":
		return "fetch.html"
	default:
		return "fetch.md"
	}
}

// formatTimeout converts timeout seconds to duration string
func formatTimeout(timeout int) string {
	if timeout == 0 {
		return ""
	}
	return (time.Duration(timeout) * time.Second).String()
}

// -----------------------------------------------------------------------------
//  Glob renderer
// -----------------------------------------------------------------------------

// globRenderer handles file pattern matching with path filtering
type globRenderer struct {
	baseRenderer
}

// Render displays the glob pattern with optional path parameter
func (gr globRenderer) Render(v *toolCallCmp) string {
	var params tools.GlobParams
	if err := gr.unmarshalParams(v.call.Input, &params); err != nil {
		return gr.renderError(v, "Invalid glob parameters")
	}

	args := newParamBuilder().
		addMain(params.Pattern).
		addKeyValue("path", params.Path).
		build()

	return gr.renderWithParams(v, "Glob", args, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  Grep renderer
// -----------------------------------------------------------------------------

// grepRenderer handles content searching with pattern matching options
type grepRenderer struct {
	baseRenderer
}

// Render displays the search pattern with path, include, and literal text options
func (gr grepRenderer) Render(v *toolCallCmp) string {
	var params tools.GrepParams
	if err := gr.unmarshalParams(v.call.Input, &params); err != nil {
		return gr.renderError(v, "Invalid grep parameters")
	}

	args := newParamBuilder().
		addMain(params.Pattern).
		addKeyValue("path", params.Path).
		addKeyValue("include", params.Include).
		addFlag("literal", params.LiteralText).
		build()

	return gr.renderWithParams(v, "Grep", args, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  LS renderer
// -----------------------------------------------------------------------------

// lsRenderer handles directory listing with default path handling
type lsRenderer struct {
	baseRenderer
}

// Render displays the directory path, defaulting to current directory
func (lr lsRenderer) Render(v *toolCallCmp) string {
	var params tools.LSParams
	if err := lr.unmarshalParams(v.call.Input, &params); err != nil {
		return lr.renderError(v, "Invalid ls parameters")
	}

	path := params.Path
	if path == "" {
		path = "."
	}
	path = fileutil.PrettyPath(path)

	args := newParamBuilder().addMain(path).build()

	return lr.renderWithParams(v, "List", args, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  Sourcegraph renderer
// -----------------------------------------------------------------------------

// sourcegraphRenderer handles code search with count and context options
type sourcegraphRenderer struct {
	baseRenderer
}

// Render displays the search query with optional count and context window parameters
func (sr sourcegraphRenderer) Render(v *toolCallCmp) string {
	var params tools.SourcegraphParams
	if err := sr.unmarshalParams(v.call.Input, &params); err != nil {
		return sr.renderError(v, "Invalid sourcegraph parameters")
	}

	args := newParamBuilder().
		addMain(params.Query).
		addKeyValue("count", formatNonZero(params.Count)).
		addKeyValue("context", formatNonZero(params.ContextWindow)).
		build()

	return sr.renderWithParams(v, "Sourcegraph", args, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  Diagnostics renderer
// -----------------------------------------------------------------------------

// diagnosticsRenderer handles project-wide diagnostic information
type diagnosticsRenderer struct {
	baseRenderer
}

// Render displays project diagnostics with plain content formatting
func (dr diagnosticsRenderer) Render(v *toolCallCmp) string {
	args := newParamBuilder().addMain("project").build()

	return dr.renderWithParams(v, "Diagnostics", args, func() string {
		return renderPlainContent(v, v.result.Content)
	})
}

// -----------------------------------------------------------------------------
//  Task renderer
// -----------------------------------------------------------------------------

// agentRenderer handles project-wide diagnostic information
type agentRenderer struct {
	baseRenderer
}

// Render displays agent task parameters and result content
func (tr agentRenderer) Render(v *toolCallCmp) string {
	var params agent.AgentParams
	if err := tr.unmarshalParams(v.call.Input, &params); err != nil {
		return tr.renderError(v, "Invalid task parameters")
	}
	prompt := params.Prompt
	prompt = strings.ReplaceAll(prompt, "\n", " ")
	args := newParamBuilder().addMain(prompt).build()

	header := tr.makeHeader(v, "Task", v.textWidth(), args...)
	t := tree.Root(header)

	for _, call := range v.nestedToolCalls {
		t.Child(call.View())
	}

	parts := []string{
		t.Enumerator(tree.RoundedEnumerator).String(),
	}
	if v.result.ToolCallID == "" {
		v.spinning = true
		parts = append(parts, v.anim.View().String())
	} else {
		v.spinning = false
	}

	header = lipgloss.JoinVertical(
		lipgloss.Left,
		parts...,
	)

	if v.result.ToolCallID == "" {
		return header
	}

	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// renderParamList renders params, params[0] (params[1]=params[2] ....)
func renderParamList(paramsWidth int, params ...string) string {
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

// earlyState returns immediately‑rendered error/cancelled/ongoing states.
func earlyState(header string, v *toolCallCmp) (string, bool) {
	t := styles.CurrentTheme()
	message := ""
	switch {
	case v.result.IsError:
		message = v.renderToolError()
	case v.cancelled:
		message = "Cancelled"
	case v.result.ToolCallID == "":
		message = "Waiting for tool to start..."
	default:
		return "", false
	}

	message = t.S().Base.PaddingLeft(2).Render(message)
	return lipgloss.JoinVertical(lipgloss.Left, header, message), true
}

func joinHeaderBody(header, body string) string {
	t := styles.CurrentTheme()
	body = t.S().Base.PaddingLeft(2).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, "")
}

func renderPlainContent(v *toolCallCmp, content string) string {
	t := styles.CurrentTheme()
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")

	width := v.textWidth() - 2 // -2 for left padding
	var out []string
	for i, ln := range lines {
		if i >= responseContextHeight {
			break
		}
		ln = " " + ln // left padding
		if len(ln) > width {
			ln = v.fit(ln, width)
		}
		out = append(out, t.S().Muted.
			Width(width).
			Background(t.BgSubtle).
			Render(ln))
	}

	if len(lines) > responseContextHeight {
		out = append(out, t.S().Muted.
			Background(t.BgSubtle).
			Width(width).
			Render(fmt.Sprintf("... (%d lines)", len(lines)-responseContextHeight)))
	}
	return strings.Join(out, "\n")
}

func renderCodeContent(v *toolCallCmp, path, content string, offset int) string {
	t := styles.CurrentTheme()
	truncated := truncateHeight(content, responseContextHeight)

	highlighted, _ := highlight.SyntaxHighlight(truncated, path, t.BgSubtle)
	lines := strings.Split(highlighted, "\n")

	if len(strings.Split(content, "\n")) > responseContextHeight {
		lines = append(lines, t.S().Muted.
			Background(t.BgSubtle).
			Width(v.textWidth()-2).
			Render(fmt.Sprintf("... (%d lines)", len(strings.Split(content, "\n"))-responseContextHeight)))
	}

	for i, ln := range lines {
		num := t.S().Muted.
			Background(t.BgSubtle).
			PaddingLeft(4).
			PaddingRight(2).
			Render(fmt.Sprintf("%d", i+1+offset))
		w := v.textWidth() - 2 - lipgloss.Width(num) // -2 for left padding
		lines[i] = lipgloss.JoinHorizontal(lipgloss.Left,
			num,
			t.S().Base.
				Width(w).
				Background(t.BgSubtle).
				Render(v.fit(ln, w)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (v *toolCallCmp) renderToolError() string {
	t := styles.CurrentTheme()
	err := strings.ReplaceAll(v.result.Content, "\n", " ")
	err = fmt.Sprintf("Error: %s", err)
	return t.S().Base.Foreground(t.Error).Render(v.fit(err, v.textWidth()))
}

func truncateHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		return strings.Join(lines[:h], "\n")
	}
	return s
}

func prettifyToolName(name string) string {
	switch name {
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
	default:
		return name
	}
}
