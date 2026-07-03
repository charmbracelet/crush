package chat

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// subCellsPerRow is the vertical sub-resolution of a terminal row when
// rendering stacked bars with fractional block characters (▁▂▃▄▅▆▇█).
const subCellsPerRow = 8

func renderBarChart(sty *styles.Styles, meta tools.ChartResponseMetadata, width int) string {
	if len(meta.Data) == 0 || len(meta.Data[0]) == 0 {
		return ""
	}

	if len(meta.Data) > 1 {
		return renderStackedBarChart(sty, meta, width)
	}
	return renderSimpleBarChart(sty, meta, width)
}

// renderSimpleBarChart draws a single-series bar chart using ntcharts'
// barchart, adding a manual Y-axis scale (ntcharts draws none).
func renderSimpleBarChart(sty *styles.Styles, meta tools.ChartResponseMetadata, width int) string {
	values := meta.Data[0]
	labels := barLabels(meta.Labels, len(values))

	ss := seriesStyles(sty)
	axisStyle := sty.Messages.AssistantInfoIcon   // subtle
	labelStyle := sty.Messages.AssistantInfoModel // muted

	barHeight := max(len(values)+4, 12)

	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	bc := barchart.New(width, barHeight, barchart.WithMaxValue(maxVal))
	bc.AxisStyle = axisStyle
	bc.LabelStyle = labelStyle
	for i, v := range values {
		bc.Push(barchart.BarData{
			Label: labels[i],
			Values: []barchart.BarValue{
				{Name: "value", Value: v, Style: ss[0]},
			},
		})
	}
	bc.Draw()

	chartLines := strings.Split(bc.View(), "\n")
	yLabels, labelW := yAxisLabels(maxVal, len(chartLines)-1)

	var b strings.Builder
	for i, line := range chartLines {
		label := ""
		if i < len(yLabels) {
			label = yLabels[i]
		}
		b.WriteString(axisRowPrefix(label, labelW, labelStyle, axisStyle))
		b.WriteString(line)
		if i < len(chartLines)-1 {
			b.WriteString("\n")
		}
	}

	return wrapWithAxisTitles(sty, meta, b.String(), width)
}

// renderStackedBarChart draws stacked bars using fractional block
// characters (▁▂▃▄▅▆▇█) for 8x vertical sub-resolution per terminal cell.
// At a layer boundary within a cell, the lower block fills the fraction
// belonging to the lower layer (foreground) while the next layer shows
// through as the background. This keeps layer edges crisp instead of
// snapping to whole cells.
func renderStackedBarChart(sty *styles.Styles, meta tools.ChartResponseMetadata, width int) string {
	ss := seriesStyles(sty)
	axisStyle := sty.Messages.AssistantInfoIcon   // subtle
	labelStyle := sty.Messages.AssistantInfoModel // muted

	numBars := len(meta.Data[0])
	numLayers := len(meta.Data)
	labels := barLabels(meta.Labels, numBars)

	// Max stacked total across all bars sets the Y-axis scale.
	maxVal := 0.0
	for bar := range numBars {
		sum := 0.0
		for l := range numLayers {
			if bar < len(meta.Data[l]) {
				sum += meta.Data[l][bar]
			}
		}
		maxVal = max(maxVal, sum)
	}
	if maxVal == 0 {
		maxVal = 1
	}

	graphH := max(numBars+2, 10)
	totalSubs := graphH * subCellsPerRow
	subsPerVal := float64(totalSubs) / maxVal

	yLabels, labelW := yAxisLabels(maxVal, graphH)
	barW := max((width-labelW-1)/numBars, 3)
	const gap = 1

	// For each bar, resolve the color of every vertical sub-unit
	// (index 0 = bottom) by walking its stacked layers.
	barSubs := make([][]color.Color, numBars)
	for bar := range numBars {
		subs := make([]color.Color, totalSubs)
		cumulative := 0.0
		for l := range numLayers {
			val := 0.0
			if bar < len(meta.Data[l]) {
				val = meta.Data[l][bar]
			}
			start := int(cumulative * subsPerVal)
			end := int((cumulative + val) * subsPerVal)
			for s := max(start, 0); s < end && s < totalSubs; s++ {
				subs[s] = ss[l%len(ss)].GetForeground()
			}
			cumulative += val
		}
		barSubs[bar] = subs
	}

	var b strings.Builder
	for row := range graphH {
		label := ""
		if row < len(yLabels) {
			label = yLabels[row]
		}
		b.WriteString(axisRowPrefix(label, labelW, labelStyle, axisStyle))

		// Top-most sub-unit of this row. Rows render top to bottom.
		topSub := totalSubs - 1 - row*subCellsPerRow
		for bar := range numBars {
			b.WriteString(stackedCell(barSubs[bar], topSub, barW))
			b.WriteString(strings.Repeat(" ", gap))
		}
		b.WriteString("\n")
	}

	// X-axis line and labels.
	b.WriteString(axisRowPrefix("", labelW, labelStyle, axisStyle))
	b.WriteString(strings.Repeat("─", numBars*(barW+gap)))
	b.WriteString("\n")
	b.WriteString(axisRowPrefix("", labelW, labelStyle, axisStyle))
	for i := range numBars {
		b.WriteString(centerStr(labels[i], barW))
		b.WriteString(strings.Repeat(" ", gap))
	}
	b.WriteString("\n")

	if legend := stackedLegend(meta.SeriesLabels, numLayers, ss, labelStyle); legend != "" {
		b.WriteString(legend)
		b.WriteString("\n")
	}

	return wrapWithAxisTitles(sty, meta, strings.TrimRight(b.String(), "\n"), width)
}

// stackedCell renders one bar's cell for a row. The cell spans 8 vertical
// sub-units ending at topSub. It fills from the bottom with the lower
// layer's color (foreground) up to the first layer boundary, showing the
// next layer through the background.
func stackedCell(subs []color.Color, topSub, barW int) string {
	bottomSub := topSub - (subCellsPerRow - 1)

	// Color of the bottom-most filled sub-unit in this cell.
	var bottom color.Color
	for offset := range subCellsPerRow {
		if idx := bottomSub + offset; idx >= 0 && idx < len(subs) && subs[idx] != nil {
			bottom = subs[idx]
			break
		}
	}
	if bottom == nil {
		return strings.Repeat(" ", barW)
	}

	// Count contiguous sub-units from the bottom sharing that color,
	// skipping any leading empty sub-units.
	filled, started := 0, false
	var above color.Color
	for offset := range subCellsPerRow {
		idx := bottomSub + offset
		var c color.Color
		if idx >= 0 && idx < len(subs) {
			c = subs[idx]
		}
		if !started {
			if c == nil {
				continue
			}
			started = true
		}
		if c == bottom {
			filled++
		} else {
			above = c
			break
		}
	}

	if filled >= subCellsPerRow {
		return lipgloss.NewStyle().Foreground(bottom).Render(strings.Repeat("█", barW))
	}
	style := lipgloss.NewStyle().Foreground(bottom)
	if above != nil {
		style = style.Background(above)
	}
	// lowerBlocks[n] fills n/8ths from the bottom.
	lowerBlocks := [...]rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	return style.Render(strings.Repeat(string(lowerBlocks[filled]), barW))
}

// stackedLegend renders a colored swatch legend for the layer names.
func stackedLegend(names []string, numLayers int, ss []lipgloss.Style, labelStyle lipgloss.Style) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	for l, name := range names {
		if l >= numLayers {
			break
		}
		b.WriteString(ss[l%len(ss)].Render("█"))
		b.WriteString(" ")
		b.WriteString(labelStyle.Render(name))
		if l < len(names)-1 && l < numLayers-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}

// barLabels returns labels for n bars, generating numeric fallbacks for
// any missing entries.
func barLabels(labels []string, n int) []string {
	if len(labels) >= n {
		return labels
	}
	out := make([]string, n)
	for i := range n {
		if i < len(labels) {
			out[i] = labels[i]
		} else {
			out[i] = fmt.Sprintf("%d", i)
		}
	}
	return out
}
