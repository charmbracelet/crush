package list

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestRenderHelper(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.SetSize(80, 10)

	// Render to string
	output := l.Render()

	if len(output) == 0 {
		t.Error("expected non-empty output from Render()")
	}

	// Check that output contains the items
	if !strings.Contains(output, "Item 1") {
		t.Error("expected output to contain 'Item 1'")
	}
	if !strings.Contains(output, "Item 2") {
		t.Error("expected output to contain 'Item 2'")
	}
	if !strings.Contains(output, "Item 3") {
		t.Error("expected output to contain 'Item 3'")
	}
}

func TestRenderWithScrolling(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
		NewStringItem("4", "Item 4"),
		NewStringItem("5", "Item 5"),
	}

	l := New(items...)
	l.SetSize(80, 2) // Small viewport

	// Initial render should show first 2 items
	output := l.Render()
	if !strings.Contains(output, "Item 1") {
		t.Error("expected output to contain 'Item 1'")
	}
	if !strings.Contains(output, "Item 2") {
		t.Error("expected output to contain 'Item 2'")
	}
	if strings.Contains(output, "Item 3") {
		t.Error("expected output to NOT contain 'Item 3' in initial view")
	}

	// Scroll down and render
	l.ScrollBy(2)
	output = l.Render()

	// Now should show items 3 and 4
	if strings.Contains(output, "Item 1") {
		t.Error("expected output to NOT contain 'Item 1' after scrolling")
	}
	if !strings.Contains(output, "Item 3") {
		t.Error("expected output to contain 'Item 3' after scrolling")
	}
	if !strings.Contains(output, "Item 4") {
		t.Error("expected output to contain 'Item 4' after scrolling")
	}
}

func TestRenderEmptyList(t *testing.T) {
	l := New()
	l.SetSize(80, 10)

	output := l.Render()
	if output != "" {
		t.Errorf("expected empty output for empty list, got: %q", output)
	}
}

func TestRenderVsDrawConsistency(t *testing.T) {
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
	}

	l := New(items...)
	l.SetSize(80, 10)

	// Render using Render() method
	renderOutput := l.Render()

	// Render using Draw() method
	screen := uv.NewScreenBuffer(80, 10)
	area := uv.Rect(0, 0, 80, 10)
	l.Draw(&screen, area)
	drawOutput := screen.Render()

	// Trim any trailing whitespace for comparison
	renderOutput = strings.TrimRight(renderOutput, "\n")
	drawOutput = strings.TrimRight(drawOutput, "\n")

	// Both methods should produce the same output
	if renderOutput != drawOutput {
		t.Errorf("Render() and Draw() produced different outputs:\nRender():\n%q\n\nDraw():\n%q",
			renderOutput, drawOutput)
	}
}

func BenchmarkRender(b *testing.B) {
	items := make([]Item, 100)
	for i := range items {
		items[i] = NewStringItem(string(rune(i)), "Item content here")
	}

	l := New(items...)
	l.SetSize(80, 24)
	l.Render() // Prime the buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Render()
	}
}

func BenchmarkRenderWithScrolling(b *testing.B) {
	items := make([]Item, 1000)
	for i := range items {
		items[i] = NewStringItem(string(rune(i)), "Item content here")
	}

	l := New(items...)
	l.SetSize(80, 24)
	l.Render() // Prime the buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.ScrollBy(1)
		_ = l.Render()
	}
}

func TestStringItemCache(t *testing.T) {
	item := NewStringItem("1", "Test content")

	// First draw at width 80 should populate cache
	screen1 := uv.NewScreenBuffer(80, 5)
	area1 := uv.Rect(0, 0, 80, 5)
	item.Draw(&screen1, area1)

	if len(item.cache) != 1 {
		t.Errorf("expected cache to have 1 entry after first draw, got %d", len(item.cache))
	}
	if _, ok := item.cache[80]; !ok {
		t.Error("expected cache to have entry for width 80")
	}

	// Second draw at same width should reuse cache
	screen2 := uv.NewScreenBuffer(80, 5)
	area2 := uv.Rect(0, 0, 80, 5)
	item.Draw(&screen2, area2)

	if len(item.cache) != 1 {
		t.Errorf("expected cache to still have 1 entry after second draw, got %d", len(item.cache))
	}

	// Draw at different width should add to cache
	screen3 := uv.NewScreenBuffer(40, 5)
	area3 := uv.Rect(0, 0, 40, 5)
	item.Draw(&screen3, area3)

	if len(item.cache) != 2 {
		t.Errorf("expected cache to have 2 entries after draw at different width, got %d", len(item.cache))
	}
	if _, ok := item.cache[40]; !ok {
		t.Error("expected cache to have entry for width 40")
	}
}

func TestWrappingItemHeight(t *testing.T) {
	// Short text that fits in one line
	item1 := NewWrappingStringItem("1", "Short")
	if h := item1.Height(80); h != 1 {
		t.Errorf("expected height 1 for short text, got %d", h)
	}

	// Long text that wraps
	longText := "This is a very long line that will definitely wrap when constrained to a narrow width"
	item2 := NewWrappingStringItem("2", longText)

	// At width 80, should be fewer lines than width 20
	height80 := item2.Height(80)
	height20 := item2.Height(20)

	if height20 <= height80 {
		t.Errorf("expected more lines at narrow width (20: %d lines) than wide width (80: %d lines)",
			height20, height80)
	}

	// Non-wrapping version should always be 1 line
	item3 := NewStringItem("3", longText)
	if h := item3.Height(20); h != 1 {
		t.Errorf("expected height 1 for non-wrapping item, got %d", h)
	}
}
