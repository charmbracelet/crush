package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func makeToolCall(name string, params any) fantasy.ToolCall {
	input, _ := json.Marshal(params)
	return fantasy.ToolCall{
		Name:  name,
		Input: string(input),
	}
}

func TestChartTool_LineChart(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"type":   "line",
		"data":   [][]float64{{1, 4, 9, 16, 25, 36, 49}},
		"title":  "Squares",
		"width":  40,
		"height": 10,
	}))
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Squares")
	require.Contains(t, resp.Content, "7 points")

	var meta ChartResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "line", meta.Type)
	require.Equal(t, "Squares", meta.Title)
	require.Len(t, meta.Data, 1)
	require.Len(t, meta.Data[0], 7)
	require.Equal(t, 10, meta.Height)
}

func TestChartTool_MultiSeriesLineChart(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"type": "line",
		"data": [][]float64{
			{1, 4, 9, 16, 25},
			{25, 16, 9, 4, 1},
		},
	}))
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "2 series")

	var meta ChartResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Len(t, meta.Data, 2)
}

func TestChartTool_BarChart(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"type":   "bar",
		"data":   [][]float64{{14, 36, 11}},
		"labels": []string{"Sausage", "Pepperoni", "Mushrooms"},
		"title":  "Pizza Toppings",
	}))
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Pizza Toppings")

	var meta ChartResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "bar", meta.Type)
	require.Equal(t, []string{"Sausage", "Pepperoni", "Mushrooms"}, meta.Labels)
}

func TestChartTool_EmptyData(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"type": "line",
		"data": [][]float64{},
	}))
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "data is required")
}

func TestChartTool_UnsupportedType(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"type": "pie",
		"data": [][]float64{{1, 2, 3}},
	}))
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "unsupported chart type")
}

func TestChartTool_DefaultType(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"data": [][]float64{{1, 2, 3, 4, 5}},
	}))
	require.NoError(t, err)
	require.False(t, resp.IsError)

	var meta ChartResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "line", meta.Type)
}

func TestChartTool_DefaultHeight(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	resp, err := tool.Run(context.Background(), makeToolCall(ChartToolName, map[string]any{
		"data": [][]float64{{1, 2, 3}},
	}))
	require.NoError(t, err)
	require.False(t, resp.IsError)

	var meta ChartResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, 15, meta.Height)
}

func TestChartTool_ToolInfo(t *testing.T) {
	t.Parallel()

	tool := NewChartTool()
	info := tool.Info()
	require.Equal(t, ChartToolName, info.Name)
	require.NotEmpty(t, info.Description)
}
