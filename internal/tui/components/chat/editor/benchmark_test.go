package editor

import (
	"fmt"
	"runtime"
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
			for b.Loop() {
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
			for b.Loop() {
				_ = esm.GetSelectedText()
			}
		})
	}
}

// BenchmarkPerformanceComparison measures selection performance across different scenarios
func BenchmarkPerformanceComparison(b *testing.B) {
	text := strings.Repeat("a", 10000)
	
	b.Run("SelectAll", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		
		esm := NewSelectionManager(ta)
		
		b.ResetTimer()
		for b.Loop() {
			esm.SelectAll()
		}
	})
	
	b.Run("GetSelectedText", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		
		esm := NewSelectionManager(ta)
		esm.SelectAll()
		
		b.ResetTimer()
		for b.Loop() {
			_ = esm.GetSelectedText()
		}
	})
	
	b.Run("CompleteWorkflow", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		
		esm := NewSelectionManager(ta)
		
		b.ResetTimer()
		for b.Loop() {
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

// BenchmarkMemoryAllocation tracks memory usage during selection operations
func BenchmarkMemoryAllocation(b *testing.B) {
	text := strings.Repeat("a", 10000)
	
	b.Run("SelectAll", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		esm := NewSelectionManager(ta)
		
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		b.ResetTimer()
		for b.Loop() {
			esm.SelectAll()
		}
		
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})
	
	b.Run("GetSelectedText", func(b *testing.B) {
		ta := textarea.New()
		ta.SetValue(text)
		esm := NewSelectionManager(ta)
		esm.SelectAll()
		
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		b.ResetTimer()
		for b.Loop() {
			_ = esm.GetSelectedText()
		}
		
		runtime.ReadMemStats(&m2)
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})
}