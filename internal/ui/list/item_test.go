package list

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour/v2/ansi"
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

func TestSpacerItem(t *testing.T) {
	spacer := NewSpacerItem("spacer1", 3)

	// Check ID
	if spacer.ID() != "spacer1" {
		t.Errorf("expected ID 'spacer1', got %q", spacer.ID())
	}

	// Check height
	if h := spacer.Height(80); h != 3 {
		t.Errorf("expected height 3, got %d", h)
	}

	// Height should be constant regardless of width
	if h := spacer.Height(20); h != 3 {
		t.Errorf("expected height 3 for width 20, got %d", h)
	}

	// Draw should not produce any visible content
	screen := uv.NewScreenBuffer(20, 3)
	area := uv.Rect(0, 0, 20, 3)
	spacer.Draw(&screen, area)

	output := screen.Render()
	// Should be empty (just spaces)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			t.Errorf("expected empty spacer output, got: %q", line)
		}
	}
}

func TestSpacerItemInList(t *testing.T) {
	// Create a list with items separated by spacers
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewSpacerItem("spacer1", 1),
		NewStringItem("2", "Item 2"),
		NewSpacerItem("spacer2", 2),
		NewStringItem("3", "Item 3"),
	}

	l := New(items...)
	l.SetSize(20, 10)

	output := l.Render()

	// Should contain all three items
	if !strings.Contains(output, "Item 1") {
		t.Error("expected output to contain 'Item 1'")
	}
	if !strings.Contains(output, "Item 2") {
		t.Error("expected output to contain 'Item 2'")
	}
	if !strings.Contains(output, "Item 3") {
		t.Error("expected output to contain 'Item 3'")
	}

	// Total height should be: 1 (item1) + 1 (spacer1) + 1 (item2) + 2 (spacer2) + 1 (item3) = 6
	expectedHeight := 6
	if l.TotalHeight() != expectedHeight {
		t.Errorf("expected total height %d, got %d", expectedHeight, l.TotalHeight())
	}
}

func TestSpacerItemNavigation(t *testing.T) {
	// Spacers should not be selectable (they're not focusable)
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewSpacerItem("spacer1", 1),
		NewStringItem("2", "Item 2"),
	}

	l := New(items...)
	l.SetSize(20, 10)

	// Select first item
	l.SetSelectedIndex(0)
	if l.SelectedIndex() != 0 {
		t.Errorf("expected selected index 0, got %d", l.SelectedIndex())
	}

	// Can select the spacer (it's a valid item, just not focusable)
	l.SetSelectedIndex(1)
	if l.SelectedIndex() != 1 {
		t.Errorf("expected selected index 1, got %d", l.SelectedIndex())
	}

	// Can select item after spacer
	l.SetSelectedIndex(2)
	if l.SelectedIndex() != 2 {
		t.Errorf("expected selected index 2, got %d", l.SelectedIndex())
	}
}

// Helper function to create a pointer to uint
func uintPtr(v uint) *uint {
	return &v
}

func TestListDoesNotEatLastLine(t *testing.T) {
	// Create items that exactly fill the viewport
	items := []Item{
		NewStringItem("1", "Line 1"),
		NewStringItem("2", "Line 2"),
		NewStringItem("3", "Line 3"),
		NewStringItem("4", "Line 4"),
		NewStringItem("5", "Line 5"),
	}

	// Create list with height exactly matching content (5 lines, no gaps)
	l := New(items...)
	l.SetSize(20, 5)

	// Render the list
	output := l.Render()

	// Count actual lines in output
	lines := strings.Split(strings.TrimRight(output, "\r\n"), "\r\n")
	actualLineCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			actualLineCount++
		}
	}

	// All 5 items should be visible
	if !strings.Contains(output, "Line 1") {
		t.Error("expected output to contain 'Line 1'")
	}
	if !strings.Contains(output, "Line 2") {
		t.Error("expected output to contain 'Line 2'")
	}
	if !strings.Contains(output, "Line 3") {
		t.Error("expected output to contain 'Line 3'")
	}
	if !strings.Contains(output, "Line 4") {
		t.Error("expected output to contain 'Line 4'")
	}
	if !strings.Contains(output, "Line 5") {
		t.Error("expected output to contain 'Line 5'")
	}

	if actualLineCount != 5 {
		t.Errorf("expected 5 lines with content, got %d", actualLineCount)
	}
}

func TestListWithScrollDoesNotEatLastLine(t *testing.T) {
	// Create more items than viewport height
	items := []Item{
		NewStringItem("1", "Item 1"),
		NewStringItem("2", "Item 2"),
		NewStringItem("3", "Item 3"),
		NewStringItem("4", "Item 4"),
		NewStringItem("5", "Item 5"),
		NewStringItem("6", "Item 6"),
		NewStringItem("7", "Item 7"),
	}

	// Viewport shows 3 items at a time
	l := New(items...)
	l.SetSize(20, 3)

	// Need to render first to build the buffer and calculate total height
	_ = l.Render()

	// Now scroll to bottom
	l.ScrollToBottom()

	output := l.Render()

	t.Logf("Output:\n%s", output)
	t.Logf("Offset: %d, Total height: %d", l.offset, l.TotalHeight())

	// Should show last 3 items: 5, 6, 7
	if !strings.Contains(output, "Item 5") {
		t.Error("expected output to contain 'Item 5'")
	}
	if !strings.Contains(output, "Item 6") {
		t.Error("expected output to contain 'Item 6'")
	}
	if !strings.Contains(output, "Item 7") {
		t.Error("expected output to contain 'Item 7'")
	}

	// Should not show earlier items
	if strings.Contains(output, "Item 1") {
		t.Error("expected output to NOT contain 'Item 1' when scrolled to bottom")
	}
}
