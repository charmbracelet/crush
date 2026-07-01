package chat

import (
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// useBraille decides between braille (smooth, high-resolution) and arc
// line (clean box-drawing steps) rendering.
//
// Box-drawing arc lines can only draw horizontal, vertical, and diagonal
// segments, so they render stepped/quantized data crisply but turn smooth
// curves into staircases. Braille gives 2x horizontal and 4x vertical
// sub-cell resolution, so it draws smooth curves well but smears clean
// steps into fuzzy dots.
//
// We route based on two signals:
//   - density: points relative to chart width. Dense data needs braille's
//     sub-cell resolution to avoid losing detail when downsampling.
//   - distinctRatio: unique Y values relative to total points. Low means
//     the data is quantized/stepped (few levels, flat runs); high means
//     continuous variation that reads better as a smooth line.
//
// Multiple overlapping series always use braille for cleaner separation.
func useBraille(data [][]float64, chartWidth int) bool {
	if len(data) == 0 {
		return true
	}
	if len(data) > 1 {
		return true
	}

	series := data[0]
	if len(series) < 3 {
		return false // too few points; arc lines connect them cleanly
	}

	// Dense data needs braille's extra horizontal resolution.
	if len(series) > chartWidth {
		return true
	}

	// Count distinct Y values to gauge how stepped the data is.
	seen := make(map[float64]struct{}, len(series))
	for _, v := range series {
		seen[v] = struct{}{}
	}
	distinctRatio := float64(len(seen)) / float64(len(series))

	// High distinct ratio = continuous variation → braille.
	// Low distinct ratio = quantized/stepped → arc lines.
	return distinctRatio > 0.5
}

func renderLineChart(sty *styles.Styles, meta tools.ChartResponseMetadata, width int) string {
	if len(meta.Data) == 0 || len(meta.Data[0]) == 0 {
		return ""
	}

	height := meta.Height
	if height <= 0 {
		height = 15
	}

	ss := seriesStyles(sty)
	axisStyle := sty.Messages.AssistantInfoIcon   // subtle
	labelStyle := sty.Messages.AssistantInfoModel // muted

	// Find global min/max across all series.
	minVal, maxVal := meta.Data[0][0], meta.Data[0][0]
	for _, series := range meta.Data {
		for _, v := range series {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}

	// Snap the Y range to nice round bounds so axis labels land on clean
	// values (e.g. 0, 10, 20), and align the chart height + yStep so each
	// labeled row falls exactly on a step boundary.
	minY, maxY, step := niceAxisBounds(minVal, maxVal)
	canvasHeight, yStep := alignedHeight(minY, maxY, step, height)

	// Step interpolation looks crispest with box-drawing arc lines, which
	// render true horizontal/vertical segments. Otherwise pick based on
	// the data shape.
	braille := useBraille(meta.Data, width)
	if meta.Interpolation == "step" {
		braille = false
	}

	chartView := renderLineChartWithMode(
		meta, width, canvasHeight, yStep, ss, axisStyle, labelStyle, minY, maxY,
		braille,
	)

	return wrapWithAxisTitles(sty, meta, chartView, width)
}

// renderLineChartWithMode draws a line chart using either braille (smooth)
// or arc box-drawing lines (stepped), depending on the braille flag.
func renderLineChartWithMode(
	meta tools.ChartResponseMetadata,
	width, height, yStep int,
	ss []lipgloss.Style,
	axisStyle, labelStyle lipgloss.Style,
	minY, maxY float64,
	braille bool,
) string {
	maxX := float64(len(meta.Data[0]) - 1)
	if maxX == 0 {
		maxX = 1
	}

	lc := linechart.New(width, height, 0, maxX, minY, maxY)
	lc.AxisStyle = axisStyle
	lc.LabelStyle = labelStyle
	lc.YLabelFormatter = func(_ int, v float64) string { return formatAxisValue(v) }
	lc.SetYStep(yStep)
	// Recompute the graph layout so the Y-label column is sized for our
	// formatter's output rather than the default one used at construction.
	lc.UpdateGraphSizes()
	lc.DrawXYAxisAndLabel()

	stepMode := meta.Interpolation == "step"

	drawSegment := func(a, b canvas.Float64Point, style lipgloss.Style) {
		if braille {
			lc.DrawBrailleLineWithStyle(a, b, style)
		} else {
			lc.DrawLineWithStyle(a, b, runes.ArcLineStyle, style)
		}
	}

	for i, series := range meta.Data {
		points := make([]canvas.Float64Point, len(series))
		for j, v := range series {
			points[j] = canvas.Float64Point{X: float64(j), Y: v}
		}

		style := ss[i%len(ss)]
		for j := 1; j < len(points); j++ {
			if stepMode {
				// Step-after: hold the previous value across to the next
				// x position (horizontal), then jump to the new value
				// (vertical). This renders discrete data as clean stairs.
				corner := canvas.Float64Point{X: points[j].X, Y: points[j-1].Y}
				drawSegment(points[j-1], corner, style)
				drawSegment(corner, points[j], style)
			} else {
				drawSegment(points[j-1], points[j], style)
			}
		}
	}

	return lc.View()
}
