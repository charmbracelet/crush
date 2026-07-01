package chat

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// ChartToolMessageItem renders chart tool calls with responsive ASCII charts.
type ChartToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*ChartToolMessageItem)(nil)

// NewChartToolMessageItem creates a new [ChartToolMessageItem].
func NewChartToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &ChartToolRenderContext{}, canceled)
}

// HandleKeyEvent overrides the base copy behavior. Charts are visual;
// copying the raw text output is not useful.
func (c *ChartToolMessageItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	return false, nil
}

// ChartToolRenderContext renders chart tool messages using ntcharts.
type ChartToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (c *ChartToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	name := "Chart"

	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	var params tools.ChartParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	headerText := buildChartHeaderText(params)
	header := toolHeader(sty, opts.Status, name, cappedMessageWidth(width), opts, headerText)

	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedMessageWidth(width)); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() {
		return header
	}

	if opts.Result.Metadata != "" {
		var meta tools.ChartResponseMetadata
		if err := json.Unmarshal([]byte(opts.Result.Metadata), &meta); err == nil {
			body := renderChartBody(sty, meta, width)
			if body != "" {
				padded := indentLines(body, toolBodyLeftPaddingTotal)
				return joinToolParts(header, padded)
			}
		}
	}

	if opts.Result.Content != "" {
		bodyWidth := cappedMessageWidth(width) - toolBodyLeftPaddingTotal
		body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth}, opts.ExpandedContent)
		return joinToolParts(header, body)
	}

	return header
}

func buildChartHeaderText(params tools.ChartParams) string {
	title := params.Title
	if title != "" {
		return title
	}

	chartType := params.Type
	if chartType == "" {
		chartType = "line"
	}

	numSeries := len(params.Data)
	if numSeries <= 1 {
		return chartType + " chart"
	}
	return fmt.Sprintf("%s chart (%d series)", chartType, numSeries)
}

func renderChartBody(sty *styles.Styles, meta tools.ChartResponseMetadata, availableWidth int) string {
	renderWidth := max(availableWidth-MessageLeftPaddingTotal-toolBodyLeftPaddingTotal, 20)

	switch meta.Type {
	case "line":
		return renderLineChart(sty, meta, renderWidth)
	case "bar":
		return renderBarChart(sty, meta, renderWidth)
	default:
		return ""
	}
}

// seriesStyles returns lipgloss styles for chart series using the theme's
// ANSI palette, which maps to Charmtone colors:
// [4]=Charple, [6]=Malibu, [2]=Guac, [3]=Mustard,
// [5]=Dolly, [1]=Coral, [14]=Sardine, [11]=Zest.
func seriesStyles(sty *styles.Styles) []lipgloss.Style {
	indices := []int{4, 6, 2, 3, 5, 1, 14, 11}
	out := make([]lipgloss.Style, len(indices))
	for i, idx := range indices {
		out[i] = lipgloss.NewStyle().Foreground(sty.ANSI[idx])
	}
	return out
}

// formatAxisValue formats numeric values for chart axes with K/M
// suffixes and smart decimal handling.
func formatAxisValue(v float64) string {
	if v == 0 {
		return "0"
	}
	if v >= 1e6 {
		return fmt.Sprintf("%.1fM", v/1e6)
	}
	if v >= 1e3 {
		return fmt.Sprintf("%.1fK", v/1e3)
	}
	if v == float64(int(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

// niceAxisBounds rounds a data range to visually clean bounds so axis
// labels land on round numbers. It returns the low/high bounds and the
// step size between labels.
func niceAxisBounds(minVal, maxVal float64) (lo, hi, step float64) {
	if minVal == maxVal {
		// Flat data: pad around the single value.
		if minVal == 0 {
			return 0, 1, 1
		}
		pad := math.Abs(minVal) * 0.1
		return minVal - pad, maxVal + pad, pad
	}

	span := maxVal - minVal
	step = niceStep(span / 5) // aim for ~5 labeled intervals

	lo = math.Floor(minVal/step) * step
	hi = math.Ceil(maxVal/step) * step

	// If the data doesn't touch zero but is close, extend to zero for
	// a more honest baseline on positive data.
	if lo > 0 && lo <= span*0.5 {
		lo = 0
	}
	return lo, hi, step
}

// alignedHeight returns a canvas height and yStep such that ntcharts
// labels the Y axis exactly on the nice step boundaries. ntcharts labels
// every yStep rows with increment = range/graphHeight, so we make
// graphHeight a multiple of the interval count.
func alignedHeight(lo, hi, step float64, desiredHeight int) (canvasHeight, yStep int) {
	intervals := int(math.Round((hi - lo) / step))
	if intervals < 1 {
		return desiredHeight, 2
	}

	// graphHeight (rows excluding the 2 used by the X axis) must be a
	// multiple of intervals so each labeled row lands on a step.
	desiredGraph := max(desiredHeight-2, intervals)
	mult := max(int(math.Round(float64(desiredGraph)/float64(intervals))), 1)
	graphHeight := intervals * mult

	return graphHeight + 2, mult
}

// niceStep returns a "nice" round step size near the given raw value.
func niceStep(raw float64) float64 {
	if raw <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(raw))
	pow := math.Pow(10, exp)
	frac := raw / pow // in [1, 10)

	var nice float64
	switch {
	case frac <= 1:
		nice = 1
	case frac <= 2:
		nice = 2
	case frac <= 2.5:
		nice = 2.5
	case frac <= 5:
		nice = 5
	default:
		nice = 10
	}
	return nice * pow
}

// yAxisLabels generates right-aligned numeric labels spanning [0, maxVal]
// from top (maxVal) down to bottom (0) across the given number of rows,
// and returns the labels plus the width of the widest one.
func yAxisLabels(maxVal float64, rows int) (labels []string, width int) {
	if rows < 1 {
		return nil, 0
	}
	labels = make([]string, rows)
	for i := range rows {
		v := maxVal - float64(i)/float64(max(rows-1, 1))*maxVal
		labels[i] = formatAxisValue(v)
		if len(labels[i]) > width {
			width = len(labels[i])
		}
	}
	return labels, width
}

// axisRowPrefix renders the right-aligned Y-axis label for a row plus the
// vertical separator. Pass an empty label for unlabeled rows (axis line,
// x-axis labels).
func axisRowPrefix(label string, width int, labelStyle, sepStyle lipgloss.Style) string {
	padding := max(width-len(label), 0)
	return labelStyle.Render(strings.Repeat(" ", padding)+label) + sepStyle.Render("│")
}

// wrapWithAxisTitles prepends the y-axis title above the chart body and
// appends a centered x-axis title below it. Both are optional.
func wrapWithAxisTitles(sty *styles.Styles, meta tools.ChartResponseMetadata, body string, width int) string {
	titleStyle := sty.Messages.AssistantInfoModel // muted

	var b strings.Builder
	if meta.YLabel != "" {
		b.WriteString(titleStyle.Render(meta.YLabel))
		b.WriteString("\n\n")
	}
	b.WriteString(body)
	if meta.XLabel != "" {
		b.WriteString("\n")
		padding := max(width/2-len(meta.XLabel)/2, 0)
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(titleStyle.Render(meta.XLabel))
	}
	return b.String()
}

// indentLines adds left padding to each line without using lipgloss
// (which would override ANSI color codes).
func indentLines(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if ln != "" {
			lines[i] = prefix + ln
		}
	}
	return strings.Join(lines, "\n")
}
