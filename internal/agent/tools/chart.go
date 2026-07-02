package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

const ChartToolName = "chart"

//go:embed chart.md.tpl
var chartDescription string

// ChartParams defines the input for the chart tool.
type ChartParams struct {
	Type          string      `json:"type" description:"Chart type: 'line' or 'bar'"`
	Data          [][]float64 `json:"data" description:"Data series. For line charts, each inner slice is a series. For bar charts, use a single series."`
	Labels        []string    `json:"labels,omitempty" description:"Labels for data points (x-axis for line charts, bar labels for bar charts)"`
	Title         string      `json:"title,omitempty" description:"Chart title"`
	XLabel        string      `json:"x_label,omitempty" description:"Label for the x-axis (e.g. 'Time', 'Category')"`
	YLabel        string      `json:"y_label,omitempty" description:"Label for the y-axis (e.g. 'Revenue ($)', 'Count')"`
	Interpolation string      `json:"interpolation,omitempty" description:"Line interpolation for line charts: 'linear' (default, straight lines between points) or 'step' (flat segments with vertical jumps, for discrete/quantized data)"`
	Width         int         `json:"width,omitempty" description:"Preferred chart width in columns (renderer may adjust for terminal size)"`
	Height        int         `json:"height,omitempty" description:"Chart height in rows for line charts (default: 15)"`
}

// ChartResponseMetadata carries structured chart data to the UI renderer.
type ChartResponseMetadata struct {
	Type          string      `json:"type"`
	Data          [][]float64 `json:"data"`
	Labels        []string    `json:"labels,omitempty"`
	Title         string      `json:"title,omitempty"`
	XLabel        string      `json:"x_label,omitempty"`
	YLabel        string      `json:"y_label,omitempty"`
	Interpolation string      `json:"interpolation,omitempty"`
	Height        int         `json:"height,omitempty"`
}

// NewChartTool creates a new chart tool.
func NewChartTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ChartToolName,
		chartDescription,
		func(ctx context.Context, params ChartParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.Data) == 0 {
				return fantasy.NewTextErrorResponse("data is required: provide at least one series"), nil
			}

			chartType := params.Type
			if chartType == "" {
				chartType = "line"
			}

			switch chartType {
			case "line", "bar", "heatmap":
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf("unsupported chart type %q: use 'line', 'bar', or 'heatmap'", params.Type)), nil
			}

			height := params.Height
			if height <= 0 {
				height = 15
			}

			summary := buildChartSummary(chartType, params)

			meta := ChartResponseMetadata{
				Type:          chartType,
				Data:          params.Data,
				Labels:        params.Labels,
				Title:         params.Title,
				XLabel:        params.XLabel,
				YLabel:        params.YLabel,
				Interpolation: params.Interpolation,
				Height:        height,
			}
			metaJSON, err := json.Marshal(meta)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to marshal chart metadata: %w", err)
			}

			resp := fantasy.NewTextResponse(summary)
			resp.Metadata = string(metaJSON)
			return resp, nil
		},
	)
}

func buildChartSummary(chartType string, params ChartParams) string {
	title := params.Title
	if title == "" {
		title = chartType + " chart"
	}

	if chartType == "heatmap" {
		rows := len(params.Data)
		cols := 0
		if rows > 0 {
			cols = len(params.Data[0])
		}
		return fmt.Sprintf("%s (%dx%d grid)", title, rows, cols)
	}

	numSeries := len(params.Data)
	numPoints := 0
	if numSeries > 0 {
		numPoints = len(params.Data[0])
	}

	if numSeries == 1 {
		return fmt.Sprintf("%s (%d points)", title, numPoints)
	}
	return fmt.Sprintf("%s (%d series, %d points)", title, numSeries, numPoints)
}
