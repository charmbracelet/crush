package list

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkListInit benchmarks the initialization of a list
func BenchmarkListInit(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			b.ResetTimer()
			for range b.N {
				l := New(items, WithDirectionForward(), WithSize(80, 24))
				execCmd(l, l.Init())
			}
		})
	}
}

// BenchmarkListView benchmarks the View rendering
func BenchmarkListView(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))
			execCmd(l, l.Init())

			b.ResetTimer()
			for range b.N {
				_ = l.View()
			}
		})
	}
}

// BenchmarkListAppendItem benchmarks appending items
func BenchmarkListAppendItem(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("initial=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				items := make([]Item, size)
				for i := range size {
					items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
				}
				l := New(items, WithDirectionForward(), WithSize(80, 24))
				execCmd(l, l.Init())
				b.StartTimer()

				newItem := NewSelectableItem("New Item")
				execCmd(l, l.AppendItem(newItem))
			}
		})
	}
}

// BenchmarkListPrependItem benchmarks prepending items
func BenchmarkListPrependItem(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("initial=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				items := make([]Item, size)
				for i := range size {
					items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
				}
				l := New(items, WithDirectionForward(), WithSize(80, 24))
				execCmd(l, l.Init())
				b.StartTimer()

				newItem := NewSelectableItem("New Item")
				execCmd(l, l.PrependItem(newItem))
			}
		})
	}
}

// BenchmarkListUpdateItem benchmarks updating items
func BenchmarkListUpdateItem(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))
			execCmd(l, l.Init())

			// Update the middle item
			middleID := items[size/2].ID()

			b.ResetTimer()
			for range b.N {
				newItem := NewSelectableItem("Updated Item")
				execCmd(l, l.UpdateItem(middleID, newItem))
			}
		})
	}
}

// BenchmarkListDeleteItem benchmarks deleting items
func BenchmarkListDeleteItem(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				items := make([]Item, size)
				for i := range size {
					items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
				}
				l := New(items, WithDirectionForward(), WithSize(80, 24))
				execCmd(l, l.Init())

				// Delete the middle item
				middleID := items[size/2].ID()
				b.StartTimer()

				execCmd(l, l.DeleteItem(middleID))
			}
		})
	}
}

// BenchmarkListMoveDown benchmarks scrolling down
func BenchmarkListMoveDown(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))
			execCmd(l, l.Init())

			b.ResetTimer()
			for i := range b.N {
				execCmd(l, l.MoveDown(1))
				// Reset position every 20 moves to avoid hitting the bottom
				if i%20 == 0 {
					execCmd(l, l.GoToTop())
				}
			}
		})
	}
}

// BenchmarkListSelectItemBelow benchmarks selecting items
func BenchmarkListSelectItemBelow(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))
			execCmd(l, l.Init())

			b.ResetTimer()
			for i := range b.N {
				execCmd(l, l.SelectItemBelow())
				// Reset position every 20 moves to avoid hitting the bottom
				if i%20 == 0 {
					execCmd(l, l.GoToTop())
				}
			}
		})
	}
}

// BenchmarkListFocusBlur benchmarks focus/blur operations
func BenchmarkListFocusBlur(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))
			execCmd(l, l.Init())

			b.ResetTimer()
			for range b.N {
				execCmd(l, l.Blur())
				execCmd(l, l.Focus())
			}
		})
	}
}

// BenchmarkListWithMultilineItems benchmarks lists with multiline items
func BenchmarkListWithMultilineItems(b *testing.B) {
	sizes := []int{10, 50, 100}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				content := strings.Repeat(fmt.Sprintf("Line %d\n", i), 5)
				content = strings.TrimSuffix(content, "\n")
				items[i] = NewSelectableItem(content)
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))

			b.ResetTimer()
			for range b.N {
				execCmd(l, l.Init())
				_ = l.View()
			}
		})
	}
}

// BenchmarkListSelectionView benchmarks rendering with text selection
func BenchmarkListSelectionView(b *testing.B) {
	sizes := []int{50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d with some longer text to make it realistic", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24)).(*list[Item])
			execCmd(l, l.Init())

			// Create a selection
			l.StartSelection(0, 0)
			l.EndSelection(10, 5)
			l.SelectionStop()

			b.ResetTimer()
			for range b.N {
				_ = l.View()
			}
		})
	}
}

// BenchmarkListSetItems benchmarks replacing all items
func BenchmarkListSetItems(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("items=%d", size), func(b *testing.B) {
			items := make([]Item, size)
			for i := range size {
				items[i] = NewSelectableItem(fmt.Sprintf("Item %d", i))
			}

			l := New(items, WithDirectionForward(), WithSize(80, 24))
			execCmd(l, l.Init())

			// Create new items for replacement
			newItems := make([]Item, size)
			for i := range size {
				newItems[i] = NewSelectableItem(fmt.Sprintf("New Item %d", i))
			}

			b.ResetTimer()
			for range b.N {
				execCmd(l, l.SetItems(newItems))
			}
		})
	}
}

// BenchmarkListAnimStep benchmarks animation step handling
func BenchmarkListAnimStep(b *testing.B) {
	// This would require HasAnim items, which we don't have in the test setup
	// but we include it as a placeholder for when such items are available
	b.Skip("Skipping anim step benchmark - requires HasAnim items")
}
