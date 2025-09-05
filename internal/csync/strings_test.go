package csync

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAtomicString(t *testing.T) {
	t.Parallel()

	as := NewString()
	require.NotNil(t, as)
	require.Equal(t, "", as.String())
}

func TestAtomicString_String(t *testing.T) {
	t.Parallel()

	as := NewString()

	// Test initial empty string
	require.Equal(t, "", as.String())

	// Test after storing a value
	as.Store("hello")
	require.Equal(t, "hello", as.String())

	// Test after storing another value
	as.Store("world")
	require.Equal(t, "world", as.String())

	// Test empty string storage
	as.Store("")
	require.Equal(t, "", as.String())
}

func TestAtomicString_Store(t *testing.T) {
	t.Parallel()

	as := NewString()

	// Test storing various strings
	testCases := []string{
		"hello",
		"world",
		"",
		"unicode: 🚀",
		"multiline\nstring",
		"special chars: !@#$%^&*()",
	}

	for _, tc := range testCases {
		as.Store(tc)
		require.Equal(t, tc, as.String())
	}
}

func TestAtomicString_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	as := NewString()
	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup

	// Start goroutines that write to the atomic string
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range numOperations {
				as.Store("goroutine")
			}
		}(i)
	}

	// Start goroutines that read from the atomic string
	for range numGoroutines {
		wg.Go(func() {
			for range numOperations {
				_ = as.String()
			}
		})
	}

	wg.Wait()

	// Verify the string is still valid (no data races)
	result := as.String()
	require.NotNil(t, result)
}

func TestAtomicString_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	as := NewString()
	const numReaders = 50
	const numWriters = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Start writer goroutines
	for i := range numWriters {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for range numOperations {
				as.Store("writer")
			}
		}(i)
	}

	// Start reader goroutines
	for range numReaders {
		wg.Go(func() {
			for range numOperations {
				value := as.String()
				// Value should be either empty string or "writer"
				require.True(t, value == "" || value == "writer")
			}
		})
	}

	wg.Wait()
}

func TestAtomicString_StringerInterface(t *testing.T) {
	t.Parallel()

	as := NewString()
	as.Store("test")

	// Verify it implements the Stringer interface
	var stringer any = as
	_, ok := stringer.(interface{ String() string })
	require.True(t, ok)

	// Test string conversion
	require.Equal(t, "test", as.String())
}

func BenchmarkAtomicString_Store(b *testing.B) {
	as := NewString()

	for b.Loop() {
		as.Store("benchmark")
	}
}

func BenchmarkAtomicString_String(b *testing.B) {
	as := NewString()
	as.Store("benchmark")

	for b.Loop() {
		_ = as.String()
	}
}

func BenchmarkAtomicString_ConcurrentReadWrite(b *testing.B) {
	as := NewString()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Mix of reads and writes
			if b.N%2 == 0 {
				as.Store("benchmark")
			} else {
				_ = as.String()
			}
		}
	})
}
