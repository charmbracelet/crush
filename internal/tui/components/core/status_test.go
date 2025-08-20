package core_test

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/x/exp/golden"
)

func TestStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		opts  core.StatusOpts
		width int
	}{
		{
			name: "Default",
			opts: core.StatusOpts{
				Title:       "Status",
				Description: "Everything is working fine",
			},
			width: 80,
		},
		{
			name: "WithCustomIcon",
			opts: core.StatusOpts{
				Icon:        "✓",
				Title:       "Success",
				Description: "Operation completed successfully",
			},
			width: 80,
		},
		{
			name: "NoIcon",
			opts: core.StatusOpts{
				Title:       "Info",
				Description: "This status has no icon",
			},
			width: 80,
		},
		{
			name: "WithColors",
			opts: core.StatusOpts{
				Icon:             "⚠",
				Title:            "Warning",
				TitleColor:       color.RGBA{255, 255, 0, 255}, // Yellow
				Description:      "This is a warning message",
				DescriptionColor: color.RGBA{255, 0, 0, 255}, // Red
			},
			width: 80,
		},
		{
			name: "WithExtraContent",
			opts: core.StatusOpts{
				Title:        "Build",
				Description:  "Building project",
				ExtraContent: "[2/5]",
			},
			width: 80,
		},
		{
			name: "LongDescription",
			opts: core.StatusOpts{
				Title:       "Processing",
				Description: "This is a very long description that should be truncated when the width is too small to display it completely without wrapping",
			},
			width: 60,
		},
		{
			name: "NarrowWidth",
			opts: core.StatusOpts{
				Icon:        "●",
				Title:       "Status",
				Description: "Short message",
			},
			width: 30,
		},
		{
			name: "VeryNarrowWidth",
			opts: core.StatusOpts{
				Icon:        "●",
				Title:       "Test",
				Description: "This will be truncated",
			},
			width: 20,
		},
		{
			name: "EmptyDescription",
			opts: core.StatusOpts{
				Icon:  "●",
				Title: "Title Only",
			},
			width: 80,
		},
		{
			name: "AllFieldsWithExtraContent",
			opts: core.StatusOpts{
				Icon:             "🚀",
				Title:            "Deployment",
				TitleColor:       color.RGBA{0, 0, 255, 255}, // Blue
				Description:      "Deploying to production environment",
				DescriptionColor: color.RGBA{128, 128, 128, 255}, // Gray
				ExtraContent:     "v1.2.3",
			},
			width: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output := core.Status(tt.opts, tt.width)
			golden.RequireEqual(t, []byte(output))
		})
	}
}

func TestStatusTruncation(t *testing.T) {
	t.Parallel()

	opts := core.StatusOpts{
		Icon:         "●",
		Title:        "Very Long Title",
		Description:  "This is an extremely long description that definitely needs to be truncated",
		ExtraContent: "[extra]",
	}

	// Test different widths to ensure truncation works correctly
	widths := []int{20, 30, 40, 50, 60}

	for _, width := range widths {
		t.Run(fmt.Sprintf("Width%d", width), func(t *testing.T) {
			t.Parallel()

			output := core.Status(opts, width)
			golden.RequireEqual(t, []byte(output))
		})
	}
}
