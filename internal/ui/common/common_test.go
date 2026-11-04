package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImagePath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"image.png",
		"image.jpg",
		"image.jpeg",
		"dir/IMAGE.PNG",
		"/abs/path/to/photo.Jpeg",
	} {
		require.True(t, IsImagePath(path), "expected %q to be an image path", path)
	}

	for _, path := range []string{
		"file.txt",
		"image.gif",
		"image.png.txt",
		"",
	} {
		require.False(t, IsImagePath(path), "expected %q to not be an image path", path)
	}
}

func TestIsAudioPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"audio.mp3",
		"audio.wav",
		"audio.M4A",
		"dir/Audio.OGG",
		"/abs/path/to/recording.FLAC",
	} {
		require.True(t, IsAudioPath(path), "expected %q to be an audio path", path)
	}

	for _, path := range []string{
		"file.txt",
		"audio.mp4",
		"audio.wav.txt",
		"",
	} {
		require.False(t, IsAudioPath(path), "expected %q to not be an audio path", path)
	}
}
