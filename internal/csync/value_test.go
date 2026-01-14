package csync

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValue_GetSet(t *testing.T) {
	t.Parallel()

	v := NewValue(42)
	require.Equal(t, 42, v.Get())

	v.Set(100)
	require.Equal(t, 100, v.Get())
}

func TestValue_ZeroValue(t *testing.T) {
	t.Parallel()

	v := NewValue("")
	require.Equal(t, "", v.Get())

	v.Set("hello")
	require.Equal(t, "hello", v.Get())
}

func TestValue_Struct(t *testing.T) {
	t.Parallel()

	type config struct {
		Name  string
		Count int
	}

	v := NewValue(config{Name: "test", Count: 1})
	require.Equal(t, config{Name: "test", Count: 1}, v.Get())

	v.Set(config{Name: "updated", Count: 2})
	require.Equal(t, config{Name: "updated", Count: 2}, v.Get())
}

func TestValue_PointerPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		NewValue(&struct{}{})
	})
}

func TestValue_SlicePanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		NewValue([]string{"a", "b"})
	})
}

func TestValue_MapPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		NewValue(map[string]int{"a": 1})
	})
}

func TestValue_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	v := NewValue(0)
	var wg sync.WaitGroup

	// Concurrent writers.
	for i := range 100 {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			v.Set(val)
		}(i)
	}

	// Concurrent readers.
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = v.Get()
		}()
	}

	wg.Wait()

	// Value should be one of the set values (0-99).
	got := v.Get()
	require.GreaterOrEqual(t, got, 0)
	require.Less(t, got, 100)
}
