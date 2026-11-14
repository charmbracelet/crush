package format

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

type ToolStatus int

const (
	ToolStatusAwaitingPermission ToolStatus = iota
	ToolStatusRunning
	ToolStatusSuccess
	ToolStatusError
	ToolStatusCanceled
)

type ToolHeaderOptions struct {
	ExpandedContent bool
	Compact         bool
}

func ToolIcon(sty *styles.Styles, status ToolStatus) string {
	switch status {
	case ToolStatusSuccess:
		return sty.Tool.IconSuccess.String()
	case ToolStatusError:
		return sty.Tool.IconError.String()
	case ToolStatusCanceled:
		return sty.Tool.IconCancelled.String()
	default:
		return sty.Tool.IconPending.String()
	}
}

func ToolParamList(sty *styles.Styles, params []string, width int, opts *ToolHeaderOptions) string {
	const minSpaceForMainParam = 30
	if len(params) == 0 {
		return ""
	}

	mainParam := params[0]
	var kvPairs []string
	for i := 1; i+1 < len(params); i += 2 {
		if params[i+1] != "" {
			kvPairs = append(kvPairs, fmt.Sprintf("%s=%s", params[i], params[i+1]))
		}
	}

	output := mainParam
	if len(kvPairs) > 0 {
		partsStr := strings.Join(kvPairs, ", ")
		if remaining := width - lipgloss.Width(partsStr) - 3; remaining >= minSpaceForMainParam {
			pairs := sty.Tool.ParamMain.Render(fmt.Sprintf("(%s)", partsStr))
			output = fmt.Sprintf("%s %s", mainParam, pairs)
		}
	}

	if opts != nil && opts.Compact {
		output = strings.ReplaceAll(output, "\n", " ")
	}
	if width >= 0 && (opts == nil || !opts.ExpandedContent || opts.Compact) {
		return sty.Tool.ParamMain.Render(ansi.Truncate(output, width, "…"))
	}
	if opts != nil && opts.ExpandedContent && width > 0 && lipgloss.Width(output) > width {
		return capToolHeaderLineWidths(sty.Tool.ParamMain.Width(width).Render(output), width)
	}
	return sty.Tool.ParamMain.Render(output)
}

func ToolHeader(sty *styles.Styles, status ToolStatus, name string, width int, opts *ToolHeaderOptions, params ...string) string {
	prefix := ToolHeaderPrefix(sty, status, name, opts)
	paramsStr := ToolParamList(sty, params, width-lipgloss.Width(prefix), opts)
	if first, rest, found := strings.Cut(paramsStr, "\n"); found {
		return prefix + first + "\n" + indentToolHeaderLines(rest, lipgloss.Width(prefix))
	}
	return prefix + paramsStr
}

func ToolHeaderPrefix(sty *styles.Styles, status ToolStatus, name string, opts *ToolHeaderOptions) string {
	nameStyle := sty.Tool.NameNormal
	if opts != nil && opts.Compact {
		nameStyle = sty.Tool.NameNested
	}
	return fmt.Sprintf("%s %s ", ToolIcon(sty, status), nameStyle.Render(name))
}

func capToolHeaderLineWidths(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			lines[i] = ansi.Truncate(line, width, "")
		}
	}
	return strings.Join(lines, "\n")
}

func indentToolHeaderLines(s string, n int) string {
	indent := strings.Repeat(" ", n)
	return indent + strings.ReplaceAll(s, "\n", "\n"+indent)
}
