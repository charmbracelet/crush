package list

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour/v2/ansi"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestMarkdownItemBasic(t *testing.T) {
	markdown := "# Hello\n\nThis is a **test**."
	item := NewMarkdownItem("1", markdown)

	if item.ID() != "1" {
		t.Errorf("expected ID '1', got '%s'", item.ID())
	}

	// Test that height is calculated
	height := item.Height(80)
	if height < 1 {
		t.Errorf("expected height >= 1, got %d", height)
	}

	// Test drawing
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	item.Draw(&screen, area)

	// Should not panic and should render something
	rendered := screen.Render()
	if len(rendered) == 0 {
		t.Error("expected non-empty rendered output")
	}
}

func TestMarkdownItemCache(t *testing.T) {
	markdown := "# Test\n\nSome content."
	item := NewMarkdownItem("1", markdown)

	// First render at width 80 should populate cache
	height1 := item.Height(80)
	if len(item.cache) != 1 {
		t.Errorf("expected cache to have 1 entry after first render, got %d", len(item.cache))
	}

	// Second render at same width should reuse cache
	height2 := item.Height(80)
	if height1 != height2 {
		t.Errorf("expected consistent height, got %d then %d", height1, height2)
	}
	if len(item.cache) != 1 {
		t.Errorf("expected cache to still have 1 entry, got %d", len(item.cache))
	}

	// Render at different width should add to cache
	_ = item.Height(40)
	if len(item.cache) != 2 {
		t.Errorf("expected cache to have 2 entries after different width, got %d", len(item.cache))
	}
}

func TestMarkdownItemMaxCacheWidth(t *testing.T) {
	markdown := "# Test\n\nSome content."
	item := NewMarkdownItem("1", markdown).WithMaxWidth(50)

	// Render at width 40 (below limit) - should cache at width 40
	_ = item.Height(40)
	if len(item.cache) != 1 {
		t.Errorf("expected cache to have 1 entry for width 40, got %d", len(item.cache))
	}

	// Render at width 80 (above limit) - should cap to 50 and cache
	_ = item.Height(80)
	// Cache should have width 50 entry (capped from 80)
	if len(item.cache) != 2 {
		t.Errorf("expected cache to have 2 entries (40 and 50), got %d", len(item.cache))
	}
	if _, ok := item.cache[50]; !ok {
		t.Error("expected cache to have entry for width 50 (capped from 80)")
	}

	// Render at width 100 (also above limit) - should reuse cached width 50
	_ = item.Height(100)
	if len(item.cache) != 2 {
		t.Errorf("expected cache to still have 2 entries (reusing 50), got %d", len(item.cache))
	}
}

func TestMarkdownItemWithStyleConfig(t *testing.T) {
	markdown := "# Styled\n\nContent with **bold** text."

	// Create a custom style config
	styleConfig := ansi.StyleConfig{
		Document: ansi.StyleBlock{
			Margin: uintPtr(0),
		},
	}

	item := NewMarkdownItem("1", markdown).WithStyleConfig(styleConfig)

	// Render should use the custom style
	height := item.Height(80)
	if height < 1 {
		t.Errorf("expected height >= 1, got %d", height)
	}

	// Draw should work without panic
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	item.Draw(&screen, area)

	rendered := screen.Render()
	if len(rendered) == 0 {
		t.Error("expected non-empty rendered output with custom style")
	}
}

func TestMarkdownItemInList(t *testing.T) {
	items := []Item{
		NewMarkdownItem("1", "# First\n\nMarkdown item."),
		NewMarkdownItem("2", "# Second\n\nAnother item."),
		NewStringItem("3", "Regular string item"),
	}

	l := New(items...)
	l.SetSize(80, 20)

	// Should render without error
	output := l.Render()
	if len(output) == 0 {
		t.Error("expected non-empty output from list with markdown items")
	}

	// Should contain content from markdown items
	if !strings.Contains(output, "First") {
		t.Error("expected output to contain 'First'")
	}
	if !strings.Contains(output, "Second") {
		t.Error("expected output to contain 'Second'")
	}
	if !strings.Contains(output, "Regular string item") {
		t.Error("expected output to contain 'Regular string item'")
	}
}

func TestMarkdownItemHeightWithWidth(t *testing.T) {
	// Test that widths are capped to maxWidth
	markdown := "This is a paragraph with some text."

	item := NewMarkdownItem("1", markdown).WithMaxWidth(50)

	// At width 30 (below limit), should cache and render at width 30
	height30 := item.Height(30)
	if height30 < 1 {
		t.Errorf("expected height >= 1, got %d", height30)
	}

	// At width 100 (above maxWidth), should cap to 50 and cache
	height100 := item.Height(100)
	if height100 < 1 {
		t.Errorf("expected height >= 1, got %d", height100)
	}

	// Both should be cached (width 30 and capped width 50)
	if len(item.cache) != 2 {
		t.Errorf("expected cache to have 2 entries (30 and 50), got %d", len(item.cache))
	}
	if _, ok := item.cache[30]; !ok {
		t.Error("expected cache to have entry for width 30")
	}
	if _, ok := item.cache[50]; !ok {
		t.Error("expected cache to have entry for width 50 (capped from 100)")
	}
}

func BenchmarkMarkdownItemRender(b *testing.B) {
	markdown := "# Heading\n\nThis is a paragraph with **bold** and *italic* text.\n\n- Item 1\n- Item 2\n- Item 3"
	item := NewMarkdownItem("1", markdown)

	// Prime the cache
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	item.Draw(&screen, area)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screen := uv.NewScreenBuffer(80, 10)
		area := uv.Rect(0, 0, 80, 10)
		item.Draw(&screen, area)
	}
}

func BenchmarkMarkdownItemUncached(b *testing.B) {
	markdown := "# Heading\n\nThis is a paragraph with **bold** and *italic* text.\n\n- Item 1\n- Item 2\n- Item 3"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := NewMarkdownItem("1", markdown)
		screen := uv.NewScreenBuffer(80, 10)
		area := uv.Rect(0, 0, 80, 10)
		item.Draw(&screen, area)
	}
}

// Helper function to create a pointer to uint
func uintPtr(v uint) *uint {
	return &v
}
