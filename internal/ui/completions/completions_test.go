package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestUpdateSize(t *testing.T) {
	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())

	// Test with 1 item
	files := []FileCompletionValue{
		{Path: "long_filename_that_should_determine_width"},
	}
	c.SetItems(files, nil)

	// Expected width: length of string + 2 (padding/borders if any, strictly +2 in updateSize)
	// "long_filename_that_should_determine_width" is 41 chars.
	// width should be 43.
	// But clamps apply: minWidth=10, maxWidth=100.
	expectedWidth := 43
	assert.Equal(t, expectedWidth, c.width, "Width should be calculated correctly for 1 item")

	// Test with 0 items
	c.SetItems(nil, nil)
	// Width should be minWidth? Or whatever it was before?
	// The loop doesn't run, width=0. Clamped to minWidth (10).
	assert.Equal(t, minWidth, c.width, "Width should be minWidth for 0 items")

	// Test with multiple items
	files = []FileCompletionValue{
		{Path: "short"},
		{Path: "longer_path"},
	}
	c.SetItems(files, nil)
	// "longer_path" is 11 chars. +2 = 13.
	assert.Equal(t, 13, c.width, "Width should be calculated correctly for multiple items")
}
