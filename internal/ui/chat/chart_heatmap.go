package chat

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

func renderHeatmap(sty *styles.Styles, meta tools.ChartResponseMetadata, width int) string {
	if len(meta.Data) == 0 || len(meta.Data[0]) == 0 {
		return ""
	}

	rows := len(meta.Data)
	cols := len(meta.Data[0])
	labels := meta.Labels

	minVal, maxVal := meta.Data[0][0], meta.Data[0][0]
	for _, row := range meta.Data {
		for _, v := range row {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if minVal == maxVal {
		maxVal = minVal + 1
	}

	colorScale := buildHeatColorScale(sty)
	labelStyle := sty.Messages.AssistantInfoModel // muted
	axisStyle := sty.Messages.AssistantInfoIcon   // subtle

	// Label column width, with a floor for unlabeled heatmaps.
	labelW := 3
	for _, l := range labels {
		if len(l) > labelW {
			labelW = len(l)
		}
	}

	// Cell dimensions to fill available width.
	cellW := max((width-labelW-2)/cols, 3)
	cellH := 3 // fixed height per cell for readability

	var b strings.Builder

	if meta.YLabel != "" {
		b.WriteString(labelStyle.Render(meta.YLabel))
		b.WriteString("\n\n")
	}

	// Render rows top-to-bottom (data row 0 at top).
	for r := range rows {
		rowLabel := ""
		if r < len(labels) {
			rowLabel = labels[r]
		}
		for dy := range cellH {
			// Only the middle line of the cell block carries the label.
			label := ""
			if dy == cellH/2 {
				label = rowLabel
			}
			b.WriteString(axisRowPrefix(label, labelW, labelStyle, axisStyle))
			for c := range cols {
				clr := mapValueToColor(meta.Data[r][c], minVal, maxVal, colorScale)
				b.WriteString(lipgloss.NewStyle().Background(clr).Render(strings.Repeat(" ", cellW)))
			}
			b.WriteString("\n")
		}
	}

	// Column labels below the grid.
	if len(labels) > 0 {
		b.WriteString(axisRowPrefix("", labelW, labelStyle, axisStyle))
		for _, l := range labels {
			b.WriteString(centerStr(l, cellW))
		}
		b.WriteString("\n")
	}

	if meta.XLabel != "" {
		b.WriteString("\n")
		padding := max(labelW+1+width/2-len(meta.XLabel)/2, 0)
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(labelStyle.Render(meta.XLabel))
	}

	return strings.TrimRight(b.String(), "\n")
}

// mapValueToColor maps v within [minVal, maxVal] to a color from scale.
func mapValueToColor(v, minVal, maxVal float64, scale []color.Color) color.Color {
	if len(scale) == 0 {
		return nil
	}
	t := (v - minVal) / (maxVal - minVal)
	idx := int(t * float64(len(scale)-1))
	idx = max(0, min(idx, len(scale)-1))
	return scale[idx]
}

// buildHeatColorScale creates a gradient from Charple (dark/low values)
// to Dolly (bright/high values) using lipgloss.Blend1D, the same gradient
// system used for the Crush logo.
func buildHeatColorScale(sty *styles.Styles) []color.Color {
	return lipgloss.Blend1D(16,
		sty.ANSI[4], // Blue/Charple (primary, dark end)
		sty.ANSI[5], // Magenta/Dolly (secondary, bright end)
	)
}
