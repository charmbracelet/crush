package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// A collapsed card shows collapsedMaxLines, so the cost of rendering one
// should not scale with the size of the stored content. These benchmarks pin
// that: each renders a large collapsed diff.

func benchContent(n int) (before, after string) {
	return strings.Repeat(strings.Repeat("a", 200)+"\n", n),
		strings.Repeat(strings.Repeat("b", 200)+"\n", n)
}

func BenchmarkToolOutputDiffContentLargeDiff(b *testing.B) {
	sty := styles.CharmtonePantera()
	before, after := benchContent(50000)
	b.ReportAllocs()
	for b.Loop() {
		_ = toolOutputDiffContent(&sty, "f.txt", before, after, 120, false)
	}
}

func BenchmarkToolOutputMultiEditDiffContentLargeDiff(b *testing.B) {
	sty := styles.CharmtonePantera()
	before, after := benchContent(50000)
	meta := tools.MultiEditResponseMetadata{OldContent: before, NewContent: after}
	b.ReportAllocs()
	for b.Loop() {
		_ = toolOutputMultiEditDiffContent(&sty, "f.txt", meta, 1, 120, false)
	}
}

func BenchmarkToolOutputDiffContentFromUnifiedLargeDiff(b *testing.B) {
	sty := styles.CharmtonePantera()
	before, after := benchContent(50000)
	unified, _, _ := diff.GenerateDiff(before, after, "f.txt")
	b.ReportAllocs()
	for b.Loop() {
		_ = toolOutputDiffContentFromUnified(&sty, unified, 120, false)
	}
}

// Glamour is superlinear, so large content is shown as plain text instead.
// Rendering this as markdown takes seconds; the assertion is on the output
// because unlike the diff renderers this fallback changes what is displayed.
func TestToolOutputMarkdownContentFallsBackWhenLarge(t *testing.T) {
	sty := styles.CharmtonePantera()
	content := strings.Repeat("# heading\nsome text in a line here\n", 12000)
	if len(content) <= maxMarkdownBytes {
		t.Fatalf("test content is %d bytes, want more than %d", len(content), maxMarkdownBytes)
	}

	got := toolOutputMarkdownContent(&sty, content, 80, false)
	want := toolOutputPlainContent(&sty, content, 80, false)
	if got != want {
		t.Fatal("large content should fall back to plain rendering")
	}
}

func TestToolOutputMarkdownContentRendersSmallContent(t *testing.T) {
	sty := styles.CharmtonePantera()
	content := "# heading\n\nsome *emphasised* text\n"

	got := toolOutputMarkdownContent(&sty, content, 80, false)
	if got == toolOutputPlainContent(&sty, content, 80, false) {
		t.Fatal("small content should still be rendered as markdown")
	}
}
