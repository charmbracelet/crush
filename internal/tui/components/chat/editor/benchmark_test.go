package editor

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/stretchr/testify/require"
)

// BenchmarkSelectAll measures performance of select all on various text sizes
func BenchmarkSelectAll(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			// Generate test text
			text := strings.Repeat("a", size)
			
			ta := textarea.New()
			ta.SetValue(text)
			
			esm := NewSelectionManager(ta)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				esm.SelectAll()
			}
		})
	}
}

// BenchmarkGetSelectedText measures performance of text extraction
func BenchmarkGetSelectedText(b *testing.B) {
	selectionSizes := []int{10, 100, 1000, 10000}
	textSize := 100000
	
	for _, selSize := range selectionSizes {
		b.Run(fmt.Sprintf("selection_%d", selSize), func(b *testing.B) {
			text := strings.Repeat("a", textSize)
			
			ta := textarea.New()
			ta.SetValue(text)
			
			esm := NewSelectionManager(ta)
			esm.SetSelection(0, selSize)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = esm.GetSelectedText()
			}
		})
	}
}

// BenchmarkPositionConversion measures performance of position<->char conversion
func BenchmarkPositionConversion(b *testing.B) {
	text := strings.Repeat("line\n", 10000) // ~50k chars
	
	for i := 0; i < b.N; i++ {
		charPos := i % 50000
		pos := FromCharPosition(text, charPos)
		_ = pos.CharPosition(text)
	}
}

// BenchmarkEnhancedVsOriginal compares enhanced selection with original
func BenchmarkEnhancedVsOriginal(b *testing.B) {
	text := strings.Repeat("a", 10000)
	
	b.Run("Original", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		
		esm := NewSelectionManager(ta)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			esm.SelectAll()
			_ = esm.GetSelectedText()
			esm.Clear()
		}
	})
	
	b.Run("Enhanced", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		
		esm := NewEnhancedSelectionManager(ta)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			esm.SelectAll()
			_ = esm.GetSelectedText()
			esm.Clear()
		}
	})
}

// Test performance regressions
func TestPerformanceRegression(t *testing.T) {
	t.Parallel()
	
	// Test that large selections don't cause performance issues
	text := strings.Repeat("test content ", 1000) // ~13k chars
	
	ta := textarea.New()
	ta.SetValue(text)
	esm := NewSelectionManager(ta)
	
	// Time select all operation
	start := time.Now()
	esm.SelectAll()
	selectAllTime := time.Since(start)
	
	// Should complete within reasonable time (10ms for 13k chars)
	require.Less(t, selectAllTime, 10*time.Millisecond, "SelectAll should be fast")
	
	// Time get selected text
	start = time.Now()
	selectedText := esm.GetSelectedText()
	getTextTime := time.Since(start)
	
	require.Less(t, getTextTime, 5*time.Millisecond, "GetSelectedText should be fast")
	require.Equal(t, text, selectedText, "Should select all text correctly")
}