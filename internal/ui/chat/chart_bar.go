package chat

import (
	"fmt"
	"strings"

	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

func renderBarChart(sty *styles.Styles, meta tools.ChartResponseMetadata, width int) string {
	if len(meta.Data) == 0 || len(meta.Data[0]) == 0 {
		return ""
	}

	values := meta.Data[0]
	labels := barLabels(meta.Labels, len(values))

	ss := seriesStyles(sty)
	axisStyle := sty.Messages.AssistantInfoIcon   // subtle
	labelStyle := sty.Messages.AssistantInfoModel // muted

	// Make bars tall enough to be visually useful.
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

	// ntcharts barchart doesn't draw a Y-axis scale, so we add one
	// manually on the left side.
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
