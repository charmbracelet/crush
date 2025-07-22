package csync

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLazySlice_Iter(t *testing.T) {
	t.Parallel()

	data := []string{"a", "b", "c"}
	s := NewLazySlice(func() []string {
		// TODO: use synctest when new Go is out.
		time.Sleep(10 * time.Millisecond) // Small delay to ensure loading happens
		return data
	})

	var result []string
	for v := range s.Iter() {
		result = append(result, v)
	}

	assert.Equal(t, data, result)
}

func TestLazySlice_IterWaitsForLoading(t *testing.T) {
	t.Parallel()

	var loaded atomic.Bool
	data := []string{"x", "y", "z"}

	s := NewLazySlice(func() []string {
		// TODO: use synctest when new Go is out.
		time.Sleep(100 * time.Millisecond)
		loaded.Store(true)
		return data
	})

	assert.False(t, loaded.Load(), "should not be loaded immediately")

	var result []string
	for v := range s.Iter() {
		result = append(result, v)
	}

	assert.True(t, loaded.Load(), "should be loaded after Iter")
	assert.Equal(t, data, result)
}

func TestLazySlice_EmptySlice(t *testing.T) {
	t.Parallel()

	s := NewLazySlice(func() []string {
		return []string{}
	})

	var result []string
	for v := range s.Iter() {
		result = append(result, v)
	}

	assert.Empty(t, result)
}

func TestLazySlice_EarlyBreak(t *testing.T) {
	t.Parallel()

	data := []string{"a", "b", "c", "d", "e"}
	s := NewLazySlice(func() []string {
		time.Sleep(10 * time.Millisecond) // Small delay to ensure loading happens
		return data
	})

	var result []string
	for v := range s.Iter() {
		result = append(result, v)
		if len(result) == 2 {
			break
		}
	}

	assert.Equal(t, []string{"a", "b"}, result)
}
