package list

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
)

// Mock item for benchmarking
type benchItem struct {
	id      string
	content string
	height  int
	width   int
}

func (b benchItem) ID() string {
	return b.id
}

func (b benchItem) SetSize(width, height int) tea.Cmd {
	b.width = width
	b.height = height
	return nil
}

func (b benchItem) GetSize() (int, int) {
	return b.width, b.height
}

func (b benchItem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return b, nil
}

func (b benchItem) Init() tea.Cmd {
	return nil
}

func (b benchItem) View() string {
	return b.content
}

func (b benchItem) Height() int {
	return b.height
}

// createBenchItems creates n items for benchmarking
func createBenchItems(n int) []Item {
	items := make([]Item, n)
	for i := 0; i < n; i++ {
		items[i] = benchItem{
			id:      fmt.Sprintf("item-%d", i),
			content: fmt.Sprintf("This is item %d with some content that spans multiple lines\nLine 2\nLine 3", i),
			height:  3,
		}
	}
	return items
}

// BenchmarkListRender benchmarks the render performance with different list sizes
func BenchmarkListRender(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			items := createBenchItems(size)
			list := New(items, WithDirectionForward()).(*list[Item])

			// Set dimensions
			list.SetSize(80, 30)

			// Initialize to calculate positions
			list.Init()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				list.render()
			}
		})
	}
}

// BenchmarkListScroll benchmarks scrolling performance
func BenchmarkListScroll(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			items := createBenchItems(size)
			list := New(items, WithDirectionForward())

			// Set dimensions
			list.SetSize(80, 30)

			// Initialize
			list.Init()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Scroll down and up
				list.MoveDown(10)
				list.MoveUp(10)
			}
		})
	}
}

// BenchmarkListView benchmarks the View() method performance
func BenchmarkListView(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			items := createBenchItems(size)
			list := New(items, WithDirectionForward()).(*list[Item])

			// Set dimensions
			list.SetSize(80, 30)

			// Initialize and render once
			list.Init()
			list.render()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = list.View()
			}
		})
	}
}

// BenchmarkListMemory benchmarks memory allocation
func BenchmarkListMemory(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				items := createBenchItems(size)
				list := New(items, WithDirectionForward()).(*list[Item])
				list.SetSize(80, 30)
				list.Init()
				list.render()
				_ = list.View()
			}
		})
	}
}

// BenchmarkVirtualScrolling specifically tests virtual scrolling efficiency
func BenchmarkVirtualScrolling(b *testing.B) {
	// Test with a very large list to see virtual scrolling benefits
	items := createBenchItems(10000)
	list := New(items, WithDirectionForward()).(*list[Item])
	list.SetSize(80, 30)
	list.Init()

	b.Run("RenderVisibleOnly", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// This should only render ~10 items that fit in viewport
			list.renderVirtualScrolling()
		}
	})

	b.Run("ScrollThroughList", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Scroll through the entire list
			for j := 0; j < 100; j++ {
				list.MoveDown(100)
			}
			// Reset to top
			list.GoToTop()
		}
	})
}

// BenchmarkCalculatePositions benchmarks position calculation
func BenchmarkCalculatePositions(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			items := createBenchItems(size)
			list := New(items, WithDirectionForward()).(*list[Item])
			list.SetSize(80, 30)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				list.calculateItemPositions()
			}
		})
	}
}
