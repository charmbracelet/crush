package messages

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

type renderer interface {
	// Render returns the complete (already styled) tool‑call view, not
	// including the outer border.
	Render(v *toolCallCmp) string
}

type rendererFactory func() renderer

type renderRegistry map[string]rendererFactory

func (rr renderRegistry) register(name string, f rendererFactory) { rr[name] = f }
func (rr renderRegistry) lookup(name string) renderer {
	if f, ok := rr[name]; ok {
		return f()
	}
	return genericRenderer{} // sensible fallback
}

var registry = renderRegistry{}

// Registger tool renderers
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
	registry.register(tools.PatchToolName, func() renderer { return patchRenderer{} })
	registry.register(tools.DiagnosticsToolName, func() renderer { return diagnosticsRenderer{} })
}

// -----------------------------------------------------------------------------
//  Generic renderer
// -----------------------------------------------------------------------------

type genericRenderer struct{}

func (genericRenderer) Render(v *toolCallCmp) string {
	header := makeHeader(prettifyToolName(v.call.Name), v.textWidth(), v.call.Input)
	if res, done := earlyState(header, v); done {
		return res
	}
	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Bash renderer
// -----------------------------------------------------------------------------

type bashRenderer struct{}

func (bashRenderer) Render(v *toolCallCmp) string {
	var p tools.BashParams
	_ = json.Unmarshal([]byte(v.call.Input), &p)

	cmd := strings.ReplaceAll(p.Command, "\n", " ")
	header := makeHeader("Bash", v.textWidth(), cmd)
	if res, done := earlyState(header, v); done {
		return res
	}
	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  View renderer
// -----------------------------------------------------------------------------

type viewRenderer struct{}

func (viewRenderer) Render(v *toolCallCmp) string {
	var params tools.ViewParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	file := removeWorkingDirPrefix(params.FilePath)
	args := []string{file}
	if params.Limit != 0 {
		args = append(args, "limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset != 0 {
		args = append(args, "offset", fmt.Sprintf("%d", params.Offset))
	}

	header := makeHeader("View", v.textWidth(), args...)
	if res, done := earlyState(header, v); done {
		return res
	}

	var meta tools.ViewResponseMetadata
	_ = json.Unmarshal([]byte(v.result.Metadata), &meta)

	body := renderCodeContent(v, meta.FilePath, meta.Content, params.Offset)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Edit renderer
// -----------------------------------------------------------------------------

type editRenderer struct{}

func (editRenderer) Render(v *toolCallCmp) string {
	var params tools.EditParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	file := removeWorkingDirPrefix(params.FilePath)
	header := makeHeader("Edit", v.textWidth(), file)
	if res, done := earlyState(header, v); done {
		return res
	}

	var meta tools.EditResponseMetadata
	_ = json.Unmarshal([]byte(v.result.Metadata), &meta)

	trunc := truncateHeight(meta.Diff, responseContextHeight)
	diffView, _ := diff.FormatDiff(trunc, diff.WithTotalWidth(v.textWidth()))
	return joinHeaderBody(header, diffView)
}

// -----------------------------------------------------------------------------
//  Write renderer
// -----------------------------------------------------------------------------

type writeRenderer struct{}

func (writeRenderer) Render(v *toolCallCmp) string {
	var params tools.WriteParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	file := removeWorkingDirPrefix(params.FilePath)
	header := makeHeader("Write", v.textWidth(), file)
	if res, done := earlyState(header, v); done {
		return res
	}

	body := renderCodeContent(v, file, params.Content, 0)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Fetch renderer
// -----------------------------------------------------------------------------

type fetchRenderer struct{}

func (fetchRenderer) Render(v *toolCallCmp) string {
	var params tools.FetchParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	args := []string{params.URL}
	if params.Format != "" {
		args = append(args, "format", params.Format)
	}
	if params.Timeout != 0 {
		args = append(args, "timeout", (time.Duration(params.Timeout) * time.Second).String())
	}

	header := makeHeader("Fetch", v.textWidth(), args...)
	if res, done := earlyState(header, v); done {
		return res
	}

	file := "fetch.md"
	switch params.Format {
	case "text":
		file = "fetch.txt"
	case "html":
		file = "fetch.html"
	}

	body := renderCodeContent(v, file, v.result.Content, 0)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Glob renderer
// -----------------------------------------------------------------------------

type globRenderer struct{}

func (globRenderer) Render(v *toolCallCmp) string {
	var params tools.GlobParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	args := []string{params.Pattern}
	if params.Path != "" {
		args = append(args, "path", params.Path)
	}

	header := makeHeader("Glob", v.textWidth(), args...)
	if res, done := earlyState(header, v); done {
		return res
	}

	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Grep renderer
// -----------------------------------------------------------------------------

type grepRenderer struct{}

func (grepRenderer) Render(v *toolCallCmp) string {
	var params tools.GrepParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	args := []string{params.Pattern}
	if params.Path != "" {
		args = append(args, "path", params.Path)
	}
	if params.Include != "" {
		args = append(args, "include", params.Include)
	}
	if params.LiteralText {
		args = append(args, "literal", "true")
	}

	header := makeHeader("Grep", v.textWidth(), args...)
	if res, done := earlyState(header, v); done {
		return res
	}

	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  LS renderer
// -----------------------------------------------------------------------------

type lsRenderer struct{}

func (lsRenderer) Render(v *toolCallCmp) string {
	var params tools.LSParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	path := params.Path
	if path == "" {
		path = "."
	}

	header := makeHeader("List", v.textWidth(), path)
	if res, done := earlyState(header, v); done {
		return res
	}

	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Sourcegraph renderer
// -----------------------------------------------------------------------------

type sourcegraphRenderer struct{}

func (sourcegraphRenderer) Render(v *toolCallCmp) string {
	var params tools.SourcegraphParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	args := []string{params.Query}
	if params.Count != 0 {
		args = append(args, "count", fmt.Sprintf("%d", params.Count))
	}
	if params.ContextWindow != 0 {
		args = append(args, "context", fmt.Sprintf("%d", params.ContextWindow))
	}

	header := makeHeader("Sourcegraph", v.textWidth(), args...)
	if res, done := earlyState(header, v); done {
		return res
	}

	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Patch renderer
// -----------------------------------------------------------------------------

type patchRenderer struct{}

func (patchRenderer) Render(v *toolCallCmp) string {
	var params tools.PatchParams
	_ = json.Unmarshal([]byte(v.call.Input), &params)

	header := makeHeader("Patch", v.textWidth(), "multiple files")
	if res, done := earlyState(header, v); done {
		return res
	}

	var meta tools.PatchResponseMetadata
	_ = json.Unmarshal([]byte(v.result.Metadata), &meta)

	// Format the result as a summary of changes
	summary := fmt.Sprintf("Changed %d files (%d+ %d-)",
		len(meta.FilesChanged), meta.Additions, meta.Removals)

	// List the changed files
	filesList := strings.Join(meta.FilesChanged, "\n")

	body := renderPlainContent(v, summary+"\n\n"+filesList)
	return joinHeaderBody(header, body)
}

// -----------------------------------------------------------------------------
//  Diagnostics renderer
// -----------------------------------------------------------------------------

type diagnosticsRenderer struct{}

func (diagnosticsRenderer) Render(v *toolCallCmp) string {
	header := makeHeader("Diagnostics", v.textWidth(), "project")
	if res, done := earlyState(header, v); done {
		return res
	}

	body := renderPlainContent(v, v.result.Content)
	return joinHeaderBody(header, body)
}

// makeHeader builds "<Tool>: param (key=value)" and truncates as needed.
func makeHeader(tool string, width int, params ...string) string {
	prefix := tool + ": "
	return prefix + renderParams(width-lipgloss.Width(prefix), params...)
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

// earlyState returns immediately‑rendered error/cancelled/ongoing states.
func earlyState(header string, v *toolCallCmp) (string, bool) {
	switch {
	case v.result.IsError:
		return lipgloss.JoinVertical(lipgloss.Left, header, v.renderToolError()), true
	case v.cancelled:
		return lipgloss.JoinVertical(lipgloss.Left, header, "Cancelled"), true
	case v.result.ToolCallID == "":
		return lipgloss.JoinVertical(lipgloss.Left, header, "Waiting for tool to finish..."), true
	default:
		return "", false
	}
}

func joinHeaderBody(header, body string) string {
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "")
}

func renderPlainContent(v *toolCallCmp, content string) string {
	t := theme.CurrentTheme()
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")

	var out []string
	for i, ln := range lines {
		if i >= responseContextHeight {
			break
		}
		ln = " " + ln // left padding
		if len(ln) > v.textWidth() {
			ln = v.fit(ln, v.textWidth())
		}
		out = append(out, lipgloss.NewStyle().
			Width(v.textWidth()).
			Background(t.BackgroundSecondary()).
			Foreground(t.TextMuted()).
			Render(ln))
	}

	if len(lines) > responseContextHeight {
		out = append(out, lipgloss.NewStyle().
			Background(t.BackgroundSecondary()).
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf("... (%d lines)", len(lines)-responseContextHeight)))
	}
	return strings.Join(out, "\n")
}

func renderCodeContent(v *toolCallCmp, path, content string, offset int) string {
	t := theme.CurrentTheme()
	truncated := truncateHeight(content, responseContextHeight)

	highlighted, _ := highlight.SyntaxHighlight(truncated, path, t.BackgroundSecondary())
	lines := strings.Split(highlighted, "\n")

	if len(strings.Split(content, "\n")) > responseContextHeight {
		lines = append(lines, lipgloss.NewStyle().
			Background(t.BackgroundSecondary()).
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf("... (%d lines)", len(strings.Split(content, "\n"))-responseContextHeight)))
	}

	for i, ln := range lines {
		num := lipgloss.NewStyle().
			PaddingLeft(4).PaddingRight(2).
			Background(t.BackgroundSecondary()).
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf("%d", i+1+offset))
		w := v.textWidth() - lipgloss.Width(num)
		lines[i] = lipgloss.JoinHorizontal(lipgloss.Left,
			num,
			lipgloss.NewStyle().
				Width(w).
				Background(t.BackgroundSecondary()).
				Render(v.fit(ln, w)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (v *toolCallCmp) renderToolError() string {
	t := theme.CurrentTheme()
	err := strings.ReplaceAll(v.result.Content, "\n", " ")
	err = fmt.Sprintf("Error: %s", err)
	return styles.BaseStyle().Foreground(t.Error()).Render(v.fit(err, v.textWidth()))
}

func removeWorkingDirPrefix(path string) string {
	wd := config.WorkingDirectory()
	return strings.TrimPrefix(path, wd)
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
	case tools.PatchToolName:
		return "Patch"
	default:
		return name
	}
}

func toolAction(name string) string {
	switch name {
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
