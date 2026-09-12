package tools

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/stringext"
)

// TruncationProvenance records where content was cut: at capture,
// before the result landed in history, or at recall, on read-back of
// the stored result. The marker text carries it so a reader can tell
// a persistence cap from a retrieval excerpt.
type TruncationProvenance string

const (
	// TruncatedAtCapture marks a write-time cap — a per-tool output
	// limit or the universal tool-result cap.
	TruncatedAtCapture TruncationProvenance = "capture"
	// TruncatedAtRecall marks a read-back bound applied to a stored
	// result being returned through recall.
	TruncatedAtRecall TruncationProvenance = "recall"
)

// TruncateHeadTail caps content at maxBytes, keeping the head and tail
// and replacing the middle with a labeled marker: what was cut, how
// much, where the cut happened, and how to recover. It is byte-based,
// never splits a UTF-8 rune, and never leaves a partial ANSI escape at
// the cut.
func TruncateHeadTail(content string, maxBytes int, prov TruncationProvenance) string {
	if len(content) <= maxBytes {
		return content
	}
	half := maxBytes / 2
	headEnd := stringext.CutANSISafeLeft(content, half)
	tailStart := stringext.CutANSISafeRight(content, len(content)-half)
	head := content[:headEnd]
	tail := content[tailStart:]
	omitted := content[headEnd:tailStart]
	return fmt.Sprintf("%s\n\n... [%d lines (%s) truncated at %s; re-run or re-view the source for the rest] ...\n\n%s",
		head, strings.Count(omitted, "\n"), humanBytes(int64(len(omitted))), prov, tail)
}

// TruncateOutput caps tool output at MaxOutputLength at capture time.
// It is the standard per-tool output bound.
func TruncateOutput(content string) string {
	return TruncateHeadTail(content, MaxOutputLength, TruncatedAtCapture)
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
