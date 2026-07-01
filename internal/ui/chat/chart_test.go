package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestUseBraille(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		data       [][]float64
		chartWidth int
		want       bool
	}{
		{
			name:       "empty defaults to braille",
			data:       nil,
			chartWidth: 40,
			want:       true,
		},
		{
			name:       "multi-series always braille",
			data:       [][]float64{{1, 2, 3}, {3, 2, 1}},
			chartWidth: 40,
			want:       true,
		},
		{
			name:       "too few points use arc",
			data:       [][]float64{{1, 5}},
			chartWidth: 40,
			want:       false,
		},
		{
			name:       "dense data uses braille",
			data:       [][]float64{make([]float64, 100)},
			chartWidth: 40,
			want:       true,
		},
		{
			name:       "stepped data uses arc lines",
			data:       [][]float64{{1, 1, 1, 3, 3, 3, 2, 2, 5, 5}},
			chartWidth: 40,
			want:       false,
		},
		{
			name:       "continuous data uses braille",
			data:       [][]float64{{1.1, 2.3, 3.7, 4.2, 5.9, 6.1, 7.8, 8.4}},
			chartWidth: 40,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := useBraille(tt.data, tt.chartWidth)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNiceAxisBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		minVal, maxVal float64
		wantLo, wantHi float64
		wantStep       float64
	}{
		{"10 to 50 extends to zero", 10, 50, 0, 50, 10},
		{"0 to 100", 0, 100, 0, 100, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lo, hi, step := niceAxisBounds(tt.minVal, tt.maxVal)
			require.InDelta(t, tt.wantLo, lo, 0.01, "lo")
			require.InDelta(t, tt.wantHi, hi, 0.01, "hi")
			require.InDelta(t, tt.wantStep, step, 0.01, "step")
		})
	}
}

func TestAlignedHeight(t *testing.T) {
	t.Parallel()

	// 0..50 step 10 = 5 intervals. graphHeight must be a multiple of 5
	// so labeled rows land exactly on step boundaries.
	ch, yStep := alignedHeight(0, 50, 10, 15)
	graphHeight := ch - 2
	require.Equal(t, 0, graphHeight%5, "graph height must be a multiple of interval count")
	require.GreaterOrEqual(t, yStep, 1)
}

func TestRenderLineChart_StepInterpolation(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	meta := tools.ChartResponseMetadata{
		Type:          "line",
		Data:          [][]float64{{10, 10, 25, 25, 40, 15, 50}},
		Interpolation: "step",
		Height:        12,
	}
	out := renderLineChart(&sty, meta, 60)
	require.NotEmpty(t, out)
	// Step charts use box-drawing lines; expect vertical and horizontal
	// segment runes rather than a smear of braille dots.
	require.Contains(t, out, "│")
	require.Contains(t, out, "─")
}

func TestFormatAxisValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input float64
		want  string
	}{
		{0, "0"},
		{42, "42"},
		{100, "100"},
		{1500, "1.5K"},
		{2500000, "2.5M"},
		{3.5, "3.5"},
	}

	for _, tt := range tests {
		got := formatAxisValue(tt.input)
		require.Equal(t, tt.want, got, "formatAxisValue(%v)", tt.input)
	}
}
