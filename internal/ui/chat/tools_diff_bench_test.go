package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
)

// BenchmarkToolOutputDiffContentLargeDiff pins the cost of rendering a large
// collapsed diff. A collapsed card shows collapsedMaxLines lines, so this
// should not scale with the size of the diff.
func BenchmarkToolOutputDiffContentLargeDiff(b *testing.B) {
	sty := styles.CharmtonePantera()
	before := strings.Repeat(strings.Repeat("a", 200)+"\n", 50000)
	after := strings.Repeat(strings.Repeat("b", 200)+"\n", 50000)
	b.ReportAllocs()
	for b.Loop() {
		_ = toolOutputDiffContent(&sty, "f.txt", before, after, 120, false)
	}
}
