package chat

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// buildThinkingBlock generates a realistic long thinking block with
// paragraphs, lists, and code fences — the kind of content that
// triggers the CHARM-1785 perf bug.
func buildThinkingBlock(paragraphs int) string {
	var b strings.Builder
	for i := range paragraphs {
		fmt.Fprintf(&b, "Let me think about step %d of this problem.\n\n", i+1)
		fmt.Fprintf(&b, "There are several considerations here. First, we need to think about the data flow. Second, we should consider error handling. Third, performance matters.\n\n")
		b.WriteString("Here are the key points:\n\n")
		b.WriteString("- The first point is about correctness\n")
		b.WriteString("- The second point is about safety\n")
		b.WriteString("- The third point is about speed\n\n")
		fmt.Fprintf(&b, "After considering all of these factors, I believe the best approach is to proceed carefully with step %d.\n\n", i+1)
	}
	return b.String()
}

// BenchmarkStreamingThinking benchmarks the streaming render path for
// a long thinking block. Before CHARM-1785, every tick did a full
// glamour re-render of the entire accumulated text because
// prefixHasOpenHazard rejected any document containing a list marker.
// After the fix, the stable-prefix cache seeds and each tick only
// re-renders the trailing delta.
func BenchmarkStreamingThinking(b *testing.B) {
	sty := styles.CharmtonePantera()
	width := 80

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(sty.Markdown),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		b.Fatal(err)
	}

	// Simulate a thinking block that grows incrementally, as it
	// would during streaming. Each iteration appends one paragraph
	// and renders.
	full := buildThinkingBlock(200) // ~200 paragraphs, ~1200 lines.
	paragraphs := strings.Split(full, "\n\n")

	b.ResetTimer()
	for b.Loop() {
		var sm streamingMarkdown
		var accumulated strings.Builder
		for _, p := range paragraphs {
			if accumulated.Len() > 0 {
				accumulated.WriteString("\n\n")
			}
			accumulated.WriteString(p)
			_ = sm.Render(accumulated.String(), width, renderer)
		}
	}
}

// BenchmarkStreamingThinkingSteadyState benchmarks the steady-state
// path where the thinking block is already large and each tick appends
// a small delta. This is the hot path during actual streaming.
func BenchmarkStreamingThinkingSteadyState(b *testing.B) {
	sty := styles.CharmtonePantera()
	width := 80

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(sty.Markdown),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		b.Fatal(err)
	}

	// Pre-build a large thinking block and seed the cache.
	base := buildThinkingBlock(200)
	var sm streamingMarkdown
	_ = sm.Render(base, width, renderer)

	// Each iteration appends a small delta (one sentence) and
	// re-renders. This is what happens on every streaming tick.
	delta := "Let me reconsider this approach once more.\n\n"

	b.ResetTimer()
	for b.Loop() {
		content := base + delta
		_ = sm.Render(content, width, renderer)
	}
}

// BenchmarkFindBoundaryAfter benchmarks the incremental boundary
// search vs the old full-scan approach on a large document.
func BenchmarkFindBoundaryAfter(b *testing.B) {
	content := buildThinkingBlock(200)

	b.Run("full-scan", func(b *testing.B) {
		for b.Loop() {
			_ = findSafeMarkdownBoundary(content)
		}
	})

	b.Run("incremental", func(b *testing.B) {
		// Seed a stable prefix at ~80% of the document.
		boundary := findSafeMarkdownBoundary(content[:len(content)*4/5])
		if boundary <= 0 {
			b.Fatal("no boundary found in prefix")
		}
		sm := streamingMarkdown{
			width:             80,
			stablePrefix:      content[:boundary],
			baseFenceCount:    countFenceLines(content[:boundary]),
			baseHasListMarker: chunkHasListMarker(content[:boundary]),
		}
		b.ResetTimer()
		for b.Loop() {
			_ = sm.findBoundaryAfter(content)
		}
	})
}
