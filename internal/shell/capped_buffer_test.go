package shell

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestCappedBufferKeepsSmallOutputWhole(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(16)
	b.Write([]byte("hello "))
	b.Write([]byte("world"))
	require.Equal(t, "hello world", b.String())
}

func TestCappedBufferKeepsStartAndEnd(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(8)
	b.Write([]byte("STAR"))
	for range 1000 {
		b.Write([]byte("x"))
	}
	b.Write([]byte("THEEND"))
	require.Equal(t, "STAR\n\n... [1002 bytes truncated] ...\n\nEEND", b.String())
	require.LessOrEqual(t, len(b.head)+len(b.ring), 8, "memory stays at the limit")
}

// One write larger than the whole buffer keeps only its own start and end.
func TestCappedBufferHugeWrite(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(8)
	b.Write([]byte("abcd" + strings.Repeat("-", 100) + "wxyz"))
	require.Equal(t, "abcd\n\n... [100 bytes truncated] ...\n\nwxyz", b.String())
}

func TestCappedBufferExactHalfThenOneMore(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(8)
	b.Write([]byte("abcd"))
	require.Equal(t, "abcd", b.String())
	require.Len(t, b.head, 4)
	require.Empty(t, b.ring)

	b.Write([]byte("e"))
	require.Equal(t, "abcde", b.String())
	require.Len(t, b.ring, 1)
}

func TestCappedBufferExactLimit(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(8)
	b.Write([]byte("abcdefgh"))
	require.Equal(t, "abcdefgh", b.String())
	require.Zero(t, b.dropped)

	b.Write([]byte("i"))
	require.Equal(t, "abcd\n\n... [1 bytes truncated] ...\n\nfghi", b.String())
}

// Multi-byte runes straddling both cut points are trimmed whole.
func TestCappedBufferKeepsUTF8Valid(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(8)
	// The head cut falls inside the first "é" and the tail cut inside
	// the last "世".
	b.Write([]byte("abcé" + strings.Repeat("-", 50) + "世xy"))
	got := b.String()
	require.True(t, utf8.ValidString(got), "got %q", got)
	require.Equal(t, "abc\n\n... [55 bytes truncated] ...\n\nxy", got)
}

func TestCappedBufferConcurrentWriters(t *testing.T) {
	t.Parallel()
	b := newCappedBuffer(64)
	var wg sync.WaitGroup
	for _, s := range []string{"a", "b"} {
		wg.Go(func() {
			for range 1000 {
				b.Write([]byte(s))
			}
		})
	}
	wg.Wait()
	got := b.String()
	require.Contains(t, got, "[1936 bytes truncated]")
	require.Len(t, got, 64+len("\n\n... [1936 bytes truncated] ...\n\n"))
}

// A command that never stops writing must not grow memory without bound.
func TestBackgroundShellOutputStaysBounded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping flacky test on windows")
	}
	t.Parallel()
	manager := newBackgroundShellManager()
	// Long lines reach the cap in few loop iterations, even under -race.
	command := "while :; do echo " + strings.Repeat("x", 4096) + "; done"
	bg, err := manager.Start(t.Context(), nil, t.TempDir(), nil, CoreutilsTurtle, command, "endless", testMaxJobs)
	require.NoError(t, err)
	t.Cleanup(func() { manager.Kill(context.Background(), bg.ID) })

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(bg.stdout.String(), "bytes truncated") {
		require.True(t, time.Now().Before(deadline), "output never reached the cap")
		time.Sleep(10 * time.Millisecond)
	}
	stdout, _, done, _ := bg.GetOutput()
	require.False(t, done)
	require.Less(t, len(stdout), maxKeptOutput+64)
}

func TestCappedBufferTail(t *testing.T) {
	t.Parallel()

	t.Run("spans head and ring without truncation", func(t *testing.T) {
		t.Parallel()
		b := newCappedBuffer(16)
		require.Zero(t, b.seq.Load())
		b.Write(nil)
		require.Zero(t, b.seq.Load())

		b.Write([]byte("hello world"))
		require.Equal(t, uint64(1), b.seq.Load())
		require.Equal(t, "", b.Tail(0))
		require.Equal(t, "ld", b.Tail(2))
		require.Equal(t, "world", b.Tail(5))
		require.Equal(t, "hello world", b.Tail(20))
	})

	t.Run("wrapped ring returns only continuous tail", func(t *testing.T) {
		t.Parallel()
		b := newCappedBuffer(8)
		b.Write([]byte("abcd"))
		b.Write([]byte("123456"))
		b.Write([]byte("wxyz"))
		require.Equal(t, "yz", b.Tail(2))
		require.Equal(t, "wxyz", b.Tail(4))
		require.Equal(t, "wxyz", b.Tail(100))
	})

	t.Run("skips leading partial UTF-8 rune on cut", func(t *testing.T) {
		t.Parallel()
		b := newCappedBuffer(16)
		b.Write([]byte("ab世"))
		got := b.Tail(2)
		require.True(t, utf8.ValidString(got))
		require.Equal(t, "", got)
		require.Equal(t, "世", b.Tail(3))
		require.Equal(t, "b世", b.Tail(4))
	})
}
