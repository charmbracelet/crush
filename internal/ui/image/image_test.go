package image

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResetCache(t *testing.T) {
	t.Parallel()

	storeCached(imageKey{id: "a", cols: 10, rows: 10}, cachedImage{blocks: "a"})
	storeCached(imageKey{id: "b", cols: 20, rows: 20}, cachedImage{blocks: "b"})

	ResetCache()

	cachedMutex.RLock()
	length := len(cachedImages)
	cachedMutex.RUnlock()

	require.Equal(t, 0, length)
}

func TestResetIdempotent(t *testing.T) {
	t.Parallel()

	// Calling Reset on an empty cache should not panic.
	ResetCache()

	cachedMutex.RLock()
	length := len(cachedImages)
	cachedMutex.RUnlock()

	require.Equal(t, 0, length)
}

// TestTransmitBlocksPaintsOnce verifies the blocks encoding paints the image
// in the transmit command and Render returns that cached result instead of
// painting again. It is not parallel because it relies on the package-wide
// cache and paint hook.
func TestTransmitBlocksPaintsOnce(t *testing.T) {
	ResetCache()
	t.Cleanup(ResetCache)

	var paints int
	orig := paintBlocksFunc
	paintBlocksFunc = func(_ image.Image, cols, rows int) string {
		paints++
		return fmt.Sprintf("painted %dx%d", cols, rows)
	}
	t.Cleanup(func() { paintBlocksFunc = orig })

	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})

	require.False(t, HasTransmitted("img", 4, 2))
	require.Empty(t, EncodingBlocks.Render("img", 4, 2))

	cmd := EncodingBlocks.Transmit("img", img, CellSize{}, 4, 2, false)
	require.NotNil(t, cmd)
	require.Equal(t, TransmittedMsg{ID: "img-4x2"}, cmd())
	require.Equal(t, 1, paints)

	require.True(t, HasTransmitted("img", 4, 2))
	for range 3 {
		require.Equal(t, "painted 4x2", EncodingBlocks.Render("img", 4, 2))
	}
	require.Equal(t, 1, paints)

	// Already cached, so there is nothing left to transmit.
	require.Nil(t, EncodingBlocks.Transmit("img", img, CellSize{}, 4, 2, false))
}

// TestCacheBounded verifies the cache keeps only the most recent entries. It
// is not parallel because it relies on the package-wide cache.
func TestCacheBounded(t *testing.T) {
	ResetCache()
	t.Cleanup(ResetCache)

	const extra = 4
	for i := range maxCachedImages + extra {
		storeCached(imageKey{id: fmt.Sprint(i), cols: 1, rows: 1}, cachedImage{})
	}

	cachedMutex.RLock()
	length := len(cachedImages)
	cachedMutex.RUnlock()
	require.Equal(t, maxCachedImages, length)

	for i := range extra {
		require.False(t, HasTransmitted(fmt.Sprint(i), 1, 1))
	}
	for i := extra; i < maxCachedImages+extra; i++ {
		require.True(t, HasTransmitted(fmt.Sprint(i), 1, 1))
	}
}
