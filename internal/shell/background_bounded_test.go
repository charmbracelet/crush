package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncBufferKeepsHeadAndTail(t *testing.T) {
	t.Parallel()

	sb := &syncBuffer{headLimit: 4, tailLimit: 4}
	for _, s := range []string{"HEAD", "middle one", "middle two", "TAIL"} {
		n, err := sb.WriteString(s)
		require.NoError(t, err)
		require.Equal(t, len(s), n, "Write must report every byte handed to it")
	}

	require.Equal(t, int64(len("middle one")+len("middle two")), sb.Dropped())
	got := sb.String()
	require.True(t, strings.HasPrefix(got, "HEAD"), got)
	require.True(t, strings.HasSuffix(got, "TAIL"), got)
	require.Contains(t, got, "bytes of output dropped")
	require.NotContains(t, got, "middle one")
}

func TestSyncBufferUnderLimitIsUnchanged(t *testing.T) {
	t.Parallel()

	sb := newSyncBuffer()
	_, err := sb.WriteString("hello world")
	require.NoError(t, err)
	require.Equal(t, int64(0), sb.Dropped())
	require.Equal(t, "hello world", sb.String())
}

func TestSyncBufferSingleWriteLargerThanLimit(t *testing.T) {
	t.Parallel()

	sb := &syncBuffer{headLimit: 3, tailLimit: 3}
	n, err := sb.WriteString("abcdefghij")
	require.NoError(t, err)
	require.Equal(t, 10, n)
	require.Equal(t, int64(4), sb.Dropped())
	require.True(t, strings.HasPrefix(sb.String(), "abc"))
	require.True(t, strings.HasSuffix(sb.String(), "hij"))
}

func TestSyncBufferZeroValueIsUnbounded(t *testing.T) {
	t.Parallel()

	var sb syncBuffer
	_, err := sb.WriteString(strings.Repeat("x", 4<<20))
	require.NoError(t, err)
	require.Equal(t, int64(0), sb.Dropped())
	require.Len(t, sb.String(), 4<<20)
}

// TestBackgroundJobOutputIsBounded is the regression test: a job that
// writes far more than the limit must not hold all of it in memory.
func TestBackgroundJobOutputIsBounded(t *testing.T) {
	t.Parallel()

	const sizeMiB = 64
	dir := t.TempDir()
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = 'x'
	}
	f, err := os.Create(filepath.Join(dir, "big.txt"))
	require.NoError(t, err)
	for range sizeMiB {
		_, err := f.Write(chunk)
		require.NoError(t, err)
	}
	require.NoError(t, f.Close())

	manager := newBackgroundShellManager()
	bg, err := manager.Start(context.Background(), dir, nil, "cat big.txt", "", testMaxJobs)
	require.NoError(t, err)
	bg.Wait()

	stdout, _, done, _ := bg.GetOutput()
	require.True(t, done)
	require.Less(t, len(stdout), maxBackgroundOutputBytes+1024,
		"a %d MiB job must not keep %d bytes of output in memory", sizeMiB, len(stdout))
	require.Greater(t, bg.stdout.Dropped(), int64(0))
}
