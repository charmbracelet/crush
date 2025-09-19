package csync

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLazySlice_Seq(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		t.Helper()
		data := []string{"a", "b", "c"}
		s := NewLazySlice(func() []string {
			time.Sleep(10 * time.Millisecond) // Small delay to ensure loading happens
			return data
		})
		require.Equal(t, data, slices.Collect(s.Seq()))
	})
}

func TestLazySlice_SeqWaitsForLoading(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		t.Helper()

		var loaded atomic.Bool
		data := []string{"x", "y", "z"}

		s := NewLazySlice(func() []string {
			time.Sleep(100 * time.Millisecond)
			loaded.Store(true)
			return data
		})

		require.False(t, loaded.Load(), "should not be loaded immediately")
		require.Equal(t, data, slices.Collect(s.Seq()))
		require.True(t, loaded.Load(), "should be loaded after Seq")
	})
}

func TestLazySlice_EmptySlice(t *testing.T) {
	t.Parallel()
	s := NewLazySlice(func() []string {
		return []string{}
	})
	require.Empty(t, slices.Collect(s.Seq()))
}

func TestLazySlice_EarlyBreak(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		t.Helper()
		data := []string{"a", "b", "c", "d", "e"}
		s := NewLazySlice(func() []string {
			time.Sleep(10 * time.Millisecond) // Small delay to ensure loading happens
			return data
		})

		var result []string
		for v := range s.Seq() {
			result = append(result, v)
			if len(result) == 2 {
				break
			}
		}

		require.Equal(t, []string{"a", "b"}, result)
	})
}

func TestSlice(t *testing.T) {
	t.Run("NewSlice", func(t *testing.T) {
		s := NewSlice[int]()
		require.Equal(t, 0, s.Len())
	})

	t.Run("NewSliceFrom", func(t *testing.T) {
		original := []int{1, 2, 3}
		s := NewSliceFrom(original)
		require.Equal(t, 3, s.Len())

		// Verify it's a copy, not a reference
		original[0] = 999
		val, ok := s.Get(0)
		require.True(t, ok)
		require.Equal(t, 1, val)
	})

	t.Run("Append", func(t *testing.T) {
		s := NewSlice[string]()
		s.Append("hello")
		s.Append("world")

		require.Equal(t, 2, s.Len())
		val, ok := s.Get(0)
		require.True(t, ok)
		require.Equal(t, "hello", val)

		val, ok = s.Get(1)
		require.True(t, ok)
		require.Equal(t, "world", val)
	})

	t.Run("Prepend", func(t *testing.T) {
		s := NewSlice[string]()
		s.Append("world")
		s.Prepend("hello")

		require.Equal(t, 2, s.Len())
		val, ok := s.Get(0)
		require.True(t, ok)
		require.Equal(t, "hello", val)

		val, ok = s.Get(1)
		require.True(t, ok)
		require.Equal(t, "world", val)
	})

	t.Run("Delete", func(t *testing.T) {
		s := NewSliceFrom([]int{1, 2, 3, 4, 5})

		// Delete middle element
		ok := s.Delete(2)
		require.True(t, ok)
		require.Equal(t, 4, s.Len())

		expected := []int{1, 2, 4, 5}
		actual := slices.Collect(s.Seq())
		require.Equal(t, expected, actual)

		// Delete out of bounds
		ok = s.Delete(10)
		require.False(t, ok)
		require.Equal(t, 4, s.Len())

		// Delete negative index
		ok = s.Delete(-1)
		require.False(t, ok)
		require.Equal(t, 4, s.Len())
	})

	t.Run("Get", func(t *testing.T) {
		s := NewSliceFrom([]string{"a", "b", "c"})

		val, ok := s.Get(1)
		require.True(t, ok)
		require.Equal(t, "b", val)

		// Out of bounds
		_, ok = s.Get(10)
		require.False(t, ok)

		// Negative index
		_, ok = s.Get(-1)
		require.False(t, ok)
	})

	t.Run("Set", func(t *testing.T) {
		s := NewSliceFrom([]string{"a", "b", "c"})

		ok := s.Set(1, "modified")
		require.True(t, ok)

		val, ok := s.Get(1)
		require.True(t, ok)
		require.Equal(t, "modified", val)

		// Out of bounds
		ok = s.Set(10, "invalid")
		require.False(t, ok)

		// Negative index
		ok = s.Set(-1, "invalid")
		require.False(t, ok)
	})

	t.Run("SetSlice", func(t *testing.T) {
		s := NewSlice[int]()
		s.Append(1)
		s.Append(2)

		newItems := []int{10, 20, 30}
		s.SetSlice(newItems)

		require.Equal(t, 3, s.Len())
		require.Equal(t, newItems, slices.Collect(s.Seq()))

		// Verify it's a copy
		newItems[0] = 999
		val, ok := s.Get(0)
		require.True(t, ok)
		require.Equal(t, 10, val)
	})

	t.Run("Slice", func(t *testing.T) {
		original := []int{1, 2, 3}
		s := NewSliceFrom(original)

		copied := slices.Collect(s.Seq())
		require.Equal(t, original, copied)

		// Verify it's a copy
		copied[0] = 999
		val, ok := s.Get(0)
		require.True(t, ok)
		require.Equal(t, 1, val)
	})

	t.Run("Seq", func(t *testing.T) {
		s := NewSliceFrom([]int{1, 2, 3})

		var result []int
		for v := range s.Seq() {
			result = append(result, v)
		}

		require.Equal(t, []int{1, 2, 3}, result)
	})

	t.Run("SeqWithIndex", func(t *testing.T) {
		s := NewSliceFrom([]string{"a", "b", "c"})

		var indices []int
		var values []string
		for i, v := range s.Seq2() {
			indices = append(indices, i)
			values = append(values, v)
		}

		require.Equal(t, []int{0, 1, 2}, indices)
		require.Equal(t, []string{"a", "b", "c"}, values)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		s := NewSlice[int]()
		const numGoroutines = 100
		const itemsPerGoroutine = 10

		var wg sync.WaitGroup

		// Concurrent appends
		for i := range numGoroutines {
			wg.Add(2)
			go func(start int) {
				defer wg.Done()
				for j := range itemsPerGoroutine {
					s.Append(start*itemsPerGoroutine + j)
				}
			}(i)
			go func() {
				defer wg.Done()
				for range itemsPerGoroutine {
					s.Len() // Just read the length
				}
			}()
		}

		wg.Wait()

		// Should have all items
		require.Equal(t, numGoroutines*itemsPerGoroutine, s.Len())
	})
}
